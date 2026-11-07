package backup

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"filippo.io/age"

	"github.com/aiulian25/keeps/internal/config"
)

const (
	rollbackPrefix     = "rollback-"
	timestampFormat    = "20060102T150405Z"
	walSuffix          = "-wal"
	sharedMemorySuffix = "-shm"
	shardLength        = 2
)

// Stage decrypts, validates and extracts an archive into a fresh staging
// directory. Live data is never touched; on any error staging is removed.
func Stage(ctx context.Context, dataDirectory string, source io.Reader, identity age.Identity) (string, Manifest, error) {
	stagingRoot := filepath.Join(dataDirectory, config.StagingDirectoryName)
	if err := os.MkdirAll(stagingRoot, stagedDirectoryMode); err != nil {
		return "", Manifest{}, err
	}
	staging, err := os.MkdirTemp(stagingRoot, "restore-")
	if err != nil {
		return "", Manifest{}, err
	}
	manifest, err := Read(ctx, source, identity, staging)
	if err == nil {
		err = CheckDatabase(ctx, filepath.Join(staging, databasePath))
	}
	if err != nil {
		os.RemoveAll(staging)
		return "", Manifest{}, err
	}
	if err := os.MkdirAll(filepath.Join(staging, "blobs"), stagedDirectoryMode); err != nil {
		os.RemoveAll(staging)
		return "", Manifest{}, err
	}
	if err := checkReferences(ctx, staging, &manifest); err != nil {
		os.RemoveAll(staging)
		return "", Manifest{}, err
	}
	return staging, manifest, nil
}

// checkReferences compares the staged database with the staged files, so an
// archive that silently lacks a file (an old or altered manifest) is still
// reported as incomplete.
func checkReferences(ctx context.Context, staging string, manifest *Manifest) error {
	stagedDatabase := filepath.Join(staging, databasePath)
	_, keys, err := inspectSnapshot(ctx, stagedDatabase)
	if err != nil {
		return ErrDamaged
	}
	var missing []string
	for _, key := range keys {
		if !allowedEntry.MatchString(blobPrefix + shard(key) + "/" + key) {
			missing = append(missing, key)
			continue
		}
		if _, err := os.Stat(filepath.Join(staging, "blobs", shard(key), key)); err != nil {
			missing = append(missing, key)
		}
	}
	manifest.Missing = missing
	manifest.MissingFiles, err = describeMissing(ctx, stagedDatabase, missing)
	return err
}

func shard(key string) string {
	if len(key) < shardLength {
		return key
	}
	return key[:shardLength]
}

type move struct {
	from string
	to   string
}

// rename and removeAll are replaced in tests to inject filesystem failures.
var (
	rename    = os.Rename
	removeAll = os.RemoveAll
)

// RollbackError means a restore failed and putting the previous data back
// failed too. The previous data is left in Location for manual recovery.
type RollbackError struct {
	Cause    error
	Rollback error
	Location string
}

func (e *RollbackError) Error() string {
	return fmt.Sprintf("%v; putting the previous data back also failed (%v). The previous data is kept in %s", e.Cause, e.Rollback, e.Location)
}

func (e *RollbackError) Unwrap() error { return e.Cause }

// Swap replaces the live database, key and files with the staged ones. The
// previous data is kept in a rollback directory; if any step fails, every
// completed move is reversed and checked. The database must be closed by the
// caller.
func Swap(dataDirectory string, staging string) (string, error) {
	rollback := filepath.Join(dataDirectory, rollbackPrefix+time.Now().UTC().Format(timestampFormat))
	if err := os.MkdirAll(rollback, stagedDirectoryMode); err != nil {
		return "", err
	}
	live := func(name string) string { return filepath.Join(dataDirectory, name) }
	plan := []move{
		{live(config.DatabaseFileName), filepath.Join(rollback, config.DatabaseFileName)},
		{live(config.DatabaseFileName + walSuffix), filepath.Join(rollback, config.DatabaseFileName+walSuffix)},
		{live(config.DatabaseFileName + sharedMemorySuffix), filepath.Join(rollback, config.DatabaseFileName+sharedMemorySuffix)},
		{live(config.KeyDirectoryName), filepath.Join(rollback, config.KeyDirectoryName)},
		{live(config.BlobDirectoryName), filepath.Join(rollback, config.BlobDirectoryName)},
		{filepath.Join(staging, databasePath), live(config.DatabaseFileName)},
		{filepath.Join(staging, "keys"), live(config.KeyDirectoryName)},
		{filepath.Join(staging, "blobs"), live(config.BlobDirectoryName)},
	}
	var done []move
	for _, step := range plan {
		err := rename(step.from, step.to)
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err != nil {
			cause := fmt.Errorf("restore swap failed: %w", err)
			if undoErr := undo(done); undoErr != nil {
				return "", &RollbackError{Cause: cause, Rollback: undoErr, Location: rollback}
			}
			removeAll(rollback)
			return "", fmt.Errorf("%w; the previous data was put back", cause)
		}
		done = append(done, step)
	}
	removeAll(staging)
	return rollback, nil
}

// undo reverses completed moves, newest first, and reports every failure.
func undo(done []move) error {
	var failures []error
	for index := len(done) - 1; index >= 0; index-- {
		if err := rename(done[index].to, done[index].from); err != nil {
			failures = append(failures, err)
		}
	}
	return errors.Join(failures...)
}

// Undo moves rollback data back into place after a failed post-swap step.
// The database and key must be in the rollback directory; the file store,
// WAL and shared-memory files are optional because a new instance may not
// have had them. The rollback directory is kept: the caller removes it with
// DiscardRollback only after the previous data has opened again.
func Undo(dataDirectory string, rollback string) error {
	var failures []error
	required := map[string]bool{config.DatabaseFileName: true, config.KeyDirectoryName: true}
	for _, name := range []string{config.DatabaseFileName, config.KeyDirectoryName, config.BlobDirectoryName} {
		saved := filepath.Join(rollback, name)
		if _, err := os.Stat(saved); err != nil {
			if required[name] {
				failures = append(failures, fmt.Errorf("%s is missing from %s: %w", name, rollback, err))
			}
			continue
		}
		live := filepath.Join(dataDirectory, name)
		if err := removeAll(live); err != nil {
			failures = append(failures, fmt.Errorf("could not clear %s: %w", name, err))
			continue
		}
		if err := rename(saved, live); err != nil {
			failures = append(failures, fmt.Errorf("could not move %s back: %w", name, err))
		}
	}
	for _, suffix := range []string{walSuffix, sharedMemorySuffix} {
		name := config.DatabaseFileName + suffix
		live := filepath.Join(dataDirectory, name)
		if err := removeAll(live); err != nil {
			failures = append(failures, fmt.Errorf("could not clear %s: %w", name, err))
			continue
		}
		err := rename(filepath.Join(rollback, name), live)
		if err != nil && !errors.Is(err, os.ErrNotExist) {
			failures = append(failures, fmt.Errorf("could not move %s back: %w", name, err))
		}
	}
	return errors.Join(failures...)
}

// DiscardRollback removes a rollback directory once the data it protected
// is no longer needed.
func DiscardRollback(rollback string) error {
	return removeAll(rollback)
}
