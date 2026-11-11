package server

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"filippo.io/age"
	"github.com/aiulian25/keeps/internal/backup"
	"github.com/aiulian25/keeps/internal/store"
)

// probeWorker stands in for any background writer. Each generation writes
// rows tagged with its number until it is cancelled.
type probeWorker struct {
	application *Application
	generations atomic.Int64
	running     atomic.Int64
}

func (p *probeWorker) Routes(*http.ServeMux, func(http.Handler) http.Handler) {}

func (p *probeWorker) RunBackground(ctx context.Context) {
	generation := p.generations.Add(1)
	p.running.Add(1)
	defer p.running.Add(-1)
	for index := 0; ; index++ {
		select {
		case <-ctx.Done():
			return
		case <-time.After(2 * time.Millisecond):
		}
		p.application.Store.Database.Exec(`INSERT INTO audit_events (household_id, user_id, kind, detail, created_at) VALUES (0, 0, 'probe', ?, ?)`,
			fmt.Sprintf("generation-%d", generation), store.Now())
	}
}

func TestRestorePausesEveryWorkerAndRecoversRestoredImports(t *testing.T) {
	server := newTestServer(t)
	client := server.claimed()
	application := server.application
	client.createItem(map[string]any{"name": "In the backup"})
	var householdID, userID int64
	application.Store.Database.QueryRow(`SELECT household_id, id FROM users LIMIT 1`).Scan(&householdID, &userID)
	application.Store.Database.Exec(`INSERT INTO imports (id, household_id, user_id, source_url, state, attempts, expires_at, created_at, updated_at)
		VALUES ('interrupted', ?, ?, 'https://shop.example/x', 'running', ?, '2999-01-01T00:00:00Z', ?, ?)`, householdID, userID, 99, store.Now(), store.Now())
	archive := client.fetchDownload("/api/v1/backups", map[string]string{"passphrase": backupPassphrase})

	probe := &probeWorker{application: application}
	application.modules = append(application.modules, probe)
	ctx, cancel := context.WithCancel(context.Background())
	application.startBackground(ctx)
	t.Cleanup(func() {
		cancel()
		application.pauseBackground()
	})
	time.Sleep(30 * time.Millisecond)

	if result := client.restore(archive.Body, backupPassphrase); result.Status != http.StatusOK {
		t.Fatalf("restore: %d %s", result.Status, result.Body)
	}
	time.Sleep(30 * time.Millisecond)

	var oldGeneration int
	application.Store.Database.QueryRow(`SELECT count(*) FROM audit_events WHERE kind = 'probe' AND detail = 'generation-1'`).Scan(&oldGeneration)
	if oldGeneration != 0 {
		t.Fatalf("a worker from before the restore wrote %d rows into the restored data", oldGeneration)
	}
	var newGeneration int
	application.Store.Database.QueryRow(`SELECT count(*) FROM audit_events WHERE kind = 'probe' AND detail = 'generation-2'`).Scan(&newGeneration)
	if newGeneration == 0 {
		t.Fatal("the probe never wrote, so this test proves nothing")
	}
	if probe.generations.Load() != 2 || probe.running.Load() != 1 {
		t.Fatalf("workers were not restarted exactly once: %d generations, %d running", probe.generations.Load(), probe.running.Load())
	}
	var state, errorCode string
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		application.Store.Database.QueryRow(`SELECT state, error_code FROM imports WHERE id = 'interrupted'`).Scan(&state, &errorCode)
		if state != "running" {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if state != "failed" || errorCode == "" {
		t.Fatalf("a restored running import was left stuck: %s %q", state, errorCode)
	}
	var sessions int
	application.Store.Database.QueryRow(`SELECT count(*) FROM sessions`).Scan(&sessions)
	if sessions != 0 {
		t.Fatal("sessions survived the restore")
	}
}

// A restore whose instance key cannot be loaded puts the previous data back,
// checks it opens, removes the rollback copy and keeps serving.
func TestFailedRestoreRollsBackAndKeepsServing(t *testing.T) {
	server := newTestServer(t)
	client := server.claimed()
	client.createItem(map[string]any{"name": "In the backup"})
	archive := client.fetchDownload("/api/v1/backups", map[string]string{"passphrase": backupPassphrase})
	broken := rewriteArchive(t, archive.Body, func(name string, content []byte) ([]byte, bool) {
		if name == "keys/instance.key" {
			return content[:len(content)-1], true
		}
		return content, true
	})
	client.createItem(map[string]any{"name": "After the backup"})

	if result := client.restore(broken, backupPassphrase); result.code() != "restore_rolled_back" {
		t.Fatalf("a restore with an unusable key: %d %s", result.Status, result.Body)
	}
	if itemCount(server, "After the backup") != 1 {
		t.Fatal("the previous data is not back")
	}
	if rollbacks, _ := filepath.Glob(filepath.Join(server.application.Configuration.DataDirectory, "rollback-*")); len(rollbacks) != 0 {
		t.Fatalf("rollback copy kept after a confirmed rollback: %v", rollbacks)
	}
	if result := client.do(http.MethodGet, "/api/v1/items", nil); result.Status != http.StatusOK {
		t.Fatalf("Keeps did not resume after a confirmed rollback: %d", result.Status)
	}
}

func TestUnconfirmedRollbackKeepsKeepsUnavailable(t *testing.T) {
	server := newTestServer(t)
	client := server.claimed()
	if err := server.application.needRecovery(errors.New("injected")); !errors.Is(err, errRecoveryNeeded) {
		t.Fatal(err)
	}
	if result := client.do(http.MethodGet, "/api/v1/items", nil); result.code() != "recovery_needed" {
		t.Fatalf("requests were served after an unconfirmed rollback: %d %s", result.Status, result.Body)
	}
	server.application.resumeBackground()
}

func stageArchive(t *testing.T, application *Application, archive []byte) string {
	t.Helper()
	identity, err := age.NewScryptIdentity(backupPassphrase)
	if err != nil {
		t.Fatal(err)
	}
	staging, _, err := backup.Stage(context.Background(), application.Configuration.DataDirectory, bytes.NewReader(archive), identity)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.RemoveAll(staging) })
	return staging
}

// cancelOnceSwapped cancels the restore's request the moment the live data
// has been moved, like a browser closed in the middle of a restore.
func cancelOnceSwapped(t *testing.T) context.Context {
	t.Helper()
	request, cancel := context.WithCancel(context.Background())
	swapData = func(data string, staging string) (string, error) {
		rollback, err := backup.Swap(data, staging)
		cancel()
		return rollback, err
	}
	t.Cleanup(func() {
		swapData = backup.Swap
		cancel()
	})
	return request
}

func TestARestoreOutlivesTheBrowserThatStartedIt(t *testing.T) {
	server := newTestServer(t)
	client := server.claimed()
	application := server.application
	client.createItem(map[string]any{"name": "In the backup"})
	archive := client.fetchDownload("/api/v1/backups", map[string]string{"passphrase": backupPassphrase}).Body
	client.createItem(map[string]any{"name": "After the backup"})
	serving := func(want int) {
		t.Helper()
		if result := client.do(http.MethodGet, "/api/v1/items", nil); result.Status != want {
			t.Fatalf("items answered %d %s, expected %d", result.Status, result.Body, want)
		}
	}

	gone, cancel := context.WithCancel(context.Background())
	cancel()
	if err := application.swapIn(gone, stageArchive(t, application, archive)); !errors.Is(err, context.Canceled) {
		t.Fatalf("a request gone before the swap: %v", err)
	}
	if itemCount(server, "After the backup") != 1 {
		t.Fatal("a request gone before the swap still changed the data")
	}
	serving(http.StatusOK)

	broken := rewriteArchive(t, archive, func(name string, content []byte) ([]byte, bool) {
		if name == "keys/instance.key" {
			return content[:len(content)-1], true
		}
		return content, true
	})
	err := application.swapIn(cancelOnceSwapped(t), stageArchive(t, application, broken))
	if err == nil || errors.Is(err, errRecoveryNeeded) {
		t.Fatalf("a failed restore whose browser left was not put back cleanly: %v", err)
	}
	if itemCount(server, "After the backup") != 1 {
		t.Fatal("the previous data is not back")
	}
	serving(http.StatusOK)

	if err := application.swapIn(cancelOnceSwapped(t), stageArchive(t, application, archive)); err != nil {
		t.Fatalf("a restore whose browser left did not finish: %v", err)
	}
	if itemCount(server, "In the backup") != 1 || itemCount(server, "After the backup") != 0 {
		t.Fatal("the backup's data is not in place")
	}
	var sessions int
	application.Store.Database.QueryRow(`SELECT count(*) FROM sessions`).Scan(&sessions)
	if sessions != 0 {
		t.Fatal("sessions survived the restore")
	}
	serving(http.StatusUnauthorized)
}
