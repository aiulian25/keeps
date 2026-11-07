package auth

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"time"

	"github.com/aiulian25/keeps/internal/audit"
)

var (
	ErrInvalidCredentials = errors.New("invalid_credentials")
	ErrInvalidCode        = errors.New("invalid_code")
)

type SignInResult struct {
	Token                string
	Expires              time.Time
	AwaitingSecondFactor bool
}

type userCredentials struct {
	ID           int64
	HouseholdID  int64
	PasswordHash string
	TOTPEnabled  bool
}

func (s *Service) findUserByUsername(ctx context.Context, username string) (userCredentials, error) {
	var user userCredentials
	err := s.store.Database.QueryRowContext(ctx, `SELECT u.id, u.household_id, u.password_hash, coalesce(t.enabled, 0)
		FROM users u LEFT JOIN totp_credentials t ON t.user_id = u.id WHERE u.username = ?`, strings.TrimSpace(username)).
		Scan(&user.ID, &user.HouseholdID, &user.PasswordHash, &user.TOTPEnabled)
	return user, err
}

func (s *Service) findUserByID(ctx context.Context, userID int64) (userCredentials, error) {
	var user userCredentials
	err := s.store.Database.QueryRowContext(ctx, `SELECT u.id, u.household_id, u.password_hash, coalesce(t.enabled, 0)
		FROM users u LEFT JOIN totp_credentials t ON t.user_id = u.id WHERE u.id = ?`, userID).
		Scan(&user.ID, &user.HouseholdID, &user.PasswordHash, &user.TOTPEnabled)
	return user, err
}

// SignInWithPassword implements the password row of the login policy: when
// TOTP is enabled the session is created in an "awaiting second factor" state
// that grants no access until the code is verified.
func (s *Service) SignInWithPassword(ctx context.Context, username string, password string, userAgent string, clientAddress string) (SignInResult, error) {
	subjects := []string{userSubject(username), clientSubject(clientAddress)}
	if err := s.checkRateLimit(ctx, subjects...); err != nil {
		return SignInResult{}, err
	}

	user, err := s.findUserByUsername(ctx, username)
	if errors.Is(err, sql.ErrNoRows) {
		s.verifyAgainstDummy(ctx, password)
		s.recordFailure(ctx, subjects...)
		return SignInResult{}, ErrInvalidCredentials
	}
	if err != nil {
		return SignInResult{}, err
	}
	valid, err := s.VerifyPassword(ctx, password, user.PasswordHash)
	if err != nil {
		return SignInResult{}, err
	}
	if !valid {
		s.recordFailure(ctx, subjects...)
		audit.Record(ctx, s.store.Database, user.HouseholdID, user.ID, audit.SignInFailed, "")
		return SignInResult{}, ErrInvalidCredentials
	}
	s.clearFailures(ctx, userSubject(username))

	token, expires, err := s.createSession(ctx, s.store.Database, user.ID, user.TOTPEnabled, userAgent)
	if err != nil {
		return SignInResult{}, err
	}
	if !user.TOTPEnabled {
		audit.Record(ctx, s.store.Database, user.HouseholdID, user.ID, audit.SignedIn, "password")
	}
	return SignInResult{Token: token, Expires: expires, AwaitingSecondFactor: user.TOTPEnabled}, nil
}

// CompleteSecondFactor exchanges a pending session plus a valid TOTP or
// recovery code for a full session.
func (s *Service) CompleteSecondFactor(ctx context.Context, pending sessionRecord, code string, recoveryCode string, userAgent string) (SignInResult, error) {
	if !pending.AwaitingSecondFactor {
		return SignInResult{}, ErrInvalidCode
	}
	subject := codeSubjectPrefix + userSubject(pending.Username)
	if err := s.checkRateLimit(ctx, subject); err != nil {
		return SignInResult{}, err
	}

	var result SignInResult
	err := s.store.Transaction(ctx, func(transaction *sql.Tx) error {
		accepted, method, err := s.verifySecondFactor(ctx, transaction, pending.UserID, code, recoveryCode)
		if err != nil {
			return err
		}
		if !accepted {
			return ErrInvalidCode
		}
		if _, err := transaction.ExecContext(ctx, `DELETE FROM sessions WHERE id = ?`, pending.ID); err != nil {
			return err
		}
		token, expires, err := s.createSession(ctx, transaction, pending.UserID, false, userAgent)
		if err != nil {
			return err
		}
		result = SignInResult{Token: token, Expires: expires}
		return audit.Record(ctx, transaction, pending.HouseholdID, pending.UserID, audit.SignedIn, "password+"+method)
	})
	if errors.Is(err, ErrInvalidCode) {
		s.recordFailure(ctx, subject)
		return SignInResult{}, err
	}
	if err == nil {
		s.clearFailures(ctx, subject)
	}
	return result, err
}

// verifySecondFactor checks a TOTP code, or else a one-use recovery code.
func (s *Service) verifySecondFactor(ctx context.Context, transaction *sql.Tx, userID int64, code string, recoveryCode string) (bool, string, error) {
	if strings.TrimSpace(recoveryCode) != "" {
		used, err := consumeRecoveryCode(ctx, transaction, userID, recoveryCode)
		return used, "recovery_code", err
	}
	accepted, err := s.verifyTOTP(ctx, transaction, userID, code, true)
	return accepted, "totp", err
}

// Reauthenticate confirms the signed-in user's identity before sensitive
// actions: current password, plus TOTP (or a recovery code) when enrolled.
func (s *Service) Reauthenticate(ctx context.Context, principal Principal, password string, code string, recoveryCode string) error {
	subjects := []string{userSubject(principal.Username)}
	if err := s.checkRateLimit(ctx, subjects...); err != nil {
		return err
	}
	if err := s.confirmIdentity(ctx, principal.UserID, password, code, recoveryCode); err != nil {
		if errors.Is(err, ErrInvalidCredentials) || errors.Is(err, ErrInvalidCode) {
			s.recordFailure(ctx, subjects...)
		}
		return err
	}
	return s.markReauthenticated(ctx, principal.SessionID)
}

func (s *Service) confirmIdentity(ctx context.Context, userID int64, password string, code string, recoveryCode string) error {
	user, err := s.findUserByID(ctx, userID)
	if err != nil {
		return err
	}
	valid, err := s.VerifyPassword(ctx, password, user.PasswordHash)
	if err != nil {
		return err
	}
	if !valid {
		return ErrInvalidCredentials
	}
	if !user.TOTPEnabled {
		return nil
	}
	return s.store.Transaction(ctx, func(transaction *sql.Tx) error {
		accepted, _, err := s.verifySecondFactor(ctx, transaction, userID, code, recoveryCode)
		if err != nil {
			return err
		}
		if !accepted {
			return ErrInvalidCode
		}
		return nil
	})
}

func (s *Service) SignOut(ctx context.Context, principal Principal) error {
	_, err := s.store.Database.ExecContext(ctx, `DELETE FROM sessions WHERE id = ?`, principal.SessionID)
	audit.Record(ctx, s.store.Database, principal.HouseholdID, principal.UserID, audit.SignedOut, "")
	return err
}

type SessionSummary struct {
	ID         int64  `json:"id"`
	UserAgent  string `json:"userAgent"`
	CreatedAt  string `json:"createdAt"`
	LastSeenAt string `json:"lastSeenAt"`
	Current    bool   `json:"current"`
}

func (s *Service) ListSessions(ctx context.Context, principal Principal) ([]SessionSummary, error) {
	rows, err := s.store.Database.QueryContext(ctx, `SELECT id, user_agent, created_at, last_seen_at FROM sessions
		WHERE user_id = ? AND awaiting_second_factor = 0 ORDER BY last_seen_at DESC`, principal.UserID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	sessions := []SessionSummary{}
	for rows.Next() {
		var summary SessionSummary
		if err := rows.Scan(&summary.ID, &summary.UserAgent, &summary.CreatedAt, &summary.LastSeenAt); err != nil {
			return nil, err
		}
		summary.Current = summary.ID == principal.SessionID
		sessions = append(sessions, summary)
	}
	return sessions, rows.Err()
}

func (s *Service) RevokeSession(ctx context.Context, principal Principal, sessionID int64) error {
	_, err := s.store.Database.ExecContext(ctx, `DELETE FROM sessions WHERE id = ? AND user_id = ?`, sessionID, principal.UserID)
	return err
}

func (s *Service) RevokeOtherSessions(ctx context.Context, principal Principal) error {
	if err := revokeSessions(ctx, s.store.Database, principal.UserID, principal.SessionID); err != nil {
		return err
	}
	return audit.Record(ctx, s.store.Database, principal.HouseholdID, principal.UserID, audit.SessionsRevoked, "")
}
