package backup

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// instance writes a minimal data directory and a staging directory with
// different contents, so every file shows which generation it came from.
func instance(t *testing.T) (string, string) {
	t.Helper()
	data := t.TempDir()
	staging := filepath.Join(t.TempDir(), "staging")
	for root, generation := range map[string]string{data: "old", staging: "new"} {
		for _, name := range []string{"keeps.db", "keys/instance.key", "blobs/aa/aa00"} {
			path := filepath.Join(root, name)
			os.MkdirAll(filepath.Dir(path), 0o700)
			os.WriteFile(path, []byte(generation), 0o600)
		}
	}
	return data, staging
}

func content(t *testing.T, path string) string {
	t.Helper()
	read, err := os.ReadFile(path)
	if err != nil {
		return "missing"
	}
	return string(read)
}

// failRename makes the rename numbered failAt (counting from 1) fail, and
// optionally every rename back out of the rollback directory.
func failRename(t *testing.T, failAt int, failReverse bool) {
	t.Helper()
	count := 0
	rename = func(from string, to string) error {
		count++
		if count == failAt {
			return errors.New("injected rename failure")
		}
		if failReverse && strings.Contains(from, rollbackPrefix) {
			return errors.New("injected reverse failure")
		}
		return os.Rename(from, to)
	}
	t.Cleanup(func() { rename = os.Rename })
}

func rollbackDirectories(data string) []string {
	found, _ := filepath.Glob(filepath.Join(data, rollbackPrefix+"*"))
	return found
}

func TestSwapFailureWithWorkingUndoPutsPreviousDataBack(t *testing.T) {
	data, staging := instance(t)
	failRename(t, 7, false)
	_, err := Swap(data, staging)
	var rollbackFailed *RollbackError
	if err == nil || errors.As(err, &rollbackFailed) || !strings.Contains(err.Error(), "put back") {
		t.Fatalf("expected a clean rollback, got %v", err)
	}
	if content(t, filepath.Join(data, "keeps.db")) != "old" || content(t, filepath.Join(data, "keys/instance.key")) != "old" {
		t.Fatal("previous data is not back in place")
	}
	if len(rollbackDirectories(data)) != 0 {
		t.Fatal("an empty rollback directory was left behind")
	}
}

func TestSwapFailureWithFailingUndoIsReportedAndKeepsTheCopy(t *testing.T) {
	data, staging := instance(t)
	failRename(t, 7, true)
	_, err := Swap(data, staging)
	var rollbackFailed *RollbackError
	if !errors.As(err, &rollbackFailed) || strings.Contains(err.Error(), "was put back") {
		t.Fatalf("a failed rollback was not reported: %v", err)
	}
	if content(t, filepath.Join(rollbackFailed.Location, "keeps.db")) != "old" {
		t.Fatal("the remaining copy of the previous data was not kept where the error says")
	}
}

func TestUndoChecksEveryStepAndKeepsTheRollbackUntilDiscarded(t *testing.T) {
	data, staging := instance(t)
	rollback, err := Swap(data, staging)
	if err != nil {
		t.Fatal(err)
	}
	os.WriteFile(filepath.Join(data, "keeps.db-wal"), []byte("new"), 0o600)
	if err := Undo(data, rollback); err != nil {
		t.Fatal(err)
	}
	if content(t, filepath.Join(data, "keeps.db")) != "old" || content(t, filepath.Join(data, "blobs/aa/aa00")) != "old" {
		t.Fatal("undo did not put the previous data back")
	}
	if content(t, filepath.Join(data, "keeps.db-wal")) != "missing" {
		t.Fatal("the restored database's WAL was left beside the previous database")
	}
	if _, err := os.Stat(rollback); err != nil {
		t.Fatal("undo removed the rollback directory before the caller confirmed the data")
	}
	if err := DiscardRollback(rollback); err != nil || len(rollbackDirectories(data)) != 0 {
		t.Fatalf("discard: %v", err)
	}
}

func TestUndoReportsMissingRequiredDataAndFailedRemovals(t *testing.T) {
	data, staging := instance(t)
	rollback, _ := Swap(data, staging)
	os.Remove(filepath.Join(rollback, "keeps.db"))
	if err := Undo(data, rollback); err == nil || !strings.Contains(err.Error(), "keeps.db is missing") {
		t.Fatalf("missing rollback database not reported: %v", err)
	}

	data, staging = instance(t)
	rollback, _ = Swap(data, staging)
	removeAll = func(path string) error {
		if strings.HasSuffix(path, "keys") {
			return errors.New("injected removal failure")
		}
		return os.RemoveAll(path)
	}
	t.Cleanup(func() { removeAll = os.RemoveAll })
	if err := Undo(data, rollback); err == nil || !strings.Contains(err.Error(), "could not clear keys") {
		t.Fatalf("failed removal not reported: %v", err)
	}
	if content(t, filepath.Join(rollback, "keys/instance.key")) != "old" {
		t.Fatal("the previous key was lost after a failed removal")
	}
}
