// Package config reads the small set of deployment settings Keeps accepts.
package config

import (
	"errors"
	"fmt"
	"net/netip"
	"net/url"
	"path/filepath"
	"strings"
)

const (
	EnvironmentOrigin           = "APP_ORIGIN"
	EnvironmentDataDirectory    = "KEEPS_DATA_DIR"
	EnvironmentDevelopment      = "KEEPS_DEV_LOOPBACK"
	EnvironmentTrustedProxies   = "KEEPS_TRUSTED_PROXIES"
	EnvironmentTLSCertificate   = "KEEPS_TLS_CERT_FILE"
	EnvironmentTLSKey           = "KEEPS_TLS_KEY_FILE"
	DefaultDataDirectory        = "/data"
	developmentLoopbackHostname = "localhost"
	enabledValue                = "1"
)

type Configuration struct {
	Origin              *url.URL
	DataDirectory       string
	DevelopmentLoopback bool
	TrustedProxies      []netip.Prefix
	TLSCertificateFile  string
	TLSKeyFile          string
}

func (c Configuration) DatabasePath() string {
	return filepath.Join(c.DataDirectory, DatabaseFileName)
}

func (c Configuration) Directory(name string) string {
	return filepath.Join(c.DataDirectory, name)
}

func (c Configuration) SecureCookies() bool {
	return c.Origin.Scheme == "https"
}

func (c Configuration) DirectTLS() bool {
	return c.TLSCertificateFile != ""
}

// Load validates settings from a lookup function (os.LookupEnv in production).
func Load(lookup func(string) (string, bool)) (Configuration, error) {
	value := func(name string) string {
		found, _ := lookup(name)
		return strings.TrimSpace(found)
	}

	configuration := Configuration{
		DataDirectory:       value(EnvironmentDataDirectory),
		DevelopmentLoopback: value(EnvironmentDevelopment) == enabledValue,
		TLSCertificateFile:  value(EnvironmentTLSCertificate),
		TLSKeyFile:          value(EnvironmentTLSKey),
	}
	if configuration.DataDirectory == "" {
		configuration.DataDirectory = DefaultDataDirectory
	}

	origin, err := parseOrigin(value(EnvironmentOrigin), configuration.DevelopmentLoopback)
	if err != nil {
		return Configuration{}, err
	}
	configuration.Origin = origin

	proxies, err := parseTrustedProxies(value(EnvironmentTrustedProxies))
	if err != nil {
		return Configuration{}, err
	}
	configuration.TrustedProxies = proxies

	tlsHalfConfigured := (configuration.TLSCertificateFile == "") != (configuration.TLSKeyFile == "")
	if tlsHalfConfigured {
		return Configuration{}, fmt.Errorf("set both %s and %s for direct TLS, or neither", EnvironmentTLSCertificate, EnvironmentTLSKey)
	}
	return configuration, nil
}

func parseOrigin(raw string, developmentLoopback bool) (*url.URL, error) {
	if raw == "" {
		return nil, fmt.Errorf("%s is required: set it to the HTTPS address you configured, e.g. https://keeps.example.com", EnvironmentOrigin)
	}
	origin, err := url.Parse(raw)
	if err != nil {
		return nil, fmt.Errorf("%s is not a valid URL: %w", EnvironmentOrigin, err)
	}

	hasExtraParts := origin.User != nil || (origin.Path != "" && origin.Path != "/") || origin.RawQuery != "" || origin.Fragment != ""
	if hasExtraParts || origin.Host == "" {
		return nil, fmt.Errorf("%s must be only a scheme and host, without path, query or trailing slash", EnvironmentOrigin)
	}
	if strings.HasSuffix(raw, "/") {
		return nil, fmt.Errorf("%s must not end with a slash", EnvironmentOrigin)
	}

	if origin.Scheme == "https" {
		return origin, nil
	}
	isLoopbackDevelopment := origin.Scheme == "http" && developmentLoopback && origin.Hostname() == developmentLoopbackHostname
	if isLoopbackDevelopment {
		return origin, nil
	}
	return nil, errors.New(EnvironmentOrigin + " must use https (http://localhost is allowed only with " + EnvironmentDevelopment + "=1)")
}

func parseTrustedProxies(raw string) ([]netip.Prefix, error) {
	if raw == "" {
		return nil, nil
	}
	var prefixes []netip.Prefix
	for _, entry := range strings.Split(raw, ",") {
		entry = strings.TrimSpace(entry)
		prefix, err := netip.ParsePrefix(entry)
		if err != nil {
			address, addressErr := netip.ParseAddr(entry)
			if addressErr != nil {
				return nil, fmt.Errorf("%s entry %q is not an IP address or CIDR prefix", EnvironmentTrustedProxies, entry)
			}
			prefix = netip.PrefixFrom(address, address.BitLen())
		}
		prefixes = append(prefixes, prefix)
	}
	return prefixes, nil
}
