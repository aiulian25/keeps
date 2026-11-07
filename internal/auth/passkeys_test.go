package auth

import (
	"net/url"
	"testing"

	"github.com/aiulian25/keeps/internal/config"
)

func TestPasskeysOnlyOnDomainNames(t *testing.T) {
	cases := map[string]bool{
		"https://keeps.example.com":  true,
		"http://localhost:6030":      true,
		"https://192.168.10.65:6030": false,
		"https://[2001:db8::1]:6030": false,
		"http://keeps.example.com":   false,
	}
	for origin, want := range cases {
		parsed, _ := url.Parse(origin)
		relyingParty, err := newPasskeyRelyingParty(config.Configuration{Origin: parsed})
		if err != nil {
			t.Fatalf("%s: %v", origin, err)
		}
		if (relyingParty != nil) != want {
			t.Errorf("%s: passkeys available = %v, want %v", origin, relyingParty != nil, want)
		}
	}
}
