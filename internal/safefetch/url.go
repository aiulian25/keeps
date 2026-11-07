package safefetch

import (
	"golang.org/x/net/idna"
	"net"
	"net/netip"
	"net/url"
	"strconv"
	"strings"
	"unicode"
)

type signedLinkRule struct {
	queryKeys     []string
	pathContains  string
	requiresQuery bool
}

// signedPrivateDocumentLinkRules recognise pre-signed object-store and
// invoice links. Keys are matched case-insensitively; every key in a rule must
// be present. Matching links are refused before any network access.
var signedPrivateDocumentLinkRules = []signedLinkRule{
	{queryKeys: []string{"x-amz-signature"}},
	{queryKeys: []string{"x-amz-credential"}},
	{queryKeys: []string{"signature", "expires"}},
	{queryKeys: []string{"sig", "se"}},
	{queryKeys: []string{"token"}, pathContains: "invoice"},
	{pathContains: "/invoice", requiresQuery: true},
}

var defaultAllowedPorts = map[int]bool{defaultHTTPSPort: true}

// NormaliseURL applies the same validation Get performs before any network
// access, so the UI can reject a pasted link early.
func NormaliseURL(raw string) (*url.URL, error) {
	return normaliseURL(raw, defaultAllowedPorts)
}

func normaliseURL(raw string, allowedPorts map[int]bool) (*url.URL, error) {
	if raw == "" || strings.ContainsFunc(raw, isForbiddenURLRune) {
		return nil, newError(CodeInvalidURL, "", reasonMalformedURL)
	}
	parsed, err := url.Parse(raw)
	if err != nil || parsed.Opaque != "" {
		return nil, newError(CodeInvalidURL, "", reasonMalformedURL)
	}
	if parsed.Scheme != httpsScheme {
		return nil, newError(CodeInvalidURL, "", reasonSchemeNotHTTPS)
	}
	if parsed.User != nil {
		return nil, newError(CodeInvalidURL, "", reasonUserinfo)
	}
	host, err := normaliseHost(parsed.Hostname())
	if err != nil {
		return nil, err
	}
	port, err := parsePort(parsed)
	if err != nil {
		return nil, err
	}
	if !allowedPorts[port] {
		return nil, newError(CodeInvalidURL, host, reasonPortNotAllowed)
	}

	parsed.Host = joinHost(host, port)
	parsed.Fragment = ""
	parsed.RawFragment = ""

	if isSignedPrivateDocumentLink(parsed) {
		return nil, newError(CodePrivateDocumentLink, host, reasonPrivateDocument)
	}
	return parsed, nil
}

func isForbiddenURLRune(character rune) bool {
	return character == '\\' || unicode.IsSpace(character) || unicode.IsControl(character)
}

func normaliseHost(hostname string) (string, error) {
	if hostname == "" {
		return "", newError(CodeInvalidURL, "", reasonEmptyHost)
	}
	if address, err := netip.ParseAddr(hostname); err == nil {
		return normaliseAddressHost(address)
	}

	name, err := idna.Lookup.ToASCII(strings.TrimSuffix(hostname, labelSeparator))
	if err != nil || !isASCII(name) {
		return "", newError(CodeInvalidURL, "", reasonNonASCIIHost)
	}
	name = strings.ToLower(name)
	if !isValidDNSName(name) {
		return "", newError(CodeInvalidURL, "", reasonInvalidHost)
	}
	labels := strings.Split(name, labelSeparator)
	if isNumericLabel(labels[len(labels)-1]) {
		return "", newError(CodeInvalidURL, "", reasonNumericHost)
	}
	isLocalhost := name == localhostName || strings.HasSuffix(name, localhostSuffix)
	isSingleLabel := len(labels) == 1
	if isLocalhost || isSingleLabel {
		return "", newError(CodeNotPublic, name, reasonNonPublicHost)
	}
	return name, nil
}

func normaliseAddressHost(address netip.Addr) (string, error) {
	if address.Zone() != "" {
		return "", newError(CodeInvalidURL, "", reasonZonedAddress)
	}
	if !isPublicAddress(address) {
		return "", newError(CodeNotPublic, address.String(), reasonNonPublicHost)
	}
	return address.String(), nil
}

func isASCII(text string) bool {
	for index := 0; index < len(text); index++ {
		if text[index] > unicode.MaxASCII {
			return false
		}
	}
	return true
}

func isValidDNSName(name string) bool {
	if name == "" || len(name) > maxHostnameLength {
		return false
	}
	for _, label := range strings.Split(name, labelSeparator) {
		if !isValidLabel(label) {
			return false
		}
	}
	return true
}

func isValidLabel(label string) bool {
	if label == "" || len(label) > maxLabelLength {
		return false
	}
	if strings.HasPrefix(label, "-") || strings.HasSuffix(label, "-") {
		return false
	}
	for _, character := range label {
		isLetterOrDigit := (character >= 'a' && character <= 'z') || (character >= '0' && character <= '9')
		if !isLetterOrDigit && character != '-' {
			return false
		}
	}
	return true
}

// isNumericLabel catches the WHATWG rule that a host whose last label is a
// number is parsed as IPv4 (127.1, 0x7f.1, 2130706433, 0177.0.0.1).
func isNumericLabel(label string) bool {
	digits := strings.TrimPrefix(label, hexNumberPrefix)
	isHex := digits != label
	for _, character := range digits {
		isDecimal := character >= '0' && character <= '9'
		isHexLetter := isHex && character >= 'a' && character <= 'f'
		if !isDecimal && !isHexLetter {
			return false
		}
	}
	return true
}

func parsePort(parsed *url.URL) (int, error) {
	if strings.HasSuffix(parsed.Host, ":") {
		return 0, newError(CodeInvalidURL, "", reasonInvalidPort)
	}
	portText := parsed.Port()
	if portText == "" {
		return defaultHTTPSPort, nil
	}
	port, err := strconv.Atoi(portText)
	if err != nil {
		return 0, newError(CodeInvalidURL, "", reasonInvalidPort)
	}
	return port, nil
}

func joinHost(host string, port int) string {
	if port != defaultHTTPSPort {
		return net.JoinHostPort(host, strconv.Itoa(port))
	}
	if strings.Contains(host, ":") {
		return "[" + host + "]"
	}
	return host
}

func isSignedPrivateDocumentLink(parsed *url.URL) bool {
	presentKeys := map[string]bool{}
	for key := range parsed.Query() {
		presentKeys[strings.ToLower(key)] = true
	}
	path := strings.ToLower(parsed.Path)
	hasQuery := parsed.RawQuery != ""
	for _, rule := range signedPrivateDocumentLinkRules {
		if rule.matches(presentKeys, path, hasQuery) {
			return true
		}
	}
	return false
}

func (rule signedLinkRule) matches(presentKeys map[string]bool, path string, hasQuery bool) bool {
	if rule.requiresQuery && !hasQuery {
		return false
	}
	if rule.pathContains != "" && !strings.Contains(path, rule.pathContains) {
		return false
	}
	for _, key := range rule.queryKeys {
		if !presentKeys[key] {
			return false
		}
	}
	return true
}
