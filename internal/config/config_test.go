package config

import "testing"

func lookupFrom(values map[string]string) func(string) (string, bool) {
	return func(name string) (string, bool) {
		value, found := values[name]
		return value, found
	}
}

func TestLoadOrigin(t *testing.T) {
	cases := []struct {
		name        string
		environment map[string]string
		wantError   bool
	}{
		{"https origin", map[string]string{EnvironmentOrigin: "https://keeps.example.com"}, false},
		{"https with port", map[string]string{EnvironmentOrigin: "https://keeps.example.com:8443"}, false},
		{"missing", map[string]string{}, true},
		{"trailing slash", map[string]string{EnvironmentOrigin: "https://keeps.example.com/"}, true},
		{"path", map[string]string{EnvironmentOrigin: "https://example.com/keeps"}, true},
		{"plain http", map[string]string{EnvironmentOrigin: "http://keeps.example.com"}, true},
		{"http localhost without dev flag", map[string]string{EnvironmentOrigin: "http://localhost:6030"}, true},
		{"http localhost with dev flag", map[string]string{EnvironmentOrigin: "http://localhost:6030", EnvironmentDevelopment: "1"}, false},
		{"http lan ip with dev flag", map[string]string{EnvironmentOrigin: "http://192.168.10.5:6030", EnvironmentDevelopment: "1"}, true},
		{"userinfo", map[string]string{EnvironmentOrigin: "https://user@keeps.example.com"}, true},
		{"half tls", map[string]string{EnvironmentOrigin: "https://keeps.example.com", EnvironmentTLSCertificate: "/c.pem"}, true},
		{"bad proxy", map[string]string{EnvironmentOrigin: "https://keeps.example.com", EnvironmentTrustedProxies: "nope"}, true},
		{"good proxies", map[string]string{EnvironmentOrigin: "https://keeps.example.com", EnvironmentTrustedProxies: "172.18.0.0/16, 10.0.0.2"}, false},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			configuration, err := Load(lookupFrom(testCase.environment))
			if testCase.wantError != (err != nil) {
				t.Fatalf("error = %v, wantError %v", err, testCase.wantError)
			}
			if err == nil && configuration.DataDirectory != DefaultDataDirectory {
				t.Fatalf("data directory = %q", configuration.DataDirectory)
			}
		})
	}
}
