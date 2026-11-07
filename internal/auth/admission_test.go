package auth

import (
	"context"
	"errors"
	"net/url"
	"path/filepath"
	"testing"
	"time"

	"github.com/aiulian25/keeps/internal/config"
	"github.com/aiulian25/keeps/internal/secrets"
	"github.com/aiulian25/keeps/internal/store"
)

func newAdmissionService(t *testing.T) *Service {
	t.Helper()
	directory := t.TempDir()
	database, err := store.Open(context.Background(), filepath.Join(directory, "keeps.db"), "")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { database.Close() })
	keys, err := secrets.LoadOrCreate(filepath.Join(directory, "instance.key"))
	if err != nil {
		t.Fatal(err)
	}
	origin, _ := url.Parse("http://localhost")
	service, err := NewService(database, config.Configuration{Origin: origin, DevelopmentLoopback: true, DataDirectory: directory}, keys)
	if err != nil {
		t.Fatal(err)
	}
	return service
}

func TestAlreadyClaimedSetupDoesNoHashing(t *testing.T) {
	service := newAdmissionService(t)
	service.store.Database.Exec(`UPDATE bootstrap_state SET claimed = 1 WHERE id = 1`)
	for range cap(service.hashSlots) {
		service.hashSlots <- struct{}{}
	}
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()
	_, _, err := service.Claim(ctx, ClaimRequest{SetupCode: "x", HouseholdName: "Home", Username: "alex", Password: "a long enough password", ClientAddress: "198.51.100.1"})
	if !errors.Is(err, ErrAlreadyClaimed) {
		t.Fatalf("a claimed instance waited for password hashing: %v", err)
	}
}

func TestAnonymousPasskeyStartsAreBounded(t *testing.T) {
	service := newAdmissionService(t)
	ctx := context.Background()
	var limited *ErrRateLimited
	started := 0
	for range 20 {
		if _, err := service.BeginPasskeyLogin(ctx, "198.51.100.2"); err != nil {
			if !errors.As(err, &limited) {
				t.Fatal(err)
			}
			break
		}
		started++
	}
	if started == 0 || started > config.LoginFailuresBeforeDelay+1 {
		t.Fatalf("one client started %d passkey sign-ins before being slowed", started)
	}

	future := store.FormatTime(time.Now().Add(time.Minute))
	for index := range config.PasskeyLoginChallengesMaximum {
		service.store.Database.Exec(`INSERT INTO webauthn_challenges (id, user_id, ceremony, session_json, expires_at) VALUES (?, NULL, ?, '{}', ?)`,
			"filler-"+string(rune('a'+index%26))+time.Now().Format("150405.000000000")+string(rune(index)), ceremonyLogin, future)
	}
	if _, err := service.BeginPasskeyLogin(ctx, "198.51.100.3"); !errors.As(err, &limited) {
		t.Fatalf("the cap of unanswered challenges was not enforced: %v", err)
	}
	service.store.Database.Exec(`UPDATE webauthn_challenges SET expires_at = ?`, store.FormatTime(time.Now().Add(-time.Minute)))
	if _, err := service.BeginPasskeyLogin(ctx, "198.51.100.3"); err != nil {
		t.Fatalf("expired challenges did not release capacity: %v", err)
	}
}
