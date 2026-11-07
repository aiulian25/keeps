package auth

import (
	"bytes"
	"context"
	"crypto/subtle"
	"database/sql"
	"encoding/base64"
	"errors"
	"image/png"
	"strconv"
	"strings"
	"time"

	"github.com/pquerna/otp"
	"github.com/pquerna/otp/totp"

	"github.com/aiulian25/keeps/internal/audit"
	"github.com/aiulian25/keeps/internal/config"
	"github.com/aiulian25/keeps/internal/store"
)

const (
	totpIssuer        = "Keeps"
	totpDigits        = otp.DigitsSix
	totpQRCodePixels  = 200
	totpSecretPurpose = "totp-secret:"
	recoveryCodeSize  = 12
	pngDataURLPrefix  = "data:image/png;base64,"
)

var (
	ErrTOTPAlreadyEnabled = errors.New("totp_already_enabled")
	ErrTOTPNotStarted     = errors.New("totp_not_started")
	ErrTOTPNotEnabled     = errors.New("totp_not_enabled")
)

type TOTPSetup struct {
	Secret     string `json:"secret"`
	URI        string `json:"uri"`
	QRCodePNG  string `json:"qrCode"`
	AccountTag string `json:"account"`
}

func secretPurpose(userID int64) string {
	return totpSecretPurpose + strconv.FormatInt(userID, 10)
}

// BeginTOTP creates a not-yet-enabled secret. It only takes effect after
// ConfirmTOTP verifies a code from the user's authenticator.
func (s *Service) BeginTOTP(ctx context.Context, principal Principal) (TOTPSetup, error) {
	var enabled bool
	err := s.store.Database.QueryRowContext(ctx, `SELECT enabled FROM totp_credentials WHERE user_id = ?`, principal.UserID).Scan(&enabled)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return TOTPSetup{}, err
	}
	if enabled {
		return TOTPSetup{}, ErrTOTPAlreadyEnabled
	}

	key, err := totp.Generate(totp.GenerateOpts{Issuer: totpIssuer, AccountName: principal.Username, Digits: totpDigits, Period: config.TOTPPeriodSeconds})
	if err != nil {
		return TOTPSetup{}, err
	}
	sealed, err := s.keys.Seal([]byte(key.Secret()), secretPurpose(principal.UserID))
	if err != nil {
		return TOTPSetup{}, err
	}
	_, err = s.store.Database.ExecContext(ctx, `INSERT INTO totp_credentials (user_id, encrypted_secret, enabled, last_used_step, created_at)
		VALUES (?, ?, 0, 0, ?) ON CONFLICT(user_id) DO UPDATE SET encrypted_secret = excluded.encrypted_secret, last_used_step = 0, created_at = excluded.created_at`,
		principal.UserID, sealed, store.Now())
	if err != nil {
		return TOTPSetup{}, err
	}

	image, err := key.Image(totpQRCodePixels, totpQRCodePixels)
	if err != nil {
		return TOTPSetup{}, err
	}
	var encoded bytes.Buffer
	if err := png.Encode(&encoded, image); err != nil {
		return TOTPSetup{}, err
	}
	return TOTPSetup{
		Secret:     key.Secret(),
		URI:        key.URL(),
		QRCodePNG:  pngDataURLPrefix + base64.StdEncoding.EncodeToString(encoded.Bytes()),
		AccountTag: principal.Username,
	}, nil
}

// ConfirmTOTP enables the factor and returns freshly generated recovery codes,
// which are shown once and stored only as verifiers.
func (s *Service) ConfirmTOTP(ctx context.Context, principal Principal, code string) ([]string, error) {
	var codes []string
	err := s.store.Transaction(ctx, func(transaction *sql.Tx) error {
		var enabled bool
		err := transaction.QueryRowContext(ctx, `SELECT enabled FROM totp_credentials WHERE user_id = ?`, principal.UserID).Scan(&enabled)
		if errors.Is(err, sql.ErrNoRows) {
			return ErrTOTPNotStarted
		}
		if err != nil {
			return err
		}
		if enabled {
			return ErrTOTPAlreadyEnabled
		}
		accepted, err := s.verifyTOTP(ctx, transaction, principal.UserID, code, false)
		if err != nil {
			return err
		}
		if !accepted {
			return ErrInvalidCode
		}
		if _, err := transaction.ExecContext(ctx, `UPDATE totp_credentials SET enabled = 1 WHERE user_id = ?`, principal.UserID); err != nil {
			return err
		}
		codes, err = replaceRecoveryCodes(ctx, transaction, principal.UserID)
		if err != nil {
			return err
		}
		if err := revokeSessions(ctx, transaction, principal.UserID, principal.SessionID); err != nil {
			return err
		}
		return audit.Record(ctx, transaction, principal.HouseholdID, principal.UserID, audit.TOTPEnabled, "")
	})
	return codes, err
}

func (s *Service) DisableTOTP(ctx context.Context, principal Principal) error {
	return s.store.Transaction(ctx, func(transaction *sql.Tx) error {
		if _, err := transaction.ExecContext(ctx, `DELETE FROM totp_credentials WHERE user_id = ?`, principal.UserID); err != nil {
			return err
		}
		if _, err := transaction.ExecContext(ctx, `DELETE FROM recovery_codes WHERE user_id = ?`, principal.UserID); err != nil {
			return err
		}
		if err := revokeSessions(ctx, transaction, principal.UserID, principal.SessionID); err != nil {
			return err
		}
		return audit.Record(ctx, transaction, principal.HouseholdID, principal.UserID, audit.TOTPDisabled, "")
	})
}

func (s *Service) RegenerateRecoveryCodes(ctx context.Context, principal Principal) ([]string, error) {
	var codes []string
	err := s.store.Transaction(ctx, func(transaction *sql.Tx) error {
		var enabled bool
		err := transaction.QueryRowContext(ctx, `SELECT enabled FROM totp_credentials WHERE user_id = ?`, principal.UserID).Scan(&enabled)
		if errors.Is(err, sql.ErrNoRows) || !enabled {
			return ErrTOTPNotEnabled
		}
		if err != nil {
			return err
		}
		codes, err = replaceRecoveryCodes(ctx, transaction, principal.UserID)
		if err != nil {
			return err
		}
		return audit.Record(ctx, transaction, principal.HouseholdID, principal.UserID, audit.RecoveryCodesReset, "")
	})
	return codes, err
}

// verifyTOTP accepts a code within the allowed clock skew and rejects any
// time step at or before the last accepted one, so a code cannot be replayed.
func (s *Service) verifyTOTP(ctx context.Context, transaction *sql.Tx, userID int64, code string, requireEnabled bool) (bool, error) {
	var sealed []byte
	var enabled bool
	var lastUsedStep int64
	err := transaction.QueryRowContext(ctx, `SELECT encrypted_secret, enabled, last_used_step FROM totp_credentials WHERE user_id = ?`, userID).
		Scan(&sealed, &enabled, &lastUsedStep)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	if requireEnabled && !enabled {
		return false, nil
	}
	secret, err := s.keys.Open(sealed, secretPurpose(userID))
	if err != nil {
		return false, err
	}

	code = strings.ReplaceAll(strings.TrimSpace(code), " ", "")
	period := int64(config.TOTPPeriodSeconds)
	currentStep := time.Now().Unix() / period
	for offset := -int64(config.TOTPAllowedSkewSteps); offset <= int64(config.TOTPAllowedSkewSteps); offset++ {
		step := currentStep + offset
		if step <= lastUsedStep {
			continue
		}
		expected, err := totp.GenerateCodeCustom(string(secret), time.Unix(step*period, 0), totp.ValidateOpts{
			Period: uint(period), Digits: totpDigits, Algorithm: otp.AlgorithmSHA1,
		})
		if err != nil {
			return false, err
		}
		if subtle.ConstantTimeCompare([]byte(expected), []byte(code)) != 1 {
			continue
		}
		_, err = transaction.ExecContext(ctx, `UPDATE totp_credentials SET last_used_step = ? WHERE user_id = ?`, step, userID)
		return err == nil, err
	}
	return false, nil
}

func replaceRecoveryCodes(ctx context.Context, transaction *sql.Tx, userID int64) ([]string, error) {
	if _, err := transaction.ExecContext(ctx, `DELETE FROM recovery_codes WHERE user_id = ?`, userID); err != nil {
		return nil, err
	}
	codes := make([]string, 0, config.RecoveryCodeCount)
	for range config.RecoveryCodeCount {
		code, err := generateCode(recoveryCodeSize)
		if err != nil {
			return nil, err
		}
		if _, err := transaction.ExecContext(ctx, `INSERT INTO recovery_codes (user_id, code_hash) VALUES (?, ?)`, userID, setupCodeVerifier(code)); err != nil {
			return nil, err
		}
		codes = append(codes, code)
	}
	return codes, nil
}

func consumeRecoveryCode(ctx context.Context, transaction *sql.Tx, userID int64, code string) (bool, error) {
	result, err := transaction.ExecContext(ctx, `UPDATE recovery_codes SET used_at = ? WHERE user_id = ? AND code_hash = ? AND used_at IS NULL`,
		store.Now(), userID, setupCodeVerifier(code))
	if err != nil {
		return false, err
	}
	changed, _ := result.RowsAffected()
	if changed != 1 {
		return false, nil
	}
	var householdID int64
	transaction.QueryRowContext(ctx, `SELECT household_id FROM users WHERE id = ?`, userID).Scan(&householdID)
	return true, audit.Record(ctx, transaction, householdID, userID, audit.RecoveryCodeUsed, "")
}

func (s *Service) remainingRecoveryCodes(ctx context.Context, userID int64) int {
	var count int
	s.store.Database.QueryRowContext(ctx, `SELECT count(*) FROM recovery_codes WHERE user_id = ? AND used_at IS NULL`, userID).Scan(&count)
	return count
}
