package main

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/aiulian25/keeps/internal/backup"
	"github.com/aiulian25/keeps/internal/config"
	"github.com/aiulian25/keeps/internal/server"
)

const testPassphrase = "a long enough backup passphrase"

// backupWithReceipt writes an encrypted backup of a fresh instance holding one
// receipt; removeReceipt deletes the stored file first.
func backupWithReceipt(t *testing.T, removeReceipt bool) string {
	t.Helper()
	data := t.TempDir()
	configuration, err := config.Load(func(name string) (string, bool) {
		values := map[string]string{config.EnvironmentOrigin: "https://keeps.example.com", config.EnvironmentDataDirectory: data}
		value, found := values[name]
		return value, found
	})
	if err != nil {
		t.Fatal(err)
	}
	application, err := server.Open(context.Background(), configuration)
	if err != nil {
		t.Fatal(err)
	}
	defer application.Close()
	application.Store.Database.Exec(`INSERT INTO households (id, name, created_at) VALUES (1, 'Home', 'now')`)
	receipt, err := application.Attachments.Save(context.Background(), 1, "invoice", "receipt.pdf", "application/pdf", strings.NewReader("%PDF-1.4\nreceipt\n%%EOF"))
	if err != nil {
		t.Fatal(err)
	}
	if removeReceipt {
		var key string
		application.Store.Database.QueryRow(`SELECT storage_key FROM attachments WHERE id = ?`, receipt.ID).Scan(&key)
		path, _ := application.Blobs.Path(key)
		os.Remove(path)
	}
	recipient, _ := backup.PassphraseRecipient(testPassphrase)
	var archive bytes.Buffer
	if _, err := application.WriteBackup(context.Background(), &archive, recipient); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "backup.keeps.age")
	os.WriteFile(path, archive.Bytes(), 0o600)
	return path
}

func passphraseFile(t *testing.T, passphrase string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "passphrase.txt")
	os.WriteFile(path, []byte(passphrase+"\n"), 0o600)
	return path
}

func TestVerifyBackupCommandAgreesWithRestore(t *testing.T) {
	scratch := t.TempDir()
	t.Setenv("TMPDIR", scratch)
	good := passphraseFile(t, testPassphrase)

	if err := verifyBackupCommand([]string{backupWithReceipt(t, false), "--passphrase-file", good}); err != nil {
		t.Fatalf("a complete backup failed verification: %v", err)
	}
	err := verifyBackupCommand([]string{backupWithReceipt(t, true), "--passphrase-file", good})
	if err == nil || !strings.Contains(err.Error(), "--accept-incomplete") {
		t.Fatalf("an incomplete backup verified: %v", err)
	}
	if err := verifyBackupCommand([]string{backupWithReceipt(t, false), "--passphrase-file", passphraseFile(t, "the wrong passphrase")}); err == nil || !strings.Contains(err.Error(), "NOT usable") {
		t.Fatalf("a wrong passphrase verified: %v", err)
	}
	if leftovers, _ := filepath.Glob(filepath.Join(scratch, "keeps-verify-*")); len(leftovers) != 0 {
		t.Fatalf("verification left plaintext behind: %v", leftovers)
	}
}
