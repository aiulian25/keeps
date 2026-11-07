package httpapi

import (
	"bytes"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"strings"
	"testing"
)

func TestBaseSetsHeadersAndNeverLogsQuery(t *testing.T) {
	var logs bytes.Buffer
	previous := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&logs, nil)))
	defer slog.SetDefault(previous)

	handler := Base(true, http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		writer.WriteHeader(http.StatusNoContent)
	}))
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/api/v1/items?q=secret-token", nil))

	for _, header := range []string{"Content-Security-Policy", "X-Content-Type-Options", "Strict-Transport-Security", "Referrer-Policy"} {
		if recorder.Header().Get(header) == "" {
			t.Errorf("missing %s", header)
		}
	}
	if strings.Contains(logs.String(), "secret-token") {
		t.Fatalf("query string leaked into logs: %s", logs.String())
	}
}

func TestDecodeJSONRejectsUnknownFieldsAndTrailingData(t *testing.T) {
	var target struct {
		Name string `json:"name"`
	}
	for _, body := range []string{`{"name":"a","householdId":2}`, `{"name":"a"} {}`, `not json`} {
		request := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(body))
		if err := DecodeJSON(httptest.NewRecorder(), request, 1024, &target); err == nil {
			t.Errorf("accepted %s", body)
		}
	}
	request := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{"name":"`+strings.Repeat("a", 2000)+`"}`))
	if err := DecodeJSON(httptest.NewRecorder(), request, 1024, &target); err != ErrBodyTooLarge {
		t.Errorf("oversize err = %v", err)
	}
}

func TestClientAddressIgnoresForwardedFromUntrustedPeers(t *testing.T) {
	request := httptest.NewRequest(http.MethodGet, "/", nil)
	request.RemoteAddr = "203.0.113.9:4000"
	request.Header.Set("X-Forwarded-For", "1.2.3.4")
	if got := ClientAddress(request, nil); got != "203.0.113.9" {
		t.Fatalf("untrusted got %s", got)
	}
	trusted := []netip.Prefix{netip.MustParsePrefix("203.0.113.0/24")}
	if got := ClientAddress(request, trusted); got != "1.2.3.4" {
		t.Fatalf("trusted got %s", got)
	}
}
