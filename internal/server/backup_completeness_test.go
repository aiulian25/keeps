package server

import (
	"archive/tar"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"

	"filippo.io/age"

	"github.com/aiulian25/keeps/internal/backup"
	"github.com/aiulian25/keeps/internal/config"
)

// itemWithInvoice saves "Coffee machine" with receipt.pdf and returns the
// invoice's storage key.
func itemWithInvoice(t *testing.T, server *testServer, client *testClient) string {
	t.Helper()
	invoice := client.upload("invoice", "receipt.pdf", "application/pdf", []byte("%PDF-1.4\nreceipt\n%%EOF"))
	var body struct {
		ID int64 `json:"id"`
	}
	invoice.json(t, &body)
	client.createItem(map[string]any{"name": "Coffee machine", "attachmentIds": []int64{body.ID}})
	var key string
	server.application.Store.Database.QueryRow(`SELECT storage_key FROM attachments WHERE id = ?`, body.ID).Scan(&key)
	return key
}

func removeBlob(t *testing.T, server *testServer, key string) {
	t.Helper()
	path, err := server.application.Blobs.Path(key)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
}

func (c *testClient) restoreAccepting(archive []byte, accept bool) response {
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	writer.WriteField("passphrase", backupPassphrase)
	if accept {
		writer.WriteField("acceptIncomplete", "1")
	}
	part, _ := writer.CreateFormFile("file", "backup.keeps.age")
	part.Write(archive)
	writer.Close()
	request, _ := http.NewRequest(http.MethodPost, c.server.http.URL+"/api/v1/restore", &body)
	request.Header.Set("Content-Type", writer.FormDataContentType())
	request.Header.Set("Origin", c.origin)
	request.Header.Set("X-CSRF-Token", c.csrf)
	result, err := c.client.Do(request)
	if err != nil {
		c.server.t.Fatal(err)
	}
	defer result.Body.Close()
	content, _ := io.ReadAll(result.Body)
	return response{Status: result.StatusCode, Body: content}
}

func itemCount(server *testServer, name string) int {
	var count int
	server.application.Store.Database.QueryRow(`SELECT count(*) FROM items WHERE name = ?`, name).Scan(&count)
	return count
}

func preparedLeftovers(t *testing.T, server *testServer) []string {
	t.Helper()
	leftovers, _ := filepath.Glob(filepath.Join(server.application.temporaryDirectory(), preparedFilePattern))
	return leftovers
}

func TestIncompleteBackupIsNamedAndNeedsConsentToRestore(t *testing.T) {
	server := newTestServer(t)
	client := server.claimed()
	removeBlob(t, server, itemWithInvoice(t, server, client))

	prepared := client.do(http.MethodPost, "/api/v1/backups", map[string]string{"passphrase": backupPassphrase})
	var info preparedFile
	prepared.json(t, &info)
	if prepared.Status != http.StatusOK || info.Complete {
		t.Fatalf("a backup missing an invoice was reported complete: %d %s", prepared.Status, prepared.Body)
	}
	expected := backup.MissingFile{Filename: "receipt.pdf", Copy: backup.CopyOriginal, Items: []string{"Coffee machine"}, Problem: backup.ProblemMissing}
	if len(info.MissingFiles) != 1 || fmt.Sprint(info.MissingFiles[0]) != fmt.Sprint(expected) {
		t.Fatalf("missing file not named: %+v", info.MissingFiles)
	}
	archive := client.do(http.MethodGet, info.DownloadURL, nil)
	if archive.Status != http.StatusOK {
		t.Fatalf("incomplete backup not offered for recovery: %d", archive.Status)
	}

	identity, _ := age.NewScryptIdentity(backupPassphrase)
	manifest, err := backup.Read(context.Background(), bytes.NewReader(archive.Body), identity, "")
	if err != nil || manifest.Complete() || len(manifest.MissingFiles) != 1 {
		t.Fatalf("verification hid the missing file: %v %+v", err, manifest)
	}

	client.createItem(map[string]any{"name": "After backup"})
	refused := client.restoreAccepting(archive.Body, false)
	if refused.Status != http.StatusConflict || refused.code() != "backup_incomplete" || !strings.Contains(string(refused.Body), "receipt.pdf") {
		t.Fatalf("incomplete restore was not explained first: %d %s", refused.Status, refused.Body)
	}
	if itemCount(server, "After backup") != 1 {
		t.Fatal("a refused restore changed live data")
	}
	staged, _ := filepath.Glob(filepath.Join(server.application.Configuration.Directory(config.StagingDirectoryName), "restore-*"))
	if len(staged) != 0 {
		t.Fatalf("refused restore left staging behind: %v", staged)
	}

	accepted := client.restoreAccepting(archive.Body, true)
	if accepted.Status != http.StatusOK || !strings.Contains(string(accepted.Body), "receipt.pdf") {
		t.Fatalf("accepted incomplete restore: %d %s", accepted.Status, accepted.Body)
	}
	if itemCount(server, "After backup") != 0 || itemCount(server, "Coffee machine") != 1 {
		t.Fatal("accepted restore did not bring back the backed-up items")
	}
}

// An archive whose manifest does not admit a missing file (made by an older
// or altered writer) is still found incomplete when staged.
func TestStagingChecksReferencesNotJustTheManifest(t *testing.T) {
	server := newTestServer(t)
	client := server.claimed()
	key := itemWithInvoice(t, server, client)
	complete := client.fetchDownload("/api/v1/backups", map[string]string{"passphrase": backupPassphrase})
	if complete.Status != http.StatusOK {
		t.Fatalf("backup: %d %s", complete.Status, complete.Body)
	}
	silent := withoutBlob(t, complete.Body, key)

	identity, _ := age.NewScryptIdentity(backupPassphrase)
	manifest, err := backup.Read(context.Background(), bytes.NewReader(silent), identity, "")
	if err != nil || !manifest.Complete() {
		t.Fatalf("fixture should look complete to the manifest alone: %v %+v", err, manifest)
	}
	fresh := newTestServer(t)
	staging, staged, err := backup.Stage(context.Background(), fresh.application.Configuration.DataDirectory, bytes.NewReader(silent), identity)
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(staging)
	if staged.Complete() || len(staged.MissingFiles) != 1 || staged.MissingFiles[0].Filename != "receipt.pdf" {
		t.Fatalf("staging trusted the manifest: %+v", staged)
	}

	// Standalone verification reaches the same verdict as staging, and keeps
	// nothing behind.
	scratch := t.TempDir()
	verified, err := backup.Verify(context.Background(), bytes.NewReader(silent), identity, scratch)
	if err != nil || verified.Complete() || len(verified.MissingFiles) != 1 || verified.MissingFiles[0].Filename != "receipt.pdf" {
		t.Fatalf("verification disagreed with staging: %v %+v", err, verified)
	}
	if good, err := backup.Verify(context.Background(), bytes.NewReader(complete.Body), identity, scratch); err != nil || !good.Complete() {
		t.Fatalf("a complete archive did not verify: %v %+v", err, good)
	}
	wrongIdentity, _ := age.NewScryptIdentity("not the passphrase")
	if _, err := backup.Verify(context.Background(), bytes.NewReader(complete.Body), wrongIdentity, scratch); !errors.Is(err, backup.ErrWrongSecret) {
		t.Fatalf("a wrong passphrase: %v", err)
	}
	corrupted := append([]byte{}, complete.Body...)
	corrupted[len(corrupted)-40] ^= 0xff
	if _, err := backup.Verify(context.Background(), bytes.NewReader(corrupted), identity, scratch); err == nil {
		t.Fatal("a corrupted archive verified")
	}
	if leftovers, _ := os.ReadDir(scratch); len(leftovers) != 0 {
		t.Fatalf("verification left plaintext behind: %v", leftovers)
	}
}

// withoutBlob re-encrypts an archive with one file and its manifest entry removed.
func withoutBlob(t *testing.T, archive []byte, key string) []byte {
	return rewriteArchive(t, archive, func(name string, content []byte) ([]byte, bool) {
		return content, !strings.HasSuffix(name, key)
	})
}

// rewriteArchive re-encrypts an archive after edit has changed or dropped
// entries, keeping the manifest consistent with what remains.
func rewriteArchive(t *testing.T, archive []byte, edit func(name string, content []byte) ([]byte, bool)) []byte {
	t.Helper()
	identity, _ := age.NewScryptIdentity(backupPassphrase)
	decrypted, err := age.Decrypt(bytes.NewReader(archive), identity)
	if err != nil {
		t.Fatal(err)
	}
	reader := tar.NewReader(decrypted)
	type entry struct {
		header  *tar.Header
		content []byte
	}
	var entries []entry
	files := map[string]backup.FileEntry{}
	for {
		header, err := reader.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			t.Fatal(err)
		}
		content, _ := io.ReadAll(reader)
		if header.Name != "manifest.json" {
			edited, keep := edit(header.Name, content)
			if !keep {
				continue
			}
			content = edited
			sum := sha256.Sum256(content)
			files[header.Name] = backup.FileEntry{Path: header.Name, Size: int64(len(content)), SHA256: hex.EncodeToString(sum[:])}
		}
		header.Size = int64(len(content))
		entries = append(entries, entry{header, content})
	}
	var plain bytes.Buffer
	writer := tar.NewWriter(&plain)
	for _, item := range entries {
		if item.header.Name == "manifest.json" {
			var manifest backup.Manifest
			json.Unmarshal(item.content, &manifest)
			var kept []backup.FileEntry
			for _, file := range manifest.Files {
				if updated, found := files[file.Path]; found {
					kept = append(kept, updated)
				}
			}
			manifest.Files = kept
			item.content, _ = json.Marshal(manifest)
			item.header.Size = int64(len(item.content))
		}
		writer.WriteHeader(item.header)
		writer.Write(item.content)
	}
	writer.Close()
	recipient, _ := age.NewScryptRecipient(backupPassphrase)
	recipient.SetWorkFactor(10)
	var encrypted bytes.Buffer
	encrypter, _ := age.Encrypt(&encrypted, recipient)
	encrypter.Write(plain.Bytes())
	encrypter.Close()
	return encrypted.Bytes()
}

func TestArchiveExportWithMissingFileFailsInsteadOfDownloadingPartially(t *testing.T) {
	server := newTestServer(t)
	client := server.claimed()
	removeBlob(t, server, itemWithInvoice(t, server, client))

	result := client.do(http.MethodPost, "/api/v1/exports", map[string]string{"format": "archive"})
	if result.code() != "export_file_missing" || !strings.Contains(string(result.Body), "receipt.pdf") || result.Header.Get("Content-Disposition") != "" {
		t.Fatalf("partial export offered: %d %s", result.Status, result.Body)
	}
	if leftovers := preparedLeftovers(t, server); len(leftovers) != 0 {
		t.Fatalf("failed export left files: %v", leftovers)
	}
	if spreadsheet := client.fetchDownload("/api/v1/exports", map[string]string{"format": "csv"}); spreadsheet.Status != http.StatusOK {
		t.Fatalf("spreadsheet export should still work: %d", spreadsheet.Status)
	}
}

func TestPreparedDownloadsReportWriteFailuresAndAreSingleUse(t *testing.T) {
	server := newTestServer(t)
	client := server.claimed()
	directory := server.application.temporaryDirectory()

	failures := map[error]error{
		fmt.Errorf("writing: %w", syscall.ENOSPC): errNotEnoughSpace,
		errors.New("read error"):                  errPrepareFailed,
	}
	for cause, expected := range failures {
		_, err := server.application.downloads.prepare(directory, 1, "x", "text/plain", func(destination io.Writer) error {
			destination.Write([]byte("partial"))
			return cause
		})
		if !errors.Is(err, expected) {
			t.Errorf("%v reported as %v", cause, err)
		}
	}
	if leftovers := preparedLeftovers(t, server); len(leftovers) != 0 {
		t.Fatalf("failed preparation left files: %v", leftovers)
	}
	if restoreErr := translateRestoreError(fmt.Errorf("unpacking: %w", syscall.ENOSPC)); !errors.Is(restoreErr, errRestoreNoSpace) {
		t.Errorf("a full disk during restore reported as %v", restoreErr)
	}

	stale := filepath.Join(directory, "download-stale")
	os.WriteFile(stale, []byte("old"), config.FileMode)
	old := time.Now().Add(-2 * preparedDownloadLifetime)
	os.Chtimes(stale, old, old)

	prepared := client.do(http.MethodPost, "/api/v1/backups", map[string]string{"passphrase": backupPassphrase})
	var info preparedFile
	prepared.json(t, &info)
	if !info.Complete || len(info.MissingFiles) != 0 {
		t.Fatalf("complete backup reported incomplete: %s", prepared.Body)
	}
	if _, err := os.Stat(stale); !os.IsNotExist(err) {
		t.Fatal("an expired prepared file was not removed")
	}
	if first := client.do(http.MethodGet, info.DownloadURL, nil); first.Status != http.StatusOK || !strings.Contains(first.Header.Get("Content-Disposition"), ".keeps.age") {
		t.Fatalf("download: %d %s", first.Status, first.Header)
	}
	if second := client.do(http.MethodGet, info.DownloadURL, nil); second.code() != "download_expired" {
		t.Fatalf("prepared file served twice: %d", second.Status)
	}
	if leftovers := preparedLeftovers(t, server); len(leftovers) != 0 {
		t.Fatalf("downloaded file kept on the server: %v", leftovers)
	}
	if _, found := server.application.downloads.take(strings.TrimPrefix(info.DownloadURL, downloadPathPrefix), 999); found {
		t.Fatal("another user could take a prepared file")
	}
}

// replaceBlobKeepingSize overwrites a stored original with different bytes of
// the same length, as silent storage damage would.
func replaceBlobKeepingSize(t *testing.T, server *testServer, key string) {
	t.Helper()
	path, err := server.application.Blobs.Path(key)
	if err != nil {
		t.Fatal(err)
	}
	original, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	changed := append([]byte{}, original...)
	changed[len(changed)/2] ^= 0xff
	if err := os.WriteFile(path, changed, 0o600); err != nil {
		t.Fatal(err)
	}
}

func TestDamagedOriginalNeverCountsAsComplete(t *testing.T) {
	server := newTestServer(t)
	client := server.claimed()
	key := itemWithInvoice(t, server, client)
	healthy := client.fetchDownload("/api/v1/backups", map[string]string{"passphrase": backupPassphrase})
	replaceBlobKeepingSize(t, server, key)

	prepared := client.do(http.MethodPost, "/api/v1/backups", map[string]string{"passphrase": backupPassphrase})
	var info preparedFile
	prepared.json(t, &info)
	if info.Complete || len(info.MissingFiles) != 1 || info.MissingFiles[0].Problem != backup.ProblemDamaged || info.MissingFiles[0].Filename != "receipt.pdf" {
		t.Fatalf("a damaged receipt was backed up as complete: %s", prepared.Body)
	}
	archive := client.do(http.MethodGet, info.DownloadURL, nil)
	identity, _ := age.NewScryptIdentity(backupPassphrase)
	if manifest, err := backup.Read(context.Background(), bytes.NewReader(archive.Body), identity, ""); err != nil || manifest.Complete() || len(manifest.Damaged) != 1 {
		t.Fatalf("the archive does not say the receipt is damaged: %v %+v", err, manifest)
	}

	if result := client.do(http.MethodPost, "/api/v1/exports", map[string]string{"format": "archive"}); result.code() != "export_file_damaged" || !strings.Contains(string(result.Body), "receipt.pdf") {
		t.Fatalf("a damaged receipt was exported: %d %s", result.Status, result.Body)
	}

	// An old archive whose manifest is self-consistent but whose receipt bytes
	// differ from the upload record is caught when staged.
	tampered := rewriteArchive(t, healthy.Body, func(name string, content []byte) ([]byte, bool) {
		if strings.HasSuffix(name, key) {
			changed := append([]byte{}, content...)
			changed[len(changed)/2] ^= 0xff
			return changed, true
		}
		return content, true
	})
	fresh := newTestServer(t)
	staging, staged, err := backup.Stage(context.Background(), fresh.application.Configuration.DataDirectory, bytes.NewReader(tampered), identity)
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(staging)
	if staged.Complete() || len(staged.Damaged) != 1 || staged.MissingFiles[0].Problem != backup.ProblemDamaged {
		t.Fatalf("staging trusted altered receipt bytes: %+v", staged)
	}
	goodStaging, good, err := backup.Stage(context.Background(), fresh.application.Configuration.DataDirectory, bytes.NewReader(healthy.Body), identity)
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(goodStaging)
	if !good.Complete() {
		t.Fatalf("an unchanged archive was not complete: %+v", good)
	}

	client.createItem(map[string]any{"name": "After the backup"})
	if result := client.restoreAccepting(tampered, false); result.code() != "backup_incomplete" {
		t.Fatalf("a restore of a damaged receipt did not ask first: %d %s", result.Status, result.Body)
	}
	if itemCount(server, "After the backup") != 1 {
		t.Fatal("a refused restore changed live data")
	}
}
