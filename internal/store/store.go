// Package store owns the SQLite connection, migrations and transactions.
package store

import (
	"context"
	"database/sql"
	"embed"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"modernc.org/sqlite"
)

//go:embed migrations/*.sql
var migrationFiles embed.FS

const (
	connectionOptions  = "?_pragma=foreign_keys(1)&_pragma=journal_mode(WAL)&_pragma=busy_timeout(10000)&_pragma=synchronous(NORMAL)&_txlock=immediate"
	maximumConnections = 4
	migrationDirectory = "migrations"
	timestampLayout    = time.RFC3339Nano
)

var ErrSchemaTooNew = errors.New("the database was created by a newer version of Keeps; use that version or restore a compatible backup")

type Store struct {
	Database *sql.DB
	path     string
}

// Open opens the database, then applies pending migrations. A snapshot of a
// non-empty database is written to snapshotDirectory before any migration.
func Open(ctx context.Context, path string, snapshotDirectory string) (*Store, error) {
	database, err := sql.Open("sqlite", path+connectionOptions)
	if err != nil {
		return nil, err
	}
	database.SetMaxOpenConns(maximumConnections)
	store := &Store{Database: database, path: path}
	if err := store.migrate(ctx, snapshotDirectory); err != nil {
		database.Close()
		return nil, err
	}
	return store, nil
}

func (s *Store) Close() error {
	return s.Database.Close()
}

// Reopen closes and reopens the database file, applying migrations. It is
// used after a restore has replaced the file; callers must hold the
// application's maintenance lock so no request is using the old handle.
func (s *Store) Reopen(ctx context.Context, snapshotDirectory string) error {
	s.Database.Close()
	database, err := sql.Open("sqlite", s.path+connectionOptions)
	if err != nil {
		return err
	}
	database.SetMaxOpenConns(maximumConnections)
	s.Database = database
	return s.migrate(ctx, snapshotDirectory)
}

func (s *Store) Path() string {
	return s.path
}

// Transaction runs work inside one immediate transaction.
func (s *Store) Transaction(ctx context.Context, work func(*sql.Tx) error) error {
	transaction, err := s.Database.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	if err := work(transaction); err != nil {
		transaction.Rollback()
		return err
	}
	return transaction.Commit()
}

// Snapshot writes a consistent copy of the live database to destination using
// SQLite's online backup API.
func (s *Store) Snapshot(ctx context.Context, destination string) error {
	connection, err := s.Database.Conn(ctx)
	if err != nil {
		return err
	}
	defer connection.Close()
	return connection.Raw(func(driverConnection any) error {
		backupSource, ok := driverConnection.(interface {
			NewBackup(string) (*sqlite.Backup, error)
		})
		if !ok {
			return errors.New("sqlite driver does not support online backup")
		}
		backup, err := backupSource.NewBackup(destination)
		if err != nil {
			return err
		}
		for more := true; more; {
			more, err = backup.Step(-1)
			if err != nil {
				backup.Finish()
				return err
			}
		}
		return backup.Finish()
	})
}

type migration struct {
	version int
	name    string
}

func availableMigrations() ([]migration, error) {
	entries, err := fs.ReadDir(migrationFiles, migrationDirectory)
	if err != nil {
		return nil, err
	}
	var migrations []migration
	for _, entry := range entries {
		prefix, _, found := strings.Cut(entry.Name(), "_")
		if !found {
			continue
		}
		version, err := strconv.Atoi(prefix)
		if err != nil {
			return nil, fmt.Errorf("migration %s has no numeric prefix", entry.Name())
		}
		migrations = append(migrations, migration{version: version, name: entry.Name()})
	}
	sort.Slice(migrations, func(left, right int) bool { return migrations[left].version < migrations[right].version })
	return migrations, nil
}

// LatestSchemaVersion is the schema version this binary produces.
func LatestSchemaVersion() int {
	migrations, err := availableMigrations()
	if err != nil || len(migrations) == 0 {
		return 0
	}
	return migrations[len(migrations)-1].version
}

func (s *Store) SchemaVersion(ctx context.Context) (int, error) {
	if _, err := s.Database.ExecContext(ctx, `CREATE TABLE IF NOT EXISTS schema_migrations (version INTEGER PRIMARY KEY, applied_at TEXT NOT NULL)`); err != nil {
		return 0, err
	}
	var version int
	err := s.Database.QueryRowContext(ctx, `SELECT coalesce(max(version), 0) FROM schema_migrations`).Scan(&version)
	return version, err
}

func (s *Store) migrate(ctx context.Context, snapshotDirectory string) error {
	migrations, err := availableMigrations()
	if err != nil {
		return err
	}
	current, err := s.SchemaVersion(ctx)
	if err != nil {
		return err
	}
	latest := LatestSchemaVersion()
	if current > latest {
		return ErrSchemaTooNew
	}
	if current == latest {
		return nil
	}

	if current > 0 && snapshotDirectory != "" {
		if err := os.MkdirAll(snapshotDirectory, 0o700); err != nil {
			return err
		}
		snapshotPath := filepath.Join(snapshotDirectory, fmt.Sprintf("pre-migration-%d-%s.db", current, time.Now().UTC().Format("20060102T150405")))
		if err := s.Snapshot(ctx, snapshotPath); err != nil {
			return fmt.Errorf("pre-migration snapshot failed: %w", err)
		}
	}

	for _, pending := range migrations {
		if pending.version <= current {
			continue
		}
		script, err := migrationFiles.ReadFile(migrationDirectory + "/" + pending.name)
		if err != nil {
			return err
		}
		err = s.Transaction(ctx, func(transaction *sql.Tx) error {
			if _, err := transaction.ExecContext(ctx, string(script)); err != nil {
				return fmt.Errorf("migration %s: %w", pending.name, err)
			}
			_, err := transaction.ExecContext(ctx, `INSERT INTO schema_migrations (version, applied_at) VALUES (?, ?)`, pending.version, Now())
			return err
		})
		if err != nil {
			return err
		}
	}
	return nil
}

// Now returns the current time in the stored timestamp format.
func Now() string {
	return FormatTime(time.Now())
}

func FormatTime(moment time.Time) string {
	return moment.UTC().Format(timestampLayout)
}

func ParseTime(value string) (time.Time, error) {
	return time.Parse(timestampLayout, value)
}

// IsUniqueViolation reports whether err is a UNIQUE constraint failure.
func IsUniqueViolation(err error) bool {
	return err != nil && strings.Contains(err.Error(), "UNIQUE constraint failed")
}
