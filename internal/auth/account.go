package auth

import (
	"context"
	"database/sql"
	"errors"
	"net/http"
	"strings"
	"unicode/utf8"

	"github.com/aiulian25/keeps/internal/audit"
	"github.com/aiulian25/keeps/internal/money"
	"github.com/aiulian25/keeps/internal/store"
)

const (
	displayNameMaximumLength   = 80
	householdNameMaximumLength = 80
	usernameMaximumLength      = 60
	themeDark                  = "dark"
	themeLight                 = "light"
)

var (
	ErrInvalidDisplayName = errors.New("invalid_name")
	ErrInvalidTheme       = errors.New("invalid_theme")
	ErrInvalidUsername    = errors.New("invalid_username")
)

type Account struct {
	Username               string           `json:"username"`
	DisplayName            string           `json:"displayName"`
	Theme                  string           `json:"theme"`
	HouseholdName          string           `json:"householdName"`
	Currency               string           `json:"currency"`
	TOTPEnabled            bool             `json:"totpEnabled"`
	RecoveryCodesRemaining int              `json:"recoveryCodesRemaining"`
	PasskeysAvailable      bool             `json:"passkeysAvailable"`
	Passkeys               []PasskeySummary `json:"passkeys"`
	CSRFToken              string           `json:"csrfToken"`
	RecentlyConfirmed      bool             `json:"recentlyConfirmed"`
}

func (s *Service) LoadAccount(ctx context.Context, principal Principal) (Account, error) {
	account := Account{CSRFToken: principal.CSRFToken, PasskeysAvailable: s.PasskeysAvailable(), RecentlyConfirmed: RecentlyReauthenticated(principal)}
	err := s.store.Database.QueryRowContext(ctx, `SELECT u.username, u.display_name, u.theme, h.name, h.currency, coalesce(t.enabled, 0)
		FROM users u JOIN households h ON h.id = u.household_id LEFT JOIN totp_credentials t ON t.user_id = u.id WHERE u.id = ?`, principal.UserID).
		Scan(&account.Username, &account.DisplayName, &account.Theme, &account.HouseholdName, &account.Currency, &account.TOTPEnabled)
	if err != nil {
		return Account{}, err
	}
	account.RecoveryCodesRemaining = s.remainingRecoveryCodes(ctx, principal.UserID)
	account.Passkeys, err = s.ListPasskeys(ctx, principal.UserID)
	return account, err
}

// ThemeFor is used to render the saved theme before the first paint.
func (s *Service) ThemeFor(request *http.Request) string {
	session, err := s.sessionFromRequest(request)
	if err != nil {
		return themeDark
	}
	var theme string
	if s.store.Database.QueryRowContext(request.Context(), `SELECT theme FROM users WHERE id = ?`, session.UserID).Scan(&theme) != nil {
		return themeDark
	}
	return theme
}

type AccountChanges struct {
	DisplayName   *string `json:"displayName"`
	Theme         *string `json:"theme"`
	HouseholdName *string `json:"householdName"`
	Currency      *string `json:"currency"`
}

// UpdateAccount changes display preferences. The display name never affects
// the login username or passkeys.
func (s *Service) UpdateAccount(ctx context.Context, principal Principal, changes AccountChanges) error {
	return s.store.Transaction(ctx, func(transaction *sql.Tx) error {
		if changes.DisplayName != nil {
			name, err := cleanName(*changes.DisplayName, displayNameMaximumLength)
			if err != nil {
				return err
			}
			if _, err := transaction.ExecContext(ctx, `UPDATE users SET display_name = ?, updated_at = ? WHERE id = ?`, name, store.Now(), principal.UserID); err != nil {
				return err
			}
			audit.Record(ctx, transaction, principal.HouseholdID, principal.UserID, audit.NameChanged, "")
		}
		if changes.Theme != nil {
			validTheme := *changes.Theme == themeDark || *changes.Theme == themeLight
			if !validTheme {
				return ErrInvalidTheme
			}
			if _, err := transaction.ExecContext(ctx, `UPDATE users SET theme = ?, updated_at = ? WHERE id = ?`, *changes.Theme, store.Now(), principal.UserID); err != nil {
				return err
			}
		}
		if changes.HouseholdName != nil {
			name, err := cleanName(*changes.HouseholdName, householdNameMaximumLength)
			if err != nil {
				return err
			}
			if _, err := transaction.ExecContext(ctx, `UPDATE households SET name = ? WHERE id = ?`, name, principal.HouseholdID); err != nil {
				return err
			}
		}
		if changes.Currency != nil {
			currency, err := money.NormaliseCurrency(*changes.Currency)
			if err != nil {
				return err
			}
			if _, err := transaction.ExecContext(ctx, `UPDATE households SET currency = ? WHERE id = ?`, currency, principal.HouseholdID); err != nil {
				return err
			}
		}
		return nil
	})
}

func cleanName(value string, maximum int) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" || utf8.RuneCountInString(value) > maximum {
		return "", ErrInvalidDisplayName
	}
	return value, nil
}

// ValidateUsername accepts short names without spaces or control characters.
func ValidateUsername(username string) (string, error) {
	username = strings.TrimSpace(username)
	tooLong := utf8.RuneCountInString(username) > usernameMaximumLength
	if username == "" || tooLong || strings.ContainsAny(username, " \t\r\n") {
		return "", ErrInvalidUsername
	}
	return username, nil
}

// ChangePassword requires the current password (and second factor when
// enrolled), signs out every other session and rotates the current one.
func (s *Service) ChangePassword(ctx context.Context, writer http.ResponseWriter, principal Principal, current string, next string, code string, recoveryCode string, userAgent string) error {
	if err := ValidateNewPassword(next); err != nil {
		return err
	}
	subjects := []string{userSubject(principal.Username)}
	if err := s.checkRateLimit(ctx, subjects...); err != nil {
		return err
	}
	if err := s.confirmIdentity(ctx, principal.UserID, current, code, recoveryCode); err != nil {
		if errors.Is(err, ErrInvalidCredentials) || errors.Is(err, ErrInvalidCode) {
			s.recordFailure(ctx, subjects...)
		}
		return err
	}
	hash, err := s.HashPassword(ctx, next)
	if err != nil {
		return err
	}
	return s.store.Transaction(ctx, func(transaction *sql.Tx) error {
		if _, err := transaction.ExecContext(ctx, `UPDATE users SET password_hash = ?, updated_at = ? WHERE id = ?`, hash, store.Now(), principal.UserID); err != nil {
			return err
		}
		if err := revokeSessions(ctx, transaction, principal.UserID, 0); err != nil {
			return err
		}
		token, expires, err := s.createSession(ctx, transaction, principal.UserID, false, userAgent)
		if err != nil {
			return err
		}
		s.setSessionCookie(writer, token, expires)
		return audit.Record(ctx, transaction, principal.HouseholdID, principal.UserID, audit.PasswordChanged, "")
	})
}
