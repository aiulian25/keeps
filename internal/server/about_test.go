package server

import (
	"net/http"
	"testing"
)

func TestTheRunningVersionIsShownOnlyAfterSignIn(t *testing.T) {
	const release = "9.8.7"
	server := newTestServer(t)
	server.application.Version = release

	if result := server.newClient().do(http.MethodGet, "/api/v1/about", nil); result.Status != http.StatusUnauthorized || contains(result.Body, release) {
		t.Fatalf("the version was shown without signing in: %d %s", result.Status, result.Body)
	}
	var about struct {
		Version string `json:"version"`
	}
	server.claimed().do(http.MethodGet, "/api/v1/about", nil).json(t, &about)
	if about.Version != release {
		t.Fatalf("signed in, the version reads %q", about.Version)
	}
}
