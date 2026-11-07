package server

import (
	"bytes"
	"io"
	"mime/multipart"
	"net"
	"net/http"
	"path/filepath"
	"testing"
	"time"

	"github.com/aiulian25/keeps/internal/config"
)

// slowBody hands out its content in small pieces with a pause before each,
// like a backup uploading over a slow connection.
type slowBody struct {
	content []byte
	pause   time.Duration
	piece   int
}

func (b *slowBody) Read(buffer []byte) (int, error) {
	if len(b.content) == 0 {
		return 0, io.EOF
	}
	time.Sleep(b.pause)
	size := min(b.piece, len(buffer), len(b.content))
	copy(buffer, b.content[:size])
	b.content = b.content[size:]
	return size, nil
}

// realServer serves the application through net/http with the same kind of
// deadlines as production, scaled down so the test runs in seconds.
func realServer(t *testing.T, server *testServer, readTimeout time.Duration) string {
	t.Helper()
	handler, err := server.application.Handler()
	if err != nil {
		t.Fatal(err)
	}
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	httpServer := &http.Server{Handler: handler, ReadHeaderTimeout: readTimeout, ReadTimeout: readTimeout, WriteTimeout: readTimeout}
	go httpServer.Serve(listener)
	t.Cleanup(func() { httpServer.Close() })
	return "http://" + listener.Addr().String()
}

func (c *testClient) slowRestore(t *testing.T, address string, archive []byte, pause time.Duration) response {
	t.Helper()
	var form bytes.Buffer
	writer := multipart.NewWriter(&form)
	writer.WriteField("passphrase", backupPassphrase)
	part, _ := writer.CreateFormFile("file", "backup.keeps.age")
	part.Write(archive)
	writer.Close()
	body := &slowBody{content: form.Bytes(), pause: pause, piece: max(1, form.Len()/20)}
	request, _ := http.NewRequest(http.MethodPost, address+"/api/v1/restore", body)
	request.ContentLength = int64(form.Len())
	request.Header.Set("Content-Type", writer.FormDataContentType())
	request.Header.Set("Origin", c.origin)
	request.Header.Set("X-CSRF-Token", c.csrf)
	result, err := c.client.Do(request)
	if err != nil {
		return response{Status: 0, Body: []byte(err.Error())}
	}
	defer result.Body.Close()
	content, _ := io.ReadAll(result.Body)
	return response{Status: result.StatusCode, Body: content}
}

func TestSlowRestoreUploadOutlastsTheOrdinaryReadDeadline(t *testing.T) {
	server := newTestServer(t)
	client := server.claimed()
	client.createItem(map[string]any{"name": "In the backup"})
	archive := client.fetchDownload("/api/v1/backups", map[string]string{"passphrase": backupPassphrase})
	client.createItem(map[string]any{"name": "After the backup"})

	previousIdle := restoreIdleTimeout
	restoreIdleTimeout = 2 * time.Second
	t.Cleanup(func() { restoreIdleTimeout = previousIdle })
	address := realServer(t, server, 300*time.Millisecond)

	// About 2 seconds in total, far beyond the 300 ms ordinary deadline, but
	// never idle for long.
	result := client.slowRestore(t, address, archive.Body, 100*time.Millisecond)
	if result.Status != http.StatusOK {
		t.Fatalf("a slow but steady restore upload was cut off: %d %s", result.Status, result.Body)
	}
	if itemCount(server, "After the backup") != 0 {
		t.Fatal("the restore did not happen")
	}
}

func TestStalledRestoreUploadStopsCleanly(t *testing.T) {
	server := newTestServer(t)
	client := server.claimed()
	archive := client.fetchDownload("/api/v1/backups", map[string]string{"passphrase": backupPassphrase})
	client.createItem(map[string]any{"name": "Still here"})

	previousIdle := restoreIdleTimeout
	restoreIdleTimeout = 300 * time.Millisecond
	t.Cleanup(func() { restoreIdleTimeout = previousIdle })
	address := realServer(t, server, 5*time.Second)

	result := client.slowRestore(t, address, archive.Body, time.Second)
	if result.Status == http.StatusOK {
		t.Fatal("a stalled upload was accepted")
	}
	if itemCount(server, "Still here") != 1 {
		t.Fatal("a stalled restore changed live data")
	}
	if uploads := uploadLeftovers(server); len(uploads) != 0 {
		t.Fatalf("a stalled upload left files behind: %v", uploads)
	}
}

func uploadLeftovers(server *testServer) []string {
	found, _ := filepath.Glob(filepath.Join(server.application.Configuration.Directory(config.StagingDirectoryName), "*"))
	return found
}
