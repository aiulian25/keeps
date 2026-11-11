package server

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"syscall"
	"time"

	"filippo.io/age"

	"github.com/aiulian25/keeps/internal/audit"
	"github.com/aiulian25/keeps/internal/auth"
	"github.com/aiulian25/keeps/internal/backup"
	"github.com/aiulian25/keeps/internal/config"
	"github.com/aiulian25/keeps/internal/exporter"
	"github.com/aiulian25/keeps/internal/httpapi"
)

const (
	backupFilePrefix       = "keeps-backup-"
	backupFileExtension    = ".keeps.age"
	exportFilePrefix       = "keeps-export-"
	fileDateFormat         = "2006-01-02"
	drainPollInterval      = 50 * time.Millisecond
	drainTimeout           = 30 * time.Second
	passphraseMinimumRunes = 8
	passphraseField        = "passphrase"
	fileField              = "file"
	acceptIncompleteField  = "acceptIncomplete"
	acceptedValue          = "1"
	uploadFileName         = "upload.age"
)

var (
	errRecoveryNeeded    = httpapi.NewError(http.StatusServiceUnavailable, "recovery_needed", "A restore failed and Keeps couldn't confirm that your previous data is back, so it has stopped to keep both copies safe. Nothing will be changed. Follow \"A restore went wrong\" in docs/recovery.md.")
	errRestoreRolledBack = httpapi.NewError(http.StatusInternalServerError, "restore_rolled_back", "Keeps couldn't finish restoring this backup, so your previous data was put back and checked. Nothing was changed. Try again; if it keeps failing, check the server logs.")
	errMaintenance       = httpapi.NewError(http.StatusServiceUnavailable, "maintenance", "Keeps is restoring a backup. Try again in a minute.")
	errWeakPassphrase    = httpapi.FieldError("passphrase", "passphrase_too_short", "Use a passphrase of at least 8 characters. Keep it somewhere safe: the backup can't be opened without it.")
	errRestoreSecret     = httpapi.FieldError("passphrase", "backup_wrong_secret", "That passphrase doesn't open this backup.")
	errRestoreDamaged    = httpapi.NewError(http.StatusBadRequest, "backup_damaged", "This backup file is damaged or incomplete. Nothing was changed.")
	errRestoreIncompat   = httpapi.NewError(http.StatusBadRequest, "backup_incompatible", "This backup was made by a newer version of Keeps. Upgrade Keeps first. Nothing was changed.")
	errRestoreUnsafe     = httpapi.NewError(http.StatusBadRequest, "backup_unsafe", "This file isn't a Keeps backup. Nothing was changed.")
	errRestoreConcurrent = httpapi.NewError(http.StatusConflict, "restore_busy", "Another restore is in progress.")
	errRestoreNoSpace    = httpapi.NewError(http.StatusInsufficientStorage, "not_enough_space", "There isn't enough free space on the server to unpack this backup. Free some space and try again. Nothing was changed.")
	errInvalidFormat     = httpapi.FieldError("format", "invalid_format", "Choose CSV or full archive.")
)

// restoreIdleTimeout ends a restore upload that has stopped sending; a
// variable so tests can shorten it.
var restoreIdleTimeout = config.RestoreUploadIdleTimeout

// deadlineReader pushes the read deadline forward by restoreIdleTimeout before
// every read, never beyond stopAt.
type deadlineReader struct {
	reader     io.ReadCloser
	controller *http.ResponseController
	stopAt     time.Time
}

func (r *deadlineReader) Read(buffer []byte) (int, error) {
	next := time.Now().Add(restoreIdleTimeout)
	if next.After(r.stopAt) {
		next = r.stopAt
	}
	if err := r.controller.SetReadDeadline(next); err != nil && !errors.Is(err, http.ErrNotSupported) {
		return 0, err
	}
	return r.reader.Read(buffer)
}

func (r *deadlineReader) Close() error { return r.reader.Close() }

// preparedFile tells the browser where to fetch a finished backup or export,
// and whether every referenced photo and document made it in.
type preparedFile struct {
	DownloadURL  string               `json:"downloadUrl"`
	Filename     string               `json:"filename"`
	Complete     bool                 `json:"complete"`
	MissingFiles []backup.MissingFile `json:"missingFiles"`
}

func incompleteRestoreError(missing []backup.MissingFile) error {
	return &httpapi.Error{Status: http.StatusConflict, Code: "backup_incomplete",
		Message: "Some photos or documents in this backup are missing or damaged. Restoring it brings back everything else, but those files will be gone or wrong. Nothing was changed.",
		Details: map[string]any{"missingFiles": missing}}
}

// gate rejects requests during a restore and counts requests in flight so
// the restore can wait for them to finish before swapping the database.
type gate struct {
	maintenance    atomic.Bool
	recoveryNeeded atomic.Bool
	inFlight       atomic.Int64
}

func (g *gate) wrap(next http.Handler) http.Handler {
	return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		// Counted before the maintenance check: once drain sees only itself,
		// every other request has either finished or seen maintenance.
		g.inFlight.Add(1)
		defer g.inFlight.Add(-1)
		if g.recoveryNeeded.Load() {
			httpapi.WriteError(writer, request, errRecoveryNeeded)
			return
		}
		if g.maintenance.Load() {
			httpapi.WriteError(writer, request, errMaintenance)
			return
		}
		next.ServeHTTP(writer, request)
	})
}

// drain waits until only the caller's own request remains.
func (g *gate) drain() error {
	deadline := time.Now().Add(drainTimeout)
	for g.inFlight.Load() > 1 {
		if time.Now().After(deadline) {
			return errors.New("requests did not finish in time")
		}
		time.Sleep(drainPollInterval)
	}
	return nil
}

func (a *Application) dataRoutes(mux *http.ServeMux) {
	handle := func(pattern string, handler httpapi.Handler) {
		mux.Handle(pattern, a.Auth.Protect(auth.RequireRecentReauthentication(handler)))
	}
	handle("POST /api/v1/backups", a.handleBackup)
	handle("POST /api/v1/exports", a.handleExport)
	handle("POST /api/v1/restore", a.handleRestore)
	handle("GET /api/v1/downloads/{id}", a.handleDownload)
}

func (a *Application) backupSources() backup.Sources {
	return backup.Sources{
		Store:        a.Store,
		Blobs:        a.Blobs,
		KeyFile:      filepath.Join(a.Configuration.Directory(config.KeyDirectoryName), config.InstanceKeyFileName),
		TemporaryDir: a.Configuration.Directory(config.TempDirectoryName),
		AppVersion:   a.Version,
	}
}

func downloadName(prefix string, extension string) string {
	return prefix + time.Now().Format(fileDateFormat) + extension
}

func attachmentHeaders(writer http.ResponseWriter, contentType string, filename string) {
	writer.Header().Set("Content-Type", contentType)
	writer.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, filename))
	writer.Header().Set("Cache-Control", "no-store")
}

// handleBackup writes the whole encrypted backup to private temporary
// storage, then reports where to download it and whether it is complete.
func (a *Application) handleBackup(writer http.ResponseWriter, request *http.Request) error {
	var input struct {
		Passphrase string `json:"passphrase"`
	}
	if err := httpapi.DecodeJSON(writer, request, config.MaxJSONBodyBytes, &input); err != nil {
		return err
	}
	if len([]rune(input.Passphrase)) < passphraseMinimumRunes {
		return errWeakPassphrase
	}
	recipient, err := backup.PassphraseRecipient(input.Passphrase)
	if err != nil {
		return err
	}
	principal := auth.PrincipalFromRequest(request)
	filename := downloadName(backupFilePrefix, backupFileExtension)
	var manifest backup.Manifest
	downloadURL, err := a.downloads.prepare(a.temporaryDirectory(), principal.UserID, filename, "application/octet-stream", func(destination io.Writer) error {
		a.blobLock.RLock()
		defer a.blobLock.RUnlock()
		written, writeErr := backup.Write(request.Context(), a.backupSources(), destination, recipient)
		manifest = written
		return writeErr
	})
	if err != nil {
		return err
	}
	if !manifest.Complete() {
		slog.Warn("backup is incomplete", "missing", len(manifest.Missing))
	}
	audit.Record(request.Context(), a.Store.Database, principal.HouseholdID, principal.UserID, audit.BackupCreated, "browser")
	httpapi.WriteJSON(writer, http.StatusOK, preparedFile{DownloadURL: downloadURL, Filename: filename, Complete: manifest.Complete(), MissingFiles: missingOrEmpty(manifest.MissingFiles)})
	return nil
}

func missingOrEmpty(missing []backup.MissingFile) []backup.MissingFile {
	if missing == nil {
		return []backup.MissingFile{}
	}
	return missing
}

// handleExport prepares the whole export before offering it, so a missing
// file or a full disk is reported instead of producing a partial download.
func (a *Application) handleExport(writer http.ResponseWriter, request *http.Request) error {
	var input struct {
		Format string `json:"format"`
	}
	if err := httpapi.DecodeJSON(writer, request, config.MaxJSONBodyBytes, &input); err != nil {
		return err
	}
	principal := auth.PrincipalFromRequest(request)
	var filename, contentType string
	var write func(io.Writer) error
	switch input.Format {
	case "csv":
		filename, contentType = downloadName(exportFilePrefix, ".csv"), "text/csv; charset=utf-8"
		write = func(destination io.Writer) error {
			return exporter.WriteCSV(request.Context(), a.Items, principal.HouseholdID, destination)
		}
	case "archive":
		filename, contentType = downloadName(exportFilePrefix, ".zip"), "application/zip"
		write = func(destination io.Writer) error {
			a.blobLock.RLock()
			defer a.blobLock.RUnlock()
			return exporter.WriteArchive(request.Context(), a.Items, a.Store.Database, a.Blobs, principal.HouseholdID, destination)
		}
	default:
		return errInvalidFormat
	}
	downloadURL, err := a.downloads.prepare(a.temporaryDirectory(), principal.UserID, filename, contentType, func(destination io.Writer) error {
		err := write(destination)
		if errors.Is(err, exporter.ErrFileDamaged) {
			return httpapi.NewError(http.StatusConflict, "export_file_damaged", fmt.Sprintf("A file in storage no longer matches what was uploaded, so the archive can't be trusted: %s. Nothing was downloaded. Restore that file from an older backup, or upload it again.", strings.TrimPrefix(err.Error(), exporter.ErrFileDamaged.Error()+": ")))
		}
		if errors.Is(err, exporter.ErrFileMissing) {
			return httpapi.NewError(http.StatusConflict, "export_file_missing", fmt.Sprintf("A file is missing from storage, so the archive can't be complete: %s. Nothing was downloaded. A backup lists every missing file; the spreadsheet export still works.", strings.TrimPrefix(err.Error(), exporter.ErrFileMissing.Error()+": ")))
		}
		return err
	})
	if err != nil {
		return err
	}
	audit.Record(request.Context(), a.Store.Database, principal.HouseholdID, principal.UserID, audit.ExportCreated, input.Format)
	httpapi.WriteJSON(writer, http.StatusOK, preparedFile{DownloadURL: downloadURL, Filename: filename, Complete: true, MissingFiles: []backup.MissingFile{}})
	return nil
}

// handleRestore receives the passphrase then the archive, validates it into
// staging, and only then swaps it in under maintenance mode.
func (a *Application) handleRestore(writer http.ResponseWriter, request *http.Request) error {
	if !a.restoring.CompareAndSwap(false, true) {
		return errRestoreConcurrent
	}
	defer a.restoring.Store(false)

	// A backup can take far longer to upload than ordinary requests may take.
	// The deadline moves forward while data keeps arriving, so only a stalled
	// or very long upload is cut off; other routes keep their own limits.
	controller := http.NewResponseController(writer)
	stopAt := time.Now().Add(config.RestoreUploadMaximumDuration)
	controller.SetWriteDeadline(stopAt.Add(config.WriteTimeout))
	request.Body = http.MaxBytesReader(writer, &deadlineReader{reader: request.Body, controller: controller, stopAt: stopAt}, config.RestoreUploadMaximum)
	reader, err := request.MultipartReader()
	if err != nil {
		return httpapi.NewError(http.StatusBadRequest, "no_file", "Choose a backup file.")
	}
	fields, uploadPath, err := a.receiveRestoreUpload(reader)
	if uploadPath != "" {
		defer os.Remove(uploadPath)
	}
	if err != nil {
		return err
	}
	identity, err := age.NewScryptIdentity(fields[passphraseField])
	if err != nil {
		return errRestoreSecret
	}
	upload, err := os.Open(uploadPath)
	if err != nil {
		return err
	}
	defer upload.Close()
	staging, manifest, err := backup.Stage(request.Context(), a.Configuration.DataDirectory, upload, identity)
	if err != nil {
		return translateRestoreError(err)
	}
	if !manifest.Complete() && fields[acceptIncompleteField] != acceptedValue {
		os.RemoveAll(staging)
		return incompleteRestoreError(manifest.MissingFiles)
	}

	if err := a.swapIn(request.Context(), staging); err != nil {
		os.RemoveAll(staging)
		if errors.Is(err, errRecoveryNeeded) {
			return err
		}
		slog.Error("restore was rolled back", "error", err)
		return errRestoreRolledBack
	}
	a.Auth.ClearAuthCookie(writer)
	httpapi.WriteJSON(writer, http.StatusOK, map[string]any{"restored": true, "createdAt": manifest.CreatedAt, "files": len(manifest.Files), "missingFiles": missingOrEmpty(manifest.MissingFiles)})
	return nil
}

// receiveRestoreUpload reads the small fields that come before the file
// (passphrase, acceptIncomplete), then saves the file to private staging.
func (a *Application) receiveRestoreUpload(reader *multipart.Reader) (map[string]string, string, error) {
	fields := map[string]string{}
	for {
		part, err := reader.NextPart()
		if errors.Is(err, io.EOF) {
			return nil, "", httpapi.NewError(http.StatusBadRequest, "no_file", "Choose a backup file.")
		}
		if err != nil {
			return nil, "", httpapi.NewError(http.StatusBadRequest, "upload_interrupted", "The upload was interrupted. Nothing was changed. Try again.")
		}
		if part.FormName() == passphraseField || part.FormName() == acceptIncompleteField {
			value, _ := io.ReadAll(io.LimitReader(part, config.MaxJSONBodyBytes))
			fields[part.FormName()] = string(value)
			continue
		}
		if part.FormName() != fileField {
			continue
		}
		stagingRoot := a.Configuration.Directory(config.StagingDirectoryName)
		if err := os.MkdirAll(stagingRoot, config.DirectoryMode); err != nil {
			return nil, "", err
		}
		uploadPath := filepath.Join(stagingRoot, fmt.Sprintf("%d-%s", time.Now().UnixNano(), uploadFileName))
		file, err := os.OpenFile(uploadPath, os.O_CREATE|os.O_WRONLY|os.O_EXCL, config.FileMode)
		if err != nil {
			return nil, "", err
		}
		_, copyErr := io.Copy(file, part)
		file.Close()
		if copyErr != nil {
			return nil, uploadPath, httpapi.NewError(http.StatusBadRequest, "upload_interrupted", "The upload was interrupted. Nothing was changed. Try again.")
		}
		return fields, uploadPath, nil
	}
}

func translateRestoreError(err error) error {
	switch {
	case errors.Is(err, backup.ErrWrongSecret):
		return errRestoreSecret
	case errors.Is(err, backup.ErrIncompatible):
		return errRestoreIncompat
	case errors.Is(err, backup.ErrUnsafeEntry), errors.Is(err, backup.ErrArchiveTooLarge):
		return errRestoreUnsafe
	case errors.Is(err, backup.ErrDamaged):
		return errRestoreDamaged
	case errors.Is(err, syscall.ENOSPC), errors.Is(err, syscall.EDQUOT):
		return errRestoreNoSpace
	}
	return err
}

// swapData moves the staged data into place; tests replace it to act at that
// exact moment.
var swapData = backup.Swap

// maintenanceContext lasts as long as the server needs to finish or undo a
// swap, whatever happens to the request that started it.
func maintenanceContext(request context.Context) (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.WithoutCancel(request), config.RestoreFinishMaximumDuration)
}

// swapIn enters maintenance mode, stops every background worker, waits for
// other requests, swaps the staged data in, reopens the database and reloads
// the key. Any failure after the swap puts the previous data back and checks
// that it opens again. If that cannot be confirmed, Keeps stays unavailable
// and keeps every copy, instead of serving data in an unknown state.
func (a *Application) swapIn(ctx context.Context, staging string) error {
	a.gate.maintenance.Store(true)
	defer a.gate.maintenance.Store(false)
	a.pauseBackground()
	if err := a.gate.drain(); err != nil {
		a.resumeBackground()
		return err
	}
	a.blobLock.Lock()
	defer a.blobLock.Unlock()
	// The last moment an abandoned request leaves everything as it was.
	if err := ctx.Err(); err != nil {
		a.resumeBackground()
		return err
	}

	a.Store.Database.Close()
	rollback, err := swapData(a.Configuration.DataDirectory, staging)
	if err != nil {
		var rollbackFailed *backup.RollbackError
		if errors.As(err, &rollbackFailed) {
			return a.needRecovery(err)
		}
		if reopenErr := a.reopenCurrent(ctx); reopenErr != nil {
			return a.needRecovery(fmt.Errorf("%w; the previous data could not be opened again: %v", err, reopenErr))
		}
		a.resumeBackground()
		return err
	}
	finishing, cancel := maintenanceContext(ctx)
	err = a.afterSwap(finishing)
	cancel()
	if err == nil {
		slog.Info("restore completed", "rollback", rollback)
		a.resumeBackground()
		return nil
	}
	slog.Error("restore failed after swap; restoring previous data", "error", err)
	a.Store.Database.Close()
	if undoErr := backup.Undo(a.Configuration.DataDirectory, rollback); undoErr != nil {
		return a.needRecovery(&backup.RollbackError{Cause: err, Rollback: undoErr, Location: rollback})
	}
	if reopenErr := a.reopenCurrent(ctx); reopenErr != nil {
		return a.needRecovery(&backup.RollbackError{Cause: err, Rollback: reopenErr, Location: rollback})
	}
	if discardErr := backup.DiscardRollback(rollback); discardErr != nil {
		slog.Warn("previous data is back, but its rollback copy could not be removed", "rollback", rollback, "error", discardErr)
	}
	a.resumeBackground()
	return err
}

// reopenCurrent opens whatever data is now in place and loads its key, with
// its own time budget, so it still runs when the request or the budget for
// finishing the restore has ended.
func (a *Application) reopenCurrent(request context.Context) error {
	ctx, cancel := maintenanceContext(request)
	defer cancel()
	if err := a.Store.Reopen(ctx, ""); err != nil {
		return err
	}
	return a.Keys.Reload(a.keyPath())
}

// needRecovery leaves Keeps refusing every request, with workers stopped,
// until an administrator recovers the data by hand.
func (a *Application) needRecovery(err error) error {
	slog.Error("restore failed and the previous data could not be confirmed; Keeps is unavailable until recovered by hand (see docs/recovery.md)", "error", err)
	a.gate.recoveryNeeded.Store(true)
	return errRecoveryNeeded
}

func (a *Application) keyPath() string {
	return filepath.Join(a.Configuration.Directory(config.KeyDirectoryName), config.InstanceKeyFileName)
}

func (a *Application) afterSwap(ctx context.Context) error {
	if err := a.Store.Reopen(ctx, a.Configuration.Directory(config.BackupDirectoryName)); err != nil {
		return err
	}
	if err := a.Keys.Reload(a.keyPath()); err != nil {
		return err
	}
	return a.FinishRestore(ctx)
}

// FinishRestore signs everyone out so restored data never revives old access.
func (a *Application) FinishRestore(ctx context.Context) error {
	if _, err := a.Store.Database.ExecContext(ctx, `DELETE FROM sessions`); err != nil {
		return err
	}
	return audit.Record(ctx, a.Store.Database, 0, 0, audit.RestoreCompleted, "")
}

// WriteBackup is used by the CLI backup command.
func (a *Application) WriteBackup(ctx context.Context, destination io.Writer, recipients ...age.Recipient) (backup.Manifest, error) {
	a.blobLock.RLock()
	defer a.blobLock.RUnlock()
	return backup.Write(ctx, a.backupSources(), destination, recipients...)
}
