package store

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func openTemporary(t *testing.T) (*Store, string) {
	t.Helper()
	directory := t.TempDir()
	store, err := Open(context.Background(), filepath.Join(directory, "keeps.db"), filepath.Join(directory, "backups"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { store.Close() })
	return store, directory
}

func TestMigrationsApplyOnceAndConstraintsHold(t *testing.T) {
	store, directory := openTemporary(t)
	ctx := context.Background()

	version, err := store.SchemaVersion(ctx)
	if err != nil || version != LatestSchemaVersion() {
		t.Fatalf("version %d err %v", version, err)
	}

	store.Close()
	reopened, err := Open(ctx, filepath.Join(directory, "keeps.db"), filepath.Join(directory, "backups"))
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()

	database := reopened.Database
	if _, err := database.Exec(`INSERT INTO items (household_id, name, created_at, updated_at) VALUES (999, 'x', 'now', 'now')`); err == nil {
		t.Fatal("foreign key violation was accepted")
	}
	database.Exec(`INSERT INTO households (id, name, created_at) VALUES (1, 'Home', 'now')`)
	if _, err := database.Exec(`INSERT INTO items (household_id, name, paid_unit_minor, created_at, updated_at) VALUES (1, 'x', 100, 'now', 'now')`); err == nil {
		t.Fatal("price without currency was accepted")
	}
	if _, err := database.Exec(`INSERT INTO items (household_id, name, created_at, updated_at) VALUES (1, '   ', 'now', 'now')`); err == nil {
		t.Fatal("blank name was accepted")
	}
	if _, err := database.Exec(`INSERT INTO purchases (household_id, purchased_on, created_at) VALUES (1, '2024-03-12', 'now')`); err == nil {
		t.Fatal("purchase date without precision was accepted")
	}
	if _, err := database.Exec(`INSERT INTO purchases (household_id, created_at) VALUES (1, 'now')`); err != nil {
		t.Fatalf("purchase with unknown date rejected: %v", err)
	}
}

func TestNewerSchemaRefused(t *testing.T) {
	store, directory := openTemporary(t)
	store.Database.Exec(`INSERT INTO schema_migrations (version, applied_at) VALUES (9999, 'now')`)
	store.Close()
	_, err := Open(context.Background(), filepath.Join(directory, "keeps.db"), "")
	if err != ErrSchemaTooNew {
		t.Fatalf("err = %v", err)
	}
}

func TestSnapshot(t *testing.T) {
	store, directory := openTemporary(t)
	destination := filepath.Join(directory, "copy.db")
	if err := store.Snapshot(context.Background(), destination); err != nil {
		t.Fatal(err)
	}
	if info, err := os.Stat(destination); err != nil || info.Size() == 0 {
		t.Fatalf("snapshot missing: %v", err)
	}
}
