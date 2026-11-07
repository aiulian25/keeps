package auth

import (
	"context"
	"database/sql"
	"errors"
	"net/http"
	"time"

	"github.com/aiulian25/keeps/internal/audit"
	"github.com/aiulian25/keeps/internal/config"
	"github.com/aiulian25/keeps/internal/httpapi"
	"github.com/aiulian25/keeps/internal/store"
)

const (
	recoverySubjectPrefix = "recover:"
	recoveryCapabilityLen = 20
)

var ErrRecoveryInvalid = errors.New("recovery_invalid")

// IssueRecoveryCapability is run by the host owner with the server stopped.
// It signs everyone out and returns a short-lived one-use code.
func (s *Service) IssueRecoveryCapability(ctx context.Context) (string, error) {
	claimed, err := s.IsClaimed(ctx)
	if err != nil {
		return "", err
	}
	if !claimed {
		return "", errors.New("this Keeps has not been set up yet; use 'keeps bootstrap-code' instead")
	}
	code, err := generateCode(recoveryCapabilityLen)
	if err != nil {
		return "", err
	}
	err = s.store.Transaction(ctx, func(transaction *sql.Tx) error {
		if _, err := transaction.ExecContext(ctx, `DELETE FROM recovery_capabilities`); err != nil {
			return err
		}
		if _, err := transaction.ExecContext(ctx, `INSERT INTO recovery_capabilities (code_hash, expires_at) VALUES (?, ?)`,
			setupCodeVerifier(code), store.FormatTime(time.Now().Add(config.RecoveryCapabilityLifetime))); err != nil {
			return err
		}
		_, err := transaction.ExecContext(ctx, `DELETE FROM sessions`)
		return err
	})
	return code, err
}

// RecoverAdministrator redeems the capability: it sets a new password and
// removes the authenticator, recovery codes and passkeys, which the owner can
// enrol again afterwards.
func (s *Service) RecoverAdministrator(ctx context.Context, code string, newPassword string, userAgent string, clientAddress string) (SignInResult, error) {
	subject := recoverySubjectPrefix + clientAddress
	if err := s.checkRateLimit(ctx, subject); err != nil {
		return SignInResult{}, err
	}
	if err := ValidateNewPassword(newPassword); err != nil {
		return SignInResult{}, err
	}
	hash, err := s.HashPassword(ctx, newPassword)
	if err != nil {
		return SignInResult{}, err
	}
	var result SignInResult
	err = s.store.Transaction(ctx, func(transaction *sql.Tx) error {
		changed, err := transaction.ExecContext(ctx, `UPDATE recovery_capabilities SET used_at = ? WHERE code_hash = ? AND used_at IS NULL AND expires_at > ?`,
			store.Now(), setupCodeVerifier(code), store.Now())
		if err != nil {
			return err
		}
		if count, _ := changed.RowsAffected(); count != 1 {
			return ErrRecoveryInvalid
		}
		var userID, householdID int64
		if err := transaction.QueryRowContext(ctx, `SELECT id, household_id FROM users WHERE is_admin = 1 ORDER BY id LIMIT 1`).Scan(&userID, &householdID); err != nil {
			return err
		}
		statements := []string{
			`UPDATE users SET password_hash = ?, updated_at = ? WHERE id = ?`,
			`DELETE FROM totp_credentials WHERE user_id = ?`,
			`DELETE FROM recovery_codes WHERE user_id = ?`,
			`DELETE FROM passkeys WHERE user_id = ?`,
			`DELETE FROM sessions WHERE user_id = ?`,
		}
		if _, err := transaction.ExecContext(ctx, statements[0], hash, store.Now(), userID); err != nil {
			return err
		}
		for _, statement := range statements[1:] {
			if _, err := transaction.ExecContext(ctx, statement, userID); err != nil {
				return err
			}
		}
		token, expires, err := s.createSession(ctx, transaction, userID, false, userAgent)
		if err != nil {
			return err
		}
		result = SignInResult{Token: token, Expires: expires}
		return audit.Record(ctx, transaction, householdID, userID, audit.AdminRecovered, "")
	})
	if errors.Is(err, ErrRecoveryInvalid) {
		s.recordFailure(ctx, subject)
	}
	return result, err
}

func (s *Service) handleRecover(writer http.ResponseWriter, request *http.Request, clientAddress string) error {
	var body struct {
		Code        string `json:"code"`
		NewPassword string `json:"newPassword"`
	}
	if err := httpapi.DecodeJSON(writer, request, config.MaxJSONBodyBytes, &body); err != nil {
		return err
	}
	result, err := s.RecoverAdministrator(request.Context(), body.Code, body.NewPassword, request.UserAgent(), clientAddress)
	if errors.Is(err, ErrRecoveryInvalid) {
		return httpapi.FieldError("code", "recovery_invalid", "That recovery code is wrong, used or expired. Run keeps recover-admin again.")
	}
	if errors.Is(err, ErrPasswordTooShort) {
		return httpapi.FieldError("newPassword", "password_too_short", errorMessages[ErrPasswordTooShort].Message)
	}
	if err != nil {
		return err
	}
	s.setSessionCookie(writer, result.Token, result.Expires)
	httpapi.WriteJSON(writer, http.StatusOK, map[string]bool{"signedIn": true})
	return nil
}
