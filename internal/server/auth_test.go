package server

import (
	"context"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/pquerna/otp"
	"github.com/pquerna/otp/totp"
)

func TestBootstrapRaceProducesExactlyOneAdministrator(t *testing.T) {
	server := newTestServer(t)
	code := server.setupCode()

	const contenders = 20
	var wait sync.WaitGroup
	statuses := make(chan int, contenders)
	for index := range contenders {
		wait.Add(1)
		go func() {
			defer wait.Done()
			client := server.newClient()
			result := client.do(http.MethodPost, "/api/v1/bootstrap", map[string]string{
				"setupCode": code, "householdName": "Home", "username": "admin" + string(rune('a'+index)), "password": testPassword,
			})
			statuses <- result.Status
		}()
	}
	wait.Wait()
	close(statuses)

	created := 0
	for status := range statuses {
		if status == http.StatusCreated {
			created++
		}
	}
	var administrators int
	server.application.Store.Database.QueryRow(`SELECT count(*) FROM users`).Scan(&administrators)
	if created != 1 || administrators != 1 {
		t.Fatalf("created %d responses, %d users", created, administrators)
	}

	again := server.newClient().do(http.MethodPost, "/api/v1/bootstrap", map[string]string{
		"setupCode": code, "householdName": "Home", "username": "late", "password": testPassword,
	})
	if again.Status != http.StatusGone {
		t.Fatalf("bootstrap reopened: %d %s", again.Status, again.Body)
	}
	if _, err := server.application.Auth.EnsureSetupCode(context.Background(), true); err == nil {
		t.Fatal("bootstrap-code issued a code after claim")
	}
}

func TestBootstrapRejectsWrongAndExpiredCodes(t *testing.T) {
	server := newTestServer(t)
	server.setupCode()
	client := server.newClient()
	body := map[string]string{"setupCode": "AAAA-BBBB-CCCC-DDDD", "householdName": "Home", "username": "a", "password": testPassword}
	if result := client.do(http.MethodPost, "/api/v1/bootstrap", body); result.code() != "setup_code_invalid" {
		t.Fatalf("wrong code: %s", result.Body)
	}
	code := server.setupCode()
	server.application.Store.Database.Exec(`UPDATE bootstrap_state SET code_expires_at = '2000-01-01T00:00:00Z'`)
	body["setupCode"] = code
	if result := client.do(http.MethodPost, "/api/v1/bootstrap", body); result.code() != "setup_code_expired" {
		t.Fatalf("expired code: %s", result.Body)
	}
}

func TestOriginAndCSRFProtection(t *testing.T) {
	server := newTestServer(t)
	client := server.claimed()

	if result := client.do(http.MethodPatch, "/api/v1/account", map[string]string{"theme": "light"}); result.Status != http.StatusOK {
		t.Fatalf("valid request failed: %d %s", result.Status, result.Body)
	}

	csrf := client.csrf
	client.csrf = ""
	if result := client.do(http.MethodPatch, "/api/v1/account", map[string]string{"theme": "dark"}); result.code() != "bad_csrf" {
		t.Fatalf("missing csrf accepted: %s", result.Body)
	}
	client.csrf = csrf
	client.origin = "https://evil.example"
	if result := client.do(http.MethodPatch, "/api/v1/account", map[string]string{"theme": "dark"}); result.code() != "bad_origin" {
		t.Fatalf("cross origin accepted: %s", result.Body)
	}
	client.origin = "null"
	if result := client.do(http.MethodPatch, "/api/v1/account", map[string]string{"theme": "dark"}); result.code() != "bad_origin" {
		t.Fatalf("opaque origin without same-origin fetch metadata accepted: %s", result.Body)
	}
	client.origin = testOrigin
	if result := client.do(http.MethodPatch, "/api/v1/account", map[string]any{"theme": "dark", "isAdmin": true}); result.Status != http.StatusBadRequest {
		t.Fatalf("unknown field accepted: %d", result.Status)
	}
	if result := server.newClient().do(http.MethodGet, "/api/v1/account", nil); result.Status != http.StatusUnauthorized {
		t.Fatalf("anonymous access: %d", result.Status)
	}
}

func TestSignInFailureRateLimitAndSessionRotation(t *testing.T) {
	server := newTestServer(t)
	first := server.claimed()

	attacker := server.newClient()
	var last response
	for range 7 {
		last = attacker.do(http.MethodPost, "/api/v1/sessions", map[string]string{"username": testUsername, "password": "wrong password!"})
	}
	if last.code() != "rate_limited" {
		t.Fatalf("no rate limit after repeated failures: %s", last.Body)
	}
	server.application.Store.Database.Exec(`DELETE FROM login_failures`)

	unknown := attacker.do(http.MethodPost, "/api/v1/sessions", map[string]string{"username": "nobody", "password": "wrong password!"})
	known := attacker.do(http.MethodPost, "/api/v1/sessions", map[string]string{"username": testUsername, "password": "wrong password!"})
	if unknown.code() != known.code() || unknown.code() != "invalid_credentials" {
		t.Fatalf("responses reveal valid usernames: %s vs %s", unknown.Body, known.Body)
	}
	server.application.Store.Database.Exec(`DELETE FROM login_failures`)

	second := server.newClient()
	if result := second.do(http.MethodPost, "/api/v1/sessions", map[string]string{"username": testUsername, "password": testPassword}); result.Status != http.StatusOK {
		t.Fatalf("sign in: %s", result.Body)
	}
	second.refreshCSRF()

	change := first.do(http.MethodPost, "/api/v1/account/password", map[string]string{"password": testPassword, "newPassword": "a whole new passphrase"})
	if change.Status != http.StatusNoContent {
		t.Fatalf("password change: %d %s", change.Status, change.Body)
	}
	if result := second.do(http.MethodGet, "/api/v1/account", nil); result.Status != http.StatusUnauthorized {
		t.Fatalf("other session survived password change: %d", result.Status)
	}
	first.refreshCSRF()
	if result := first.do(http.MethodGet, "/api/v1/account", nil); result.Status != http.StatusOK {
		t.Fatalf("current session was not rotated into a valid one: %d", result.Status)
	}
}

func currentCode(t *testing.T, secret string, offset time.Duration) string {
	t.Helper()
	code, err := totp.GenerateCodeCustom(secret, time.Now().Add(offset), totp.ValidateOpts{Period: 30, Digits: otp.DigitsSix, Algorithm: otp.AlgorithmSHA1})
	if err != nil {
		t.Fatal(err)
	}
	return code
}

func TestTOTPEnrolmentPolicyReplayAndRecoveryCodes(t *testing.T) {
	server := newTestServer(t)
	client := server.claimed()

	if result := client.do(http.MethodPost, "/api/v1/account/totp", nil); result.Status != http.StatusOK {
		t.Fatalf("fresh session should count as recently confirmed: %d %s", result.Status, result.Body)
	}
	server.application.Store.Database.Exec(`UPDATE sessions SET reauthenticated_at = '2000-01-01T00:00:00Z'`)
	if result := client.do(http.MethodPost, "/api/v1/account/totp", nil); result.code() != "reauthentication_required" {
		t.Fatalf("stale session allowed security change: %s", result.Body)
	}
	if result := client.do(http.MethodPost, "/api/v1/account/reauthenticate", map[string]string{"password": testPassword}); result.Status != http.StatusNoContent {
		t.Fatalf("reauth: %s", result.Body)
	}
	begin := client.do(http.MethodPost, "/api/v1/account/totp", nil)
	var setup struct {
		Secret string `json:"secret"`
		QRCode string `json:"qrCode"`
	}
	begin.json(t, &setup)
	if setup.Secret == "" || !contains([]byte(setup.QRCode), "data:image/png;base64,") {
		t.Fatalf("setup incomplete: %s", begin.Body)
	}

	if result := client.do(http.MethodPost, "/api/v1/account/totp/confirm", map[string]string{"code": "000000"}); result.code() != "invalid_code" {
		t.Fatalf("wrong code enabled TOTP: %s", result.Body)
	}
	confirm := client.do(http.MethodPost, "/api/v1/account/totp/confirm", map[string]string{"code": currentCode(t, setup.Secret, 0)})
	var codes struct {
		RecoveryCodes []string `json:"recoveryCodes"`
	}
	confirm.json(t, &codes)
	if len(codes.RecoveryCodes) != 10 {
		t.Fatalf("recovery codes: %s", confirm.Body)
	}

	var stored []byte
	server.application.Store.Database.QueryRow(`SELECT encrypted_secret FROM totp_credentials`).Scan(&stored)
	if contains(stored, setup.Secret) {
		t.Fatal("TOTP secret stored in plaintext")
	}

	signIn := func() *testClient {
		visitor := server.newClient()
		result := visitor.do(http.MethodPost, "/api/v1/sessions", map[string]string{"username": testUsername, "password": testPassword})
		if !contains(result.Body, `"awaitingSecondFactor":true`) {
			t.Fatalf("second factor not required: %s", result.Body)
		}
		if blocked := visitor.do(http.MethodGet, "/api/v1/account", nil); blocked.Status != http.StatusUnauthorized {
			t.Fatalf("pending session had access: %d", blocked.Status)
		}
		return visitor
	}

	visitor := signIn()
	replayed := currentCode(t, setup.Secret, 0)
	if result := visitor.do(http.MethodPost, "/api/v1/sessions/second-factor", map[string]string{"code": replayed}); result.code() != "invalid_code" {
		t.Fatalf("code already used during enrolment was accepted again: %s", result.Body)
	}
	next := currentCode(t, setup.Secret, 30*time.Second)
	if result := visitor.do(http.MethodPost, "/api/v1/sessions/second-factor", map[string]string{"code": next}); result.Status != http.StatusOK {
		t.Fatalf("next-step code rejected: %s", result.Body)
	}
	if result := visitor.do(http.MethodGet, "/api/v1/account", nil); result.Status != http.StatusOK {
		t.Fatalf("completed session lacks access: %d", result.Status)
	}

	server.application.Store.Database.Exec(`DELETE FROM login_failures`)
	recovering := signIn()
	if result := recovering.do(http.MethodPost, "/api/v1/sessions/second-factor", map[string]string{"recoveryCode": codes.RecoveryCodes[0]}); result.Status != http.StatusOK {
		t.Fatalf("recovery code rejected: %s", result.Body)
	}
	reuse := signIn()
	if result := reuse.do(http.MethodPost, "/api/v1/sessions/second-factor", map[string]string{"recoveryCode": codes.RecoveryCodes[0]}); result.code() != "invalid_code" {
		t.Fatalf("recovery code reused: %s", result.Body)
	}
}

func TestSignOutAndSessionList(t *testing.T) {
	server := newTestServer(t)
	client := server.claimed()
	var sessions []map[string]any
	client.do(http.MethodGet, "/api/v1/sessions", nil).json(t, &sessions)
	if len(sessions) != 1 || sessions[0]["current"] != true {
		t.Fatalf("sessions: %v", sessions)
	}
	if result := client.do(http.MethodDelete, "/api/v1/session", nil); result.Status != http.StatusNoContent {
		t.Fatalf("sign out: %d", result.Status)
	}
	if result := client.do(http.MethodGet, "/api/v1/account", nil); result.Status != http.StatusUnauthorized {
		t.Fatalf("session alive after sign out: %d", result.Status)
	}
}

func TestHealthEndpointsExposeNoData(t *testing.T) {
	server := newTestServer(t)
	client := server.newClient()
	for _, path := range []string{"/healthz", "/readyz"} {
		result := client.do(http.MethodGet, path, nil)
		if result.Status != http.StatusOK || len(result.Body) > 10 {
			t.Fatalf("%s: %d %q", path, result.Status, result.Body)
		}
	}
	if result := client.do(http.MethodGet, "/api/v1/nope", nil); result.Status != http.StatusNotFound {
		t.Fatalf("unknown api path: %d", result.Status)
	}
	if result := client.do(http.MethodGet, "/items/5", nil); result.Status != http.StatusOK || !contains(result.Body, `data-theme="dark"`) {
		t.Fatalf("client route not served: %d", result.Status)
	}
}

func TestUsernameSpellingsShareOneFailureCounter(t *testing.T) {
	server := newTestServer(t)
	server.claimed()
	attacker := server.newClient()
	for _, spelling := range []string{testUsername, testUsername + " ", "\t" + testUsername, " " + strings.ToUpper(testUsername) + " "} {
		attacker.do(http.MethodPost, "/api/v1/sessions", map[string]string{"username": spelling, "password": "wrong password!"})
	}
	var subjects, failures int
	server.application.Store.Database.QueryRow(`SELECT count(*), coalesce(max(failures), 0) FROM login_failures WHERE subject LIKE 'user:%'`).Scan(&subjects, &failures)
	if subjects != 1 || failures != 4 {
		t.Fatalf("username spellings split the account counter: %d subjects, %d failures", subjects, failures)
	}
}
