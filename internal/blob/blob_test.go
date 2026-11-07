package blob

import (
	"bytes"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func newStore(t *testing.T) *Store {
	t.Helper()
	directory := t.TempDir()
	store, err := New(filepath.Join(directory, "blobs"), filepath.Join(directory, "tmp"))
	if err != nil {
		t.Fatal(err)
	}
	return store
}

type failingReader struct{ sent int }

func (r *failingReader) Read(buffer []byte) (int, error) {
	if r.sent > 0 {
		return 0, errors.New("connection reset")
	}
	r.sent = copy(buffer, "partial")
	return r.sent, nil
}

func TestWriteReadAndLimits(t *testing.T) {
	store := newStore(t)
	written, err := store.Write(strings.NewReader("invoice"), 100)
	if err != nil {
		t.Fatal(err)
	}
	file, err := store.Open(written.Key)
	if err != nil {
		t.Fatal(err)
	}
	content, _ := io.ReadAll(file)
	file.Close()
	if string(content) != "invoice" || written.Size != 7 || len(written.SHA256) != 64 {
		t.Fatalf("unexpected %q %+v", content, written)
	}
	info, _ := os.Stat(filepath.Join(store.root, written.Key[:2], written.Key))
	if info.Mode().Perm() != fileMode {
		t.Fatalf("mode %v", info.Mode().Perm())
	}

	if _, err := store.Write(bytes.NewReader(make([]byte, 101)), 100); err != ErrTooLarge {
		t.Fatalf("oversize err = %v", err)
	}
	if _, err := store.Write(&failingReader{}, 100); err == nil {
		t.Fatal("interrupted upload accepted")
	}
	leftovers, _ := os.ReadDir(store.temporary)
	if len(leftovers) != 0 {
		t.Fatalf("temporary files left behind: %d", len(leftovers))
	}
}

func TestPathRejectsTraversal(t *testing.T) {
	store := newStore(t)
	for _, key := range []string{"../../etc/passwd", "", "ABCDEF", strings.Repeat("g", 32)} {
		if _, err := store.Path(key); err != ErrInvalidKey {
			t.Errorf("key %q accepted", key)
		}
	}
}

func TestReconcileKeepsReferencedAndRecent(t *testing.T) {
	store := newStore(t)
	referenced, _ := store.Write(strings.NewReader("a"), 10)
	orphan, _ := store.Write(strings.NewReader("b"), 10)
	recentOrphan, _ := store.Write(strings.NewReader("c"), 10)
	old := time.Now().Add(-48 * time.Hour)
	for _, key := range []string{referenced.Key, orphan.Key} {
		path, _ := store.Path(key)
		os.Chtimes(path, old, old)
	}
	abandoned, _ := store.TemporaryFile()
	abandoned.Close()
	os.Chtimes(abandoned.Name(), old, old)

	removed, err := store.Reconcile(func(key string) (bool, error) { return key == referenced.Key, nil }, 24*time.Hour)
	if err != nil || removed != 2 {
		t.Fatalf("removed %d err %v", removed, err)
	}
	for key, shouldExist := range map[string]bool{referenced.Key: true, orphan.Key: false, recentOrphan.Key: true} {
		path, _ := store.Path(key)
		_, statErr := os.Stat(path)
		if (statErr == nil) != shouldExist {
			t.Errorf("key %s exists=%v want %v", key, statErr == nil, shouldExist)
		}
	}
}
