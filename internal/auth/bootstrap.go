package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"database/sql"
	"errors"
	"strings"
	"time"

	"github.com/aiulian25/keeps/internal/audit"
	"github.com/aiulian25/keeps/internal/config"
	"github.com/aiulian25/keeps/internal/store"
)

const (
	// 16 symbols from a 32-symbol alphabet = 80 bits of entropy.
	setupCodeAlphabet   = "ABCDEFGHJKLMNPQRSTUVWXYZ23456789"
	setupCodeLength     = 16
	setupCodeGroupSize  = 4
	setupCodeSeparator  = "-"
	defaultCurrencyCode = "GBP"
)

var (
	ErrAlreadyClaimed   = errors.New("setup_code_used")
	ErrSetupCodeInvalid = errors.New("setup_code_invalid")
	ErrSetupCodeExpired = errors.New("setup_code_expired")
)

// generateCode returns length symbols grouped in fours, e.g. ABCD-EFGH-JKLM.
func generateCode(length int) (string, error) {
	random := make([]byte, length)
	if _, err := rand.Read(random); err != nil {
		return "", err
	}
	var builder strings.Builder
	for index, value := range random {
		if index > 0 && index%setupCodeGroupSize == 0 {
			builder.WriteString(setupCodeSeparator)
		}
		builder.WriteByte(setupCodeAlphabet[int(value)%len(setupCodeAlphabet)])
	}
	return builder.String(), nil
}

func normaliseSetupCode(code string) string {
	code = strings.ToUpper(strings.TrimSpace(code))
	return strings.ReplaceAll(strings.ReplaceAll(code, setupCodeSeparator, ""), " ", "")
}

func setupCodeVerifier(code string) []byte {
	sum := sha256.Sum256([]byte(normaliseSetupCode(code)))
	return sum[:]
}

func (s *Service) IsClaimed(ctx context.Context) (bool, error) {
	var claimed bool
	err := s.store.Database.QueryRowContext(ctx, `SELECT claimed FROM bootstrap_state WHERE id = 1`).Scan(&claimed)
	return claimed, err
}

// EnsureSetupCode issues a new setup code when the instance is unclaimed and
// no unexpired code exists. It returns "" when nothing was issued.
func (s *Service) EnsureSetupCode(ctx context.Context, forceRotate bool) (string, error) {
	var claimed bool
	var expires sql.NullString
	err := s.store.Database.QueryRowContext(ctx, `SELECT claimed, code_expires_at FROM bootstrap_state WHERE id = 1`).Scan(&claimed, &expires)
	if err != nil {
		return "", err
	}
	if claimed {
		return "", ErrAlreadyClaimed
	}
	expiry, _ := store.ParseTime(expires.String)
	stillValid := expires.Valid && time.Now().Before(expiry)
	if stillValid && !forceRotate {
		return "", nil
	}
	code, err := generateCode(setupCodeLength)
	if err != nil {
		return "", err
	}
	_, err = s.store.Database.ExecContext(ctx, `UPDATE bootstrap_state SET code_verifier = ?, code_expires_at = ? WHERE id = 1 AND claimed = 0`,
		setupCodeVerifier(code), store.FormatTime(time.Now().Add(config.SetupCodeLifetime)))
	return code, err
}

type ClaimRequest struct {
	SetupCode     string
	HouseholdName string
	Username      string
	Password      string
	UserAgent     string
	ClientAddress string
}

// Claim atomically creates the household and its single administrator. The
// conditional UPDATE guarantees exactly one winner under concurrent claims.
func (s *Service) Claim(ctx context.Context, request ClaimRequest) (token string, expires time.Time, err error) {
	if err := s.checkRateLimit(ctx, setupSubjectPrefix+request.ClientAddress); err != nil {
		return "", time.Time{}, err
	}
	// Refused before the expensive hash; the transaction below still makes
	// the final, atomic decision.
	var claimed bool
	if err := s.store.Database.QueryRowContext(ctx, `SELECT claimed FROM bootstrap_state WHERE id = 1`).Scan(&claimed); err != nil {
		return "", time.Time{}, err
	}
	if claimed {
		return "", time.Time{}, ErrAlreadyClaimed
	}
	passwordHash, err := s.HashPassword(ctx, request.Password)
	if err != nil {
		return "", time.Time{}, err
	}

	err = s.store.Transaction(ctx, func(transaction *sql.Tx) error {
		var claimed bool
		var verifier []byte
		var expiresAt sql.NullString
		err := transaction.QueryRowContext(ctx, `SELECT claimed, code_verifier, code_expires_at FROM bootstrap_state WHERE id = 1`).
			Scan(&claimed, &verifier, &expiresAt)
		if err != nil {
			return err
		}
		if claimed {
			return ErrAlreadyClaimed
		}
		if subtle.ConstantTimeCompare(verifier, setupCodeVerifier(request.SetupCode)) != 1 {
			return ErrSetupCodeInvalid
		}
		expiry, _ := store.ParseTime(expiresAt.String)
		if time.Now().After(expiry) {
			return ErrSetupCodeExpired
		}

		result, err := transaction.ExecContext(ctx, `UPDATE bootstrap_state SET claimed = 1, code_verifier = NULL, code_expires_at = NULL WHERE id = 1 AND claimed = 0`)
		if err != nil {
			return err
		}
		if changed, _ := result.RowsAffected(); changed != 1 {
			return ErrAlreadyClaimed
		}

		now := store.Now()
		householdResult, err := transaction.ExecContext(ctx, `INSERT INTO households (name, currency, created_at) VALUES (?, ?, ?)`,
			request.HouseholdName, defaultCurrencyCode, now)
		if err != nil {
			return err
		}
		householdID, _ := householdResult.LastInsertId()
		handle, err := randomBytes(webauthnHandleBytes)
		if err != nil {
			return err
		}
		userResult, err := transaction.ExecContext(ctx, `INSERT INTO users (household_id, username, display_name, password_hash, webauthn_handle, is_admin, created_at, updated_at)
			VALUES (?, ?, ?, ?, ?, 1, ?, ?)`, householdID, request.Username, request.Username, passwordHash, handle, now, now)
		if err != nil {
			return err
		}
		userID, _ := userResult.LastInsertId()
		if err := audit.Record(ctx, transaction, householdID, userID, audit.AdministratorClaim, ""); err != nil {
			return err
		}
		token, expires, err = s.createSession(ctx, transaction, userID, false, request.UserAgent)
		return err
	})
	if errors.Is(err, ErrSetupCodeInvalid) {
		s.recordFailure(ctx, setupSubjectPrefix+request.ClientAddress)
	}
	return token, expires, err
}
