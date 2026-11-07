package server

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/aiulian25/keeps/internal/config"
)

const testOrigin = "http://localhost"

// testServer runs the full handler over HTTP with a temporary data directory.
type testServer struct {
	t           *testing.T
	application *Application
	http        *httptest.Server
}

func newTestServer(t *testing.T) *testServer {
	t.Helper()
	configuration, err := config.Load(func(name string) (string, bool) {
		values := map[string]string{
			config.EnvironmentOrigin:        testOrigin,
			config.EnvironmentDevelopment:   "1",
			config.EnvironmentDataDirectory: t.TempDir(),
		}
		value, found := values[name]
		return value, found
	})
	if err != nil {
		t.Fatal(err)
	}
	application, err := Open(context.Background(), configuration)
	if err != nil {
		t.Fatal(err)
	}
	handler, err := application.Handler()
	if err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(handler)
	t.Cleanup(func() {
		server.Close()
		application.Close()
	})
	return &testServer{t: t, application: application, http: server}
}

type testClient struct {
	server *testServer
	client *http.Client
	csrf   string
	origin string
}

func (s *testServer) newClient() *testClient {
	jar, _ := cookiejar.New(nil)
	return &testClient{server: s, client: &http.Client{Jar: jar, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}, origin: testOrigin}
}

type response struct {
	Status int
	Body   []byte
	Header http.Header
}

func (r response) json(t *testing.T, destination any) {
	t.Helper()
	if err := json.Unmarshal(r.Body, destination); err != nil {
		t.Fatalf("decode %s: %v", r.Body, err)
	}
}

func (r response) code() string {
	var body struct {
		Code string `json:"code"`
	}
	json.Unmarshal(r.Body, &body)
	return body.Code
}

func (c *testClient) do(method string, path string, body any) response {
	c.server.t.Helper()
	var reader io.Reader
	contentType := ""
	switch typed := body.(type) {
	case nil:
	case io.Reader:
		reader = typed
	default:
		encoded, _ := json.Marshal(typed)
		reader = bytes.NewReader(encoded)
		contentType = "application/json"
	}
	request, _ := http.NewRequest(method, c.server.http.URL+path, reader)
	if contentType != "" {
		request.Header.Set("Content-Type", contentType)
	}
	if c.origin != "" {
		request.Header.Set("Origin", c.origin)
	}
	if c.csrf != "" {
		request.Header.Set("X-CSRF-Token", c.csrf)
	}
	result, err := c.client.Do(request)
	if err != nil {
		c.server.t.Fatal(err)
	}
	defer result.Body.Close()
	content, _ := io.ReadAll(result.Body)
	return response{Status: result.StatusCode, Body: content, Header: result.Header}
}

// refreshCSRF loads the session's CSRF token, as the web app does on start.
func (c *testClient) refreshCSRF() {
	c.server.t.Helper()
	result := c.do(http.MethodGet, "/api/v1/session", nil)
	var account struct {
		CSRFToken string `json:"csrfToken"`
	}
	result.json(c.server.t, &account)
	c.csrf = account.CSRFToken
}

func (s *testServer) setupCode() string {
	s.t.Helper()
	code, err := s.application.Auth.EnsureSetupCode(context.Background(), true)
	if err != nil {
		s.t.Fatal(err)
	}
	return code
}

const (
	testUsername = "alex"
	testPassword = "correct horse battery staple"
)

// claimed returns a signed-in administrator client.
func (s *testServer) claimed() *testClient {
	s.t.Helper()
	client := s.newClient()
	result := client.do(http.MethodPost, "/api/v1/bootstrap", map[string]string{
		"setupCode": s.setupCode(), "householdName": "The Miller Home", "username": testUsername, "password": testPassword,
	})
	if result.Status != http.StatusCreated {
		s.t.Fatalf("claim: %d %s", result.Status, result.Body)
	}
	client.refreshCSRF()
	return client
}

func (s *testServer) url(path string) *url.URL {
	parsed, _ := url.Parse(s.http.URL + path)
	return parsed
}

func contains(body []byte, text string) bool {
	return strings.Contains(string(body), text)
}

func jsonEncoder(buffer *bytes.Buffer) *json.Encoder {
	return json.NewEncoder(buffer)
}
