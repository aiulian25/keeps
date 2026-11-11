package auth

import (
	"context"
	"database/sql"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/aiulian25/keeps/internal/config"
	"github.com/pquerna/otp"
	"github.com/pquerna/otp/totp"
)

const (
	raceUsername  = "alex"
	racePassword  = "the first long password"
	raceClient    = "198.51.100.7"
	raceUserAgent = "race test"
)

func claimedForRace(t *testing.T) (*Service, Principal) {
	t.Helper()
	service := newAdmissionService(t)
	ctx := context.Background()
	code, err := service.EnsureSetupCode(ctx, true)
	if err != nil {
		t.Fatal(err)
	}
	token, _, err := service.Claim(ctx, ClaimRequest{SetupCode: code, HouseholdName: "Home", Username: raceUsername, Password: racePassword, ClientAddress: raceClient})
	if err != nil {
		t.Fatal(err)
	}
	session, err := service.lookupSession(ctx, token)
	if err != nil {
		t.Fatal(err)
	}
	return service, Principal{UserID: session.UserID, HouseholdID: session.HouseholdID, SessionID: session.ID, Username: session.Username}
}

// signInAround pauses a password sign-in once it has read the account and
// asks to check the password, runs change, then lets the sign-in finish.
func signInAround(t *testing.T, service *Service, change func()) (SignInResult, error) {
	t.Helper()
	// Unbuffered, so the test itself hands out and takes back the hash slot.
	service.hashSlots = make(chan struct{})
	type outcome struct {
		result SignInResult
		err    error
	}
	finished := make(chan outcome, 1)
	go func() {
		result, err := service.SignInWithPassword(context.Background(), raceUsername, racePassword, raceUserAgent, raceClient)
		finished <- outcome{result, err}
	}()
	<-service.hashSlots
	change()
	service.hashSlots <- struct{}{}
	done := <-finished
	return done.result, done.err
}

func sessionCount(t *testing.T, service *Service, userID int64) int {
	t.Helper()
	var count int
	service.store.Database.QueryRow(`SELECT count(*) FROM sessions WHERE user_id = ?`, userID).Scan(&count)
	return count
}

func protectedStatus(service *Service, token string) int {
	protected := service.Protect(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		writer.WriteHeader(http.StatusNoContent)
	}))
	request := httptest.NewRequest(http.MethodGet, "http://localhost/api/v1/items", nil)
	request.AddCookie(&http.Cookie{Name: service.cookieName(), Value: token})
	recorder := httptest.NewRecorder()
	protected.ServeHTTP(recorder, request)
	return recorder.Code
}

func TestSignInStartedBeforeAPasswordChangeGetsNoSession(t *testing.T) {
	service, principal := claimedForRace(t)
	ctx := context.Background()
	replacement, err := service.HashPassword(ctx, "the second long password")
	if err != nil {
		t.Fatal(err)
	}
	// The same writes ChangePassword commits: the new hash and signing out
	// every session.
	rotate := func() {
		err := service.store.Transaction(ctx, func(transaction *sql.Tx) error {
			if _, err := transaction.ExecContext(ctx, `UPDATE users SET password_hash = ? WHERE id = ?`, replacement, principal.UserID); err != nil {
				return err
			}
			return revokeSessions(ctx, transaction, principal.UserID, 0)
		})
		if err != nil {
			t.Error(err)
		}
	}
	result, err := signInAround(t, service, rotate)
	if !errors.Is(err, ErrInvalidCredentials) || result.Token != "" {
		t.Fatalf("a sign-in checked against the replaced password was let in: %v", err)
	}
	if count := sessionCount(t, service, principal.UserID); count != 0 {
		t.Fatalf("%d sessions survived the password change", count)
	}
}

func TestSignInStartedBeforeAuthenticatorEnrolmentNeedsTheCode(t *testing.T) {
	service, principal := claimedForRace(t)
	ctx := context.Background()
	setup, err := service.BeginTOTP(ctx, principal)
	if err != nil {
		t.Fatal(err)
	}
	code, err := totp.GenerateCodeCustom(setup.Secret, time.Now(), totp.ValidateOpts{Period: config.TOTPPeriodSeconds, Digits: totpDigits, Algorithm: otp.AlgorithmSHA1})
	if err != nil {
		t.Fatal(err)
	}
	enrol := func() {
		if _, err := service.ConfirmTOTP(ctx, principal, code); err != nil {
			t.Error(err)
		}
	}
	result, err := signInAround(t, service, enrol)
	if err != nil {
		t.Fatal(err)
	}
	if !result.AwaitingSecondFactor {
		t.Fatal("a sign-in finished after enrolment got a full session without the code")
	}
	if status := protectedStatus(service, result.Token); status != http.StatusUnauthorized {
		t.Fatalf("the pending session reached a protected page: %d", status)
	}
}

func TestOrdinaryPasswordSignInIsUnchanged(t *testing.T) {
	service, _ := claimedForRace(t)
	result, err := signInAround(t, service, func() {})
	if err != nil || result.AwaitingSecondFactor {
		t.Fatalf("an ordinary sign-in failed: %v %+v", err, result)
	}
	if status := protectedStatus(service, result.Token); status != http.StatusNoContent {
		t.Fatalf("the new session can't reach a protected page: %d", status)
	}
}
