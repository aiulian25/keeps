package server

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"context"
	"encoding/csv"
	"fmt"
	"mime/multipart"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"

	"filippo.io/age"

	"github.com/aiulian25/keeps/internal/backup"
)

const backupPassphrase = "a long backup passphrase"

// fetchDownload prepares a backup or export like the web app does, then
// downloads it. A failed preparation is returned as is.
func (c *testClient) fetchDownload(path string, body map[string]string) response {
	c.server.t.Helper()
	prepared := c.do(http.MethodPost, path, body)
	if prepared.Status != http.StatusOK {
		return prepared
	}
	var info preparedFile
	prepared.json(c.server.t, &info)
	return c.do(http.MethodGet, info.DownloadURL, nil)
}

func (c *testClient) restore(archive []byte, passphrase string) response {
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	writer.WriteField("passphrase", passphrase)
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
	var buffer bytes.Buffer
	buffer.ReadFrom(result.Body)
	return response{Status: result.StatusCode, Body: buffer.Bytes()}
}

func TestBackupDuringWritesAndRestoreWithEverything(t *testing.T) {
	server := newTestServer(t)
	client := server.claimed()

	begin := client.do(http.MethodPost, "/api/v1/account/totp", nil)
	var setup struct {
		Secret string `json:"secret"`
	}
	begin.json(t, &setup)
	client.do(http.MethodPost, "/api/v1/account/totp/confirm", map[string]string{"code": currentCode(t, setup.Secret, 0)})
	client.refreshCSRF()

	invoice := client.upload("invoice", "receipt.pdf", "application/pdf", []byte("%PDF-1.4\nreceipt\n%%EOF"))
	var invoiceBody struct {
		ID int64 `json:"id"`
	}
	invoice.json(t, &invoiceBody)
	saved := client.createItem(map[string]any{"name": "Coffee machine", "pricePaid": map[string]string{"amount": "599", "currency": "GBP"}, "attachmentIds": []int64{invoiceBody.ID}})

	var writers sync.WaitGroup
	stop := make(chan struct{})
	writers.Add(1)
	go func() {
		defer writers.Done()
		for index := 0; ; index++ {
			select {
			case <-stop:
				return
			default:
				client.do(http.MethodPost, "/api/v1/items", map[string]any{"name": fmt.Sprintf("Concurrent %d", index)})
			}
		}
	}()
	time.Sleep(50 * time.Millisecond)
	archive := client.fetchDownload("/api/v1/backups", map[string]string{"passphrase": backupPassphrase})
	close(stop)
	writers.Wait()
	if archive.Status != http.StatusOK || !strings.Contains(archive.Header.Get("Content-Disposition"), ".keeps.age") {
		t.Fatalf("backup: %d %s", archive.Status, archive.Header)
	}
	if bytes.Contains(archive.Body, []byte("Coffee machine")) || bytes.Contains(archive.Body, []byte("SQLite format")) {
		t.Fatal("backup contains plaintext")
	}
	identity, _ := age.NewScryptIdentity(backupPassphrase)
	manifest, err := backup.Read(context.Background(), bytes.NewReader(archive.Body), identity, "")
	if err != nil {
		t.Fatalf("backup taken during writes does not verify: %v", err)
	}
	if len(manifest.Files) != 3 {
		t.Fatalf("expected db, key and invoice in manifest, got %d", len(manifest.Files))
	}

	if result := client.fetchDownload("/api/v1/backups", map[string]string{"passphrase": "short"}); result.code() != "passphrase_too_short" {
		t.Fatalf("weak passphrase accepted: %s", result.Body)
	}

	// Blank host: a brand-new data directory restored with the CLI path.
	fresh := newTestServer(t)
	fresh.application.Store.Database.Close()
	staging, _, err := backup.Stage(context.Background(), fresh.application.Configuration.DataDirectory, bytes.NewReader(archive.Body), identity)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := backup.Swap(fresh.application.Configuration.DataDirectory, staging); err != nil {
		t.Fatal(err)
	}
	if err := fresh.application.Store.Reopen(context.Background(), ""); err != nil {
		t.Fatal(err)
	}
	fresh.application.Keys.Reload(fresh.application.keyPath())
	fresh.application.FinishRestore(context.Background())

	visitor := fresh.newClient()
	signIn := visitor.do(http.MethodPost, "/api/v1/sessions", map[string]string{"username": testUsername, "password": testPassword})
	if !contains(signIn.Body, `"awaitingSecondFactor":true`) {
		t.Fatalf("restored instance lost TOTP: %s", signIn.Body)
	}
	if result := visitor.do(http.MethodPost, "/api/v1/sessions/second-factor", map[string]string{"code": currentCode(t, setup.Secret, 30*time.Second)}); result.Status != http.StatusOK {
		t.Fatalf("restored TOTP seed unusable: %s", result.Body)
	}
	visitor.refreshCSRF()
	var restored itemResponse
	visitor.do(http.MethodGet, fmt.Sprintf("/api/v1/items/%d", saved.ID), nil).json(t, &restored)
	if len(restored.Documents) != 1 || (*restored.PricePaid)["amount"] != "599.00" {
		t.Fatalf("restored item incomplete: %+v", restored)
	}
	download := visitor.do(http.MethodGet, fmt.Sprintf("/api/v1/attachments/%d", invoiceBody.ID), nil)
	if download.Status != http.StatusOK || !bytes.Contains(download.Body, []byte("receipt")) {
		t.Fatalf("restored invoice not downloadable: %d", download.Status)
	}
}

func TestRestoreThroughBrowserRevokesSessionsAndRejectsBadArchives(t *testing.T) {
	server := newTestServer(t)
	client := server.claimed()
	client.createItem(map[string]any{"name": "Before backup"})
	archive := client.fetchDownload("/api/v1/backups", map[string]string{"passphrase": backupPassphrase})
	client.createItem(map[string]any{"name": "After backup"})

	if result := client.restore(archive.Body, "wrong passphrase"); result.code() != "backup_wrong_secret" {
		t.Fatalf("wrong passphrase: %s", result.Body)
	}
	tampered := append([]byte{}, archive.Body...)
	tampered[len(tampered)-40] ^= 0xff
	if result := client.restore(tampered, backupPassphrase); result.Status != http.StatusBadRequest {
		t.Fatalf("tampered archive: %d %s", result.Status, result.Body)
	}
	for name, entries := range map[string][]tar.Header{
		"traversal": {{Name: "../../etc/passwd", Typeflag: tar.TypeReg, Size: 1}},
		"absolute":  {{Name: "/etc/passwd", Typeflag: tar.TypeReg, Size: 1}},
		"symlink":   {{Name: "keeps.db", Typeflag: tar.TypeSymlink, Linkname: "/etc/passwd"}},
		"unknown":   {{Name: "evil.sh", Typeflag: tar.TypeReg, Size: 1}},
	} {
		if result := client.restore(hostileArchive(t, entries), backupPassphrase); result.code() != "backup_unsafe" {
			t.Errorf("%s archive: %s", name, result.Body)
		}
	}
	var titles []string
	rows, _ := server.application.Store.Database.Query(`SELECT name FROM items ORDER BY id`)
	for rows.Next() {
		var title string
		rows.Scan(&title)
		titles = append(titles, title)
	}
	rows.Close()
	if len(titles) != 2 {
		t.Fatalf("a rejected restore changed live data: %v", titles)
	}

	other := server.newClient()
	other.do(http.MethodPost, "/api/v1/sessions", map[string]string{"username": testUsername, "password": testPassword})
	other.refreshCSRF()

	if result := client.restore(archive.Body, backupPassphrase); result.Status != http.StatusOK {
		t.Fatalf("restore: %d %s", result.Status, result.Body)
	}
	if result := other.do(http.MethodGet, "/api/v1/account", nil); result.Status != http.StatusUnauthorized {
		t.Fatal("old session survived restore")
	}
	var count int
	server.application.Store.Database.QueryRow(`SELECT count(*) FROM items WHERE name = 'After backup'`).Scan(&count)
	if count != 0 {
		t.Fatal("restore did not replace data")
	}
}

func hostileArchive(t *testing.T, entries []tar.Header) []byte {
	t.Helper()
	var plain bytes.Buffer
	archive := tar.NewWriter(&plain)
	for _, header := range entries {
		header := header
		archive.WriteHeader(&header)
		if header.Size > 0 {
			archive.Write(bytes.Repeat([]byte("x"), int(header.Size)))
		}
	}
	archive.Close()
	recipient, _ := age.NewScryptRecipient(backupPassphrase)
	recipient.SetWorkFactor(10)
	var encrypted bytes.Buffer
	writer, _ := age.Encrypt(&encrypted, recipient)
	writer.Write(plain.Bytes())
	writer.Close()
	return encrypted.Bytes()
}

func TestExportsEscapeFormulasAndIncludeFiles(t *testing.T) {
	server := newTestServer(t)
	client := server.claimed()
	photo := client.upload("photo", "drill.png", "image/png", pngBytes(10, 10))
	var photoBody struct {
		ID int64 `json:"id"`
	}
	photo.json(t, &photoBody)
	client.createItem(map[string]any{"name": "=HYPERLINK(\"http://evil\")", "notes": "@SUM(1)", "attachmentIds": []int64{photoBody.ID}})

	result := client.fetchDownload("/api/v1/exports", map[string]string{"format": "csv"})
	rows, err := csv.NewReader(bytes.NewReader(result.Body)).ReadAll()
	if err != nil || len(rows) != 2 {
		t.Fatalf("csv: %v %d", err, len(rows))
	}
	if !strings.HasPrefix(rows[1][1], "'=") || !strings.HasPrefix(rows[1][20], "'@") {
		t.Fatalf("formula not neutralised: %q %q", rows[1][1], rows[1][20])
	}

	archive := client.fetchDownload("/api/v1/exports", map[string]string{"format": "archive"})
	reader, err := zip.NewReader(bytes.NewReader(archive.Body), int64(len(archive.Body)))
	if err != nil {
		t.Fatal(err)
	}
	names := map[string]bool{}
	for _, file := range reader.File {
		names[file.Name] = true
	}
	for _, expected := range []string{"items.json", "README.txt", "checksums.sha256", fmt.Sprintf("files/%d-drill.png", photoBody.ID)} {
		if !names[expected] {
			t.Errorf("archive missing %s (%v)", expected, names)
		}
	}

	server.application.Store.Database.Exec(`UPDATE sessions SET reauthenticated_at = '2000-01-01T00:00:00Z'`)
	if stale := client.fetchDownload("/api/v1/exports", map[string]string{"format": "csv"}); stale.code() != "reauthentication_required" {
		t.Fatalf("export without recent confirmation: %s", stale.Body)
	}
}

func TestAdminRecoveryWithoutShell(t *testing.T) {
	server := newTestServer(t)
	client := server.claimed()
	code, err := server.application.Auth.IssueRecoveryCapability(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if result := client.do(http.MethodGet, "/api/v1/account", nil); result.Status != http.StatusUnauthorized {
		t.Fatal("sessions survived recovery issue")
	}
	visitor := server.newClient()
	if result := visitor.do(http.MethodPost, "/api/v1/recover", map[string]string{"code": code, "newPassword": "brand new passphrase"}); result.Status != http.StatusOK {
		t.Fatalf("recover: %s", result.Body)
	}
	if result := server.newClient().do(http.MethodPost, "/api/v1/recover", map[string]string{"code": code, "newPassword": "another passphrase"}); result.code() != "recovery_invalid" {
		t.Fatalf("recovery code reused: %s", result.Body)
	}
	if result := server.newClient().do(http.MethodPost, "/api/v1/sessions", map[string]string{"username": testUsername, "password": "brand new passphrase"}); result.Status != http.StatusOK {
		t.Fatalf("new password rejected: %s", result.Body)
	}

	release, err := AcquireInstanceLock(server.application.Configuration)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := AcquireInstanceLock(server.application.Configuration); err != ErrInstanceRunning {
		t.Fatalf("second lock holder allowed: %v", err)
	}
	release()
}
