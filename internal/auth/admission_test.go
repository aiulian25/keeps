package auth

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"path/filepath"
	"testing"
	"time"

	"github.com/aiulian25/keeps/internal/config"
	"github.com/aiulian25/keeps/internal/secrets"
	"github.com/aiulian25/keeps/internal/store"
	"github.com/go-webauthn/webauthn/webauthn"
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

// fillChallenges stores count unexpired anonymous sign-in challenges.
func fillChallenges(t *testing.T, service *Service, count int) {
	t.Helper()
	future := store.FormatTime(time.Now().Add(time.Minute))
	for index := range count {
		if _, err := service.store.Database.Exec(`INSERT INTO webauthn_challenges (id, user_id, ceremony, session_json, expires_at) VALUES (?, NULL, ?, '{}', ?)`,
			fmt.Sprintf("filler-%d", index), ceremonyLogin, future); err != nil {
			t.Fatal(err)
		}
	}
}

func anonymousChallengeCount(service *Service) int {
	var count int
	service.store.Database.QueryRow(`SELECT count(*) FROM webauthn_challenges WHERE user_id IS NULL`).Scan(&count)
	return count
}

func TestTheLastChallengePlaceIsGivenOnce(t *testing.T) {
	service, principal := claimedForRace(t)
	ctx := context.Background()
	fillChallenges(t, service, config.PasskeyLoginChallengesMaximum-1)

	// Both starts pass the early check before either stores its challenge.
	for range 2 {
		if err := service.admitLoginChallenge(ctx); err != nil {
			t.Fatalf("a start was refused with a place left: %v", err)
		}
	}
	var limited *ErrRateLimited
	if _, err := service.storeChallenge(ctx, 0, ceremonyLogin, &webauthn.SessionData{}); err != nil {
		t.Fatalf("the first start did not get the last place: %v", err)
	}
	if _, err := service.storeChallenge(ctx, 0, ceremonyLogin, &webauthn.SessionData{}); !errors.As(err, &limited) {
		t.Fatalf("the second start was not refused: %v", err)
	}
	if count := anonymousChallengeCount(service); count != config.PasskeyLoginChallengesMaximum {
		t.Fatalf("%d anonymous challenges are kept, more than the cap", count)
	}

	if _, err := service.storeChallenge(ctx, principal.UserID, ceremonyRegistration, &webauthn.SessionData{}); err != nil {
		t.Fatalf("a signed-in registration was refused by the anonymous cap: %v", err)
	}
	service.store.Database.Exec(`UPDATE webauthn_challenges SET expires_at = ? WHERE id = 'filler-0'`, store.FormatTime(time.Now().Add(-time.Minute)))
	if _, err := service.BeginPasskeyLogin(ctx, "198.51.100.20"); err != nil {
		t.Fatalf("an expired challenge did not free its place: %v", err)
	}
}
