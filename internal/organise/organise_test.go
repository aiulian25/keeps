package organise

import (
	"context"
	"database/sql"
	"errors"
	"path/filepath"
	"testing"

	"github.com/aiulian25/keeps/internal/store"
)

func TestFailedReindexRollsBackRenameAndDelete(t *testing.T) {
	ctx := context.Background()
	database, err := store.Open(ctx, filepath.Join(t.TempDir(), "keeps.db"), "")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { database.Close() })
	database.Database.Exec(`INSERT INTO households (id, name, created_at) VALUES (1, 'Home', 'now')`)
	database.Database.Exec(`INSERT INTO rooms (id, household_id, name, created_at) VALUES (1, 1, 'Kitchen', 'now')`)
	database.Database.Exec(`INSERT INTO items (household_id, name, room_id, created_at, updated_at) VALUES (1, 'Kettle', 1, 'now', 'now')`)

	failing := NewService(database, func(context.Context, *sql.Tx, []int64) error { return errors.New("index unavailable") })
	err = database.Transaction(ctx, func(transaction *sql.Tx) error {
		_, err := failing.Rename(ctx, transaction, 1, "rooms", 1, "Galley")
		return err
	})
	if err == nil {
		t.Fatal("rename reported success although the search index was not updated")
	}
	if failing.Delete(ctx, 1, "rooms", 1, 0) == nil {
		t.Fatal("delete reported success although the search index was not updated")
	}
	var name string
	database.Database.QueryRow(`SELECT name FROM rooms WHERE id = 1`).Scan(&name)
	if name != "Kitchen" {
		t.Fatalf("a failed reindex left the room changed: %q", name)
	}
}
