package safefetch

import (
	"context"
	"errors"
	"net/netip"
	"testing"
)

func errorCode(t *testing.T, err error) ErrorCode {
	t.Helper()
	var fetchError *Error
	if !errors.As(err, &fetchError) {
		t.Fatalf("expected *Error, got %T %v", err, err)
	}
	return fetchError.Code
}

func TestAdversarialURLsFailClosedWithoutNetwork(t *testing.T) {
	cases := []struct {
		name string
		url  string
		want ErrorCode
	}{
		{"localhost", "https://localhost/", CodeNotPublic},
		{"localhost subdomain", "https://shop.localhost/", CodeNotPublic},
		{"single label intranet host", "https://intranet/", CodeNotPublic},
		{"short IPv4", "https://127.1/", CodeInvalidURL},
		{"hex IPv4", "https://0x7f.1/", CodeInvalidURL},
		{"octal IPv4", "https://0177.0.0.1/", CodeInvalidURL},
		{"integer IPv4", "https://2130706433/", CodeInvalidURL},
		{"trailing dot IPv4", "https://127.0.0.1./", CodeInvalidURL},
		{"IPv6 loopback", "https://[::1]/", CodeNotPublic},
		{"IPv4-mapped loopback", "https://[::ffff:127.0.0.1]/", CodeNotPublic},
		{"cloud metadata", "https://169.254.169.254/latest/meta-data/", CodeNotPublic},
		{"private 10/8", "https://10.0.0.1/", CodeNotPublic},
		{"CGNAT", "https://100.64.0.1/", CodeNotPublic},
		{"unique local IPv6", "https://[fc00::1]/", CodeNotPublic},
		{"NAT64 embedded loopback", "https://[64:ff9b::7f00:1]/", CodeNotPublic},
		{"6to4 embedded loopback", "https://[2002:7f00:1::]/", CodeNotPublic},
		{"zoned link-local", "https://[fe80::1%25eth0]/", CodeInvalidURL},
		{"http scheme", "http://example.com/", CodeInvalidURL},
		{"ftp scheme", "ftp://example.com/", CodeInvalidURL},
		{"non-443 port", "https://example.com:8443/", CodeInvalidURL},
		{"empty port", "https://example.com:/", CodeInvalidURL},
		{"userinfo", "https://user:pass@example.com/", CodeInvalidURL},
		{"userinfo host confusion", "https://example.com@10.0.0.1/", CodeInvalidURL},
		{"empty host", "https:///path", CodeInvalidURL},
		{"backslash", "https://example.com\\@10.0.0.1/", CodeInvalidURL},
		{"whitespace", "https://exam ple.com/", CodeInvalidURL},
		{"invalid international host", "https://xn--a.example/", CodeInvalidURL},
		{"opaque", "https:example.com", CodeInvalidURL},
		{"S3 signed invoice", "https://bucket.s3.amazonaws.com/invoice-123.pdf?X-Amz-Algorithm=AWS4-HMAC-SHA256&X-Amz-Credential=a&X-Amz-Signature=b", CodePrivateDocumentLink},
		{"S3 credential only", "https://bucket.s3.amazonaws.com/file.pdf?x-amz-credential=a", CodePrivateDocumentLink},
		{"CloudFront signed", "https://cdn.example.com/doc.pdf?Expires=1&Signature=abc&Key-Pair-Id=k", CodePrivateDocumentLink},
		{"Azure SAS", "https://account.blob.core.windows.net/c/doc.pdf?sv=1&se=2030&sig=abc", CodePrivateDocumentLink},
		{"invoice token", "https://shop.example.com/orders/invoice.pdf?token=abc", CodePrivateDocumentLink},
		{"invoice path with query", "https://shop.example.com/account/invoice/42?download=1", CodePrivateDocumentLink},
	}

	resolver := newFakeResolver(nil)
	fetcher := New(WithResolver(resolver))
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			_, err := NormaliseURL(testCase.url)
			if got := errorCode(t, err); got != testCase.want {
				t.Fatalf("NormaliseURL code = %s, want %s", got, testCase.want)
			}
			_, err = fetcher.Get(context.Background(), testCase.url, DefaultPageLimits(), "")
			if got := errorCode(t, err); got != testCase.want {
				t.Fatalf("Get code = %s, want %s", got, testCase.want)
			}
		})
	}
	if calls := resolver.totalCalls(); calls != 0 {
		t.Fatalf("resolver called %d times; adversarial URLs must be rejected before DNS", calls)
	}
}

func TestNormaliseURLCanonicalises(t *testing.T) {
	normalised, err := NormaliseURL("https://Shop.Example.COM.:443/p/1?colour=red#reviews")
	if err != nil {
		t.Fatal(err)
	}
	const want = "https://shop.example.com/p/1?colour=red"
	if normalised.String() != want {
		t.Fatalf("got %s, want %s", normalised, want)
	}
}

func TestIsPublicAddress(t *testing.T) {
	cases := map[string]bool{
		"8.8.8.8":              true,
		"1.1.1.1":              true,
		"2606:4700::1111":      true,
		"::ffff:8.8.8.8":       true,
		"127.0.0.1":            false,
		"0.0.0.0":              false,
		"10.1.2.3":             false,
		"172.16.0.1":           false,
		"172.31.255.255":       false,
		"192.168.10.1":         false,
		"169.254.169.254":      false,
		"100.64.0.1":           false,
		"192.0.0.8":            false,
		"192.0.2.1":            false,
		"198.18.0.1":           false,
		"198.51.100.1":         false,
		"203.0.113.1":          false,
		"224.0.0.1":            false,
		"240.0.0.1":            false,
		"255.255.255.255":      false,
		"::":                   false,
		"::1":                  false,
		"::ffff:127.0.0.1":     false,
		"::ffff:10.0.0.1":      false,
		"fe80::1":              false,
		"fec0::1":              false,
		"fc00::1":              false,
		"fd12:3456::1":         false,
		"ff02::1":              false,
		"2001:db8::1":          false,
		"100::1":               false,
		"64:ff9b::7f00:1":      false,
		"64:ff9b:1::1":         false,
		"2002:7f00:1::":        false,
		"2001:0:4136:e378::1":  false,
		"3fff::1":              false,
		"::127.0.0.1":          false,
		"5f00::1":              false,
		"2001:2::1":            false,
		"2001:10::1":           false,
		"64:ff9b::808:808":     false,
		"2002:808:808::1":      false,
		"::ffff:169.254.169.1": false,
	}
	for text, want := range cases {
		if got := isPublicAddress(netip.MustParseAddr(text)); got != want {
			t.Errorf("isPublicAddress(%s) = %v, want %v", text, got, want)
		}
	}
}

func TestNormaliseURLConvertsInternationalHostnames(t *testing.T) {
	normalised, err := NormaliseURL("https://Bücher.example/produkt")
	if err != nil {
		t.Fatal(err)
	}
	if normalised.Hostname() != "xn--bcher-kva.example" {
		t.Fatalf("hostname = %q", normalised.Hostname())
	}
}
