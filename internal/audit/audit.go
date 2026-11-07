// Package audit records security and data events without secrets or content.
package audit

import (
	"context"
	"database/sql"

	"github.com/aiulian25/keeps/internal/store"
)

// Executor is satisfied by both *sql.DB and *sql.Tx.
type Executor interface {
	ExecContext(ctx context.Context, query string, arguments ...any) (sql.Result, error)
}

const (
	SignedIn           = "signed_in"
	SignInFailed       = "sign_in_failed"
	SignedOut          = "signed_out"
	AdministratorClaim = "administrator_claimed"
	PasswordChanged    = "password_changed"
	NameChanged        = "name_changed"
	SessionsRevoked    = "sessions_revoked"
	TOTPEnabled        = "totp_enabled"
	TOTPDisabled       = "totp_disabled"
	RecoveryCodeUsed   = "recovery_code_used"
	RecoveryCodesReset = "recovery_codes_regenerated"
	PasskeyAdded       = "passkey_added"
	PasskeyRemoved     = "passkey_removed"
	ExportCreated      = "export_created"
	BackupCreated      = "backup_created"
	RestoreCompleted   = "restore_completed"
	AdminRecovered     = "administrator_recovered"
	ItemPurged         = "item_permanently_deleted"
)

// Record writes one event. householdID and userID may be zero when unknown.
func Record(ctx context.Context, executor Executor, householdID int64, userID int64, kind string, detail string) error {
	_, err := executor.ExecContext(ctx,
		`INSERT INTO audit_events (household_id, user_id, kind, detail, created_at) VALUES (nullif(?, 0), nullif(?, 0), ?, ?, ?)`,
		householdID, userID, kind, detail, store.Now())
	return err
}
