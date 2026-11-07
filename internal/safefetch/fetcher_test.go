package safefetch

import (
	"bytes"
	"compress/gzip"
	"context"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// testHost is the name the httptest certificate is issued for.
const testHost = "example.com"

var (
	loopbackAddress = netip.MustParseAddr("127.0.0.1")
	privateAddress  = netip.MustParseAddr("10.0.0.1")
)

type fakeResolver struct {
	mutex   sync.Mutex
	answers map[string][][]netip.Addr
	calls   map[string]int
}

// newFakeResolver answers each host with the next entry of its sequence,
// repeating the last entry once the sequence is exhausted.
func newFakeResolver(answers map[string][][]netip.Addr) *fakeResolver {
	return &fakeResolver{answers: answers, calls: map[string]int{}}
}

func (r *fakeResolver) LookupNetIP(_ context.Context, _ string, host string) ([]netip.Addr, error) {
	r.mutex.Lock()
	defer r.mutex.Unlock()
	call := r.calls[host]
	r.calls[host]++
	sequence, exists := r.answers[host]
	if !exists {
		return nil, &net.DNSError{Err: "no such host", Name: host, IsNotFound: true}
	}
	return sequence[min(call, len(sequence)-1)], nil
}

func (r *fakeResolver) callsFor(host string) int {
	r.mutex.Lock()
	defer r.mutex.Unlock()
	return r.calls[host]
}

func (r *fakeResolver) totalCalls() int {
	r.mutex.Lock()
	defer r.mutex.Unlock()
	total := 0
	for _, count := range r.calls {
		total += count
	}
	return total
}

type harness struct {
	server   *httptest.Server
	fetcher  *Fetcher
	resolver *fakeResolver
	baseURL  string
	port     int
}

func newHarness(t *testing.T, handler http.Handler, answers map[string][][]netip.Addr) *harness {
	t.Helper()
	server := httptest.NewTLSServer(handler)
	t.Cleanup(server.Close)
	serverURL, err := url.Parse(server.URL)
	if err != nil {
		t.Fatal(err)
	}
	port, err := strconv.Atoi(serverURL.Port())
	if err != nil {
		t.Fatal(err)
	}
	if answers == nil {
		answers = map[string][][]netip.Addr{
			testHost:                   {{loopbackAddress}},
			"internal." + testHost:     {{privateAddress}},
			"mixed." + testHost:        {{netip.MustParseAddr("8.8.8.8"), privateAddress}},
			"unresolvable." + testHost: {},
		}
	}
	resolver := newFakeResolver(answers)
	certificates := server.Client().Transport.(*http.Transport).TLSClientConfig.RootCAs
	fetcher := New(
		WithResolver(resolver),
		WithRootCAs(certificates),
		withAllowedPortsForTests(port),
		withInsecureAllowLoopbackForTests(),
	)
	return &harness{
		server:   server,
		fetcher:  fetcher,
		resolver: resolver,
		baseURL:  fmt.Sprintf("https://%s:%d", testHost, port),
		port:     port,
	}
}

func (h *harness) urlFor(host, path string) string {
	return fmt.Sprintf("https://%s:%d%s", host, h.port, path)
}

func (h *harness) get(t *testing.T, path string, limits Limits) (*Response, error) {
	t.Helper()
	return h.fetcher.Get(context.Background(), h.baseURL+path, limits, "text/html")
}

func expectCode(t *testing.T, err error, want ErrorCode) {
	t.Helper()
	if err == nil {
		t.Fatalf("expected %s, got success", want)
	}
	if got := errorCode(t, err); got != want {
		t.Fatalf("code = %s (%v), want %s", got, err, want)
	}
}

func TestGetSuccessReturnsBodyAndFinalURL(t *testing.T) {
	var received http.Header
	mux := http.NewServeMux()
	mux.HandleFunc("/start", func(writer http.ResponseWriter, request *http.Request) {
		http.Redirect(writer, request, "/final?variant=blue", http.StatusFound)
	})
	mux.HandleFunc("/final", func(writer http.ResponseWriter, request *http.Request) {
		received = request.Header.Clone()
		writer.Header().Set("Content-Type", "text/html; charset=utf-8")
		fmt.Fprint(writer, "<html>product</html>")
	})
	h := newHarness(t, mux, nil)

	response, err := h.fetcher.Get(context.Background(), h.baseURL+"/start#section", DefaultPageLimits(), "text/html")
	if err != nil {
		t.Fatal(err)
	}
	if string(response.Body) != "<html>product</html>" {
		t.Fatalf("body = %q", response.Body)
	}
	if response.StatusCode != http.StatusOK || !strings.HasPrefix(response.ContentType, "text/html") {
		t.Fatalf("status %d content type %q", response.StatusCode, response.ContentType)
	}
	if want := h.baseURL + "/final?variant=blue"; response.FinalURL.String() != want {
		t.Fatalf("final URL = %s, want %s", response.FinalURL, want)
	}
	if received.Get("User-Agent") != UserAgent {
		t.Fatalf("user agent = %q", received.Get("User-Agent"))
	}
	if received.Get("Accept") != "text/html" || received.Get("Accept-Language") != acceptLanguage {
		t.Fatalf("accept headers = %q / %q", received.Get("Accept"), received.Get("Accept-Language"))
	}
	for _, forbidden := range []string{"Cookie", "Authorization", "Referer"} {
		if received.Get(forbidden) != "" {
			t.Fatalf("%s header must not be sent", forbidden)
		}
	}
}

func TestRedirectToPrivateDestinationIsRejected(t *testing.T) {
	var targets = map[string]ErrorCode{}
	mux := http.NewServeMux()
	mux.HandleFunc("/go", func(writer http.ResponseWriter, request *http.Request) {
		http.Redirect(writer, request, request.URL.Query().Get("to"), http.StatusFound)
	})
	h := newHarness(t, mux, nil)
	targets[h.urlFor("internal."+testHost, "/")] = CodeNotPublic
	targets[h.urlFor("mixed."+testHost, "/")] = CodeNotPublic
	targets["https://10.0.0.1/"] = CodeNotPublic
	targets["https://169.254.169.254/latest/meta-data/"] = CodeNotPublic
	targets["https://[::ffff:127.0.0.1]/"] = CodeNotPublic
	targets["https://localhost/"] = CodeNotPublic
	targets["http://"+testHost+"/"] = CodeInvalidURL
	targets["https://"+testHost+":8443/"] = CodeInvalidURL

	for target, want := range targets {
		t.Run(target, func(t *testing.T) {
			_, err := h.get(t, "/go?to="+url.QueryEscape(target), DefaultPageLimits())
			expectCode(t, err, want)
		})
	}
}

func TestHostResolvingToAnyPrivateAddressIsRejected(t *testing.T) {
	h := newHarness(t, http.NotFoundHandler(), nil)
	_, err := h.fetcher.Get(context.Background(), h.urlFor("mixed."+testHost, "/"), DefaultPageLimits(), "")
	expectCode(t, err, CodeNotPublic)
	_, err = h.fetcher.Get(context.Background(), h.urlFor("internal."+testHost, "/"), DefaultPageLimits(), "")
	expectCode(t, err, CodeNotPublic)
	_, err = h.fetcher.Get(context.Background(), h.urlFor("unknown."+testHost, "/"), DefaultPageLimits(), "")
	expectCode(t, err, CodeFetchFailed)
}

func TestRedirectChainLongerThanLimitFails(t *testing.T) {
	var hits atomic.Int32
	mux := http.NewServeMux()
	mux.HandleFunc("/loop/", func(writer http.ResponseWriter, request *http.Request) {
		count := hits.Add(1)
		http.Redirect(writer, request, fmt.Sprintf("/loop/%d", count), http.StatusMovedPermanently)
	})
	h := newHarness(t, mux, nil)

	limits := DefaultPageLimits()
	_, err := h.get(t, "/loop/0", limits)
	expectCode(t, err, CodeFetchFailed)
	if got, want := int(hits.Load()), limits.MaxRedirects+1; got != want {
		t.Fatalf("server hit %d times, want %d", got, want)
	}
}

func TestRedirectToLoginPathNeedsLogin(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/product", func(writer http.ResponseWriter, request *http.Request) {
		http.Redirect(writer, request, "/ap/signin?return=/product", http.StatusFound)
	})
	h := newHarness(t, mux, nil)
	_, err := h.get(t, "/product", DefaultPageLimits())
	expectCode(t, err, CodeNeedsLogin)
}

func TestStatusMapping(t *testing.T) {
	cases := []struct {
		status int
		body   string
		want   ErrorCode
	}{
		{http.StatusNotFound, "", CodeNotFound},
		{http.StatusGone, "", CodeNotFound},
		{http.StatusForbidden, "Access denied", CodeBlocked},
		{http.StatusForbidden, `<form><input type="password"></form>`, CodeNeedsLogin},
		{http.StatusUnauthorized, "", CodeNeedsLogin},
		{http.StatusTooManyRequests, "", CodeBlocked},
		{http.StatusServiceUnavailable, "", CodeBlocked},
		{http.StatusInternalServerError, "", CodeFetchFailed},
		{http.StatusTeapot, "", CodeFetchFailed},
	}
	for _, testCase := range cases {
		t.Run(strconv.Itoa(testCase.status), func(t *testing.T) {
			handler := http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
				writer.WriteHeader(testCase.status)
				fmt.Fprint(writer, testCase.body)
			})
			h := newHarness(t, handler, nil)
			_, err := h.get(t, "/", DefaultPageLimits())
			expectCode(t, err, testCase.want)
		})
	}
}

func TestOversizedBodyIsTooLarge(t *testing.T) {
	const limitBytes = 1 << 10
	handler := http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		writer.Write(bytes.Repeat([]byte("a"), 4*limitBytes))
	})
	h := newHarness(t, handler, nil)
	limits := DefaultPageLimits()
	limits.MaxBodyBytes = limitBytes
	_, err := h.get(t, "/", limits)
	expectCode(t, err, CodeTooLarge)
}

func TestGzipBombIsStoppedByDecodedLimit(t *testing.T) {
	const decodedBytes = 64 << 20
	const limitBytes = 1 << 20
	var compressed bytes.Buffer
	writer := gzip.NewWriter(&compressed)
	writer.Write(make([]byte, decodedBytes))
	writer.Close()
	if compressed.Len() >= limitBytes {
		t.Fatalf("compressed payload %d bytes is not smaller than the limit", compressed.Len())
	}

	handler := http.HandlerFunc(func(responseWriter http.ResponseWriter, _ *http.Request) {
		responseWriter.Header().Set("Content-Encoding", "gzip")
		responseWriter.Header().Set("Content-Type", "text/html")
		responseWriter.Write(compressed.Bytes())
	})
	h := newHarness(t, handler, nil)
	limits := DefaultPageLimits()
	limits.MaxBodyBytes = limitBytes
	_, err := h.get(t, "/", limits)
	expectCode(t, err, CodeTooLarge)
}

func TestSlowServerTimesOut(t *testing.T) {
	handler := http.HandlerFunc(func(_ http.ResponseWriter, request *http.Request) {
		select {
		case <-request.Context().Done():
		case <-time.After(5 * time.Second):
		}
	})
	h := newHarness(t, handler, nil)
	limits := DefaultPageLimits()
	limits.TotalTimeout = 200 * time.Millisecond
	_, err := h.get(t, "/", limits)
	expectCode(t, err, CodeTimeout)
}

func TestSignedInvoiceLinkIsNeverFetched(t *testing.T) {
	var hits atomic.Int32
	handler := http.HandlerFunc(func(http.ResponseWriter, *http.Request) { hits.Add(1) })
	h := newHarness(t, handler, nil)
	signed := h.baseURL + "/invoices/INV-1.pdf?X-Amz-Algorithm=AWS4-HMAC-SHA256&X-Amz-Credential=key&X-Amz-Signature=deadbeef"
	_, err := h.fetcher.Get(context.Background(), signed, DefaultPageLimits(), "")
	expectCode(t, err, CodePrivateDocumentLink)
	if hits.Load() != 0 || h.resolver.totalCalls() != 0 {
		t.Fatalf("signed link caused network activity: %d hits, %d lookups", hits.Load(), h.resolver.totalCalls())
	}
}

func TestEnvironmentProxyIsIgnored(t *testing.T) {
	var proxyHits atomic.Int32
	proxy := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		proxyHits.Add(1)
		writer.WriteHeader(http.StatusBadGateway)
	}))
	t.Cleanup(proxy.Close)
	for _, variable := range []string{"HTTPS_PROXY", "https_proxy", "HTTP_PROXY", "http_proxy", "ALL_PROXY"} {
		t.Setenv(variable, proxy.URL)
	}
	t.Setenv("NO_PROXY", "")

	handler := http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) { fmt.Fprint(writer, "direct") })
	h := newHarness(t, handler, nil)
	response, err := h.get(t, "/", DefaultPageLimits())
	if err != nil {
		t.Fatal(err)
	}
	if string(response.Body) != "direct" || proxyHits.Load() != 0 {
		t.Fatalf("body %q, proxy hits %d", response.Body, proxyHits.Load())
	}
	transport := h.fetcher.newClient(DefaultPageLimits()).Transport.(*http.Transport)
	if transport.Proxy != nil {
		t.Fatal("transport must never use a proxy")
	}
}

// TestDNSRebindingCannotReachPrivateAddress uses a resolver that answers with
// a permitted address first and a private address afterwards.
func TestDNSRebindingCannotReachPrivateAddress(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/ok", func(writer http.ResponseWriter, _ *http.Request) { fmt.Fprint(writer, "ok") })
	mux.HandleFunc("/bounce", func(writer http.ResponseWriter, request *http.Request) {
		http.Redirect(writer, request, "/ok", http.StatusFound)
	})
	rebinding := map[string][][]netip.Addr{testHost: {{loopbackAddress}, {privateAddress}}}

	t.Run("single fetch resolves exactly once", func(t *testing.T) {
		h := newHarness(t, mux, rebinding)
		if _, err := h.get(t, "/ok", DefaultPageLimits()); err != nil {
			t.Fatal(err)
		}
		if calls := h.resolver.callsFor(testHost); calls != 1 {
			t.Fatalf("resolver called %d times; net/http must not resolve again", calls)
		}
		_, err := h.get(t, "/ok", DefaultPageLimits())
		expectCode(t, err, CodeNotPublic)
	})

	t.Run("rebinding between redirect hops", func(t *testing.T) {
		h := newHarness(t, mux, rebinding)
		_, err := h.get(t, "/bounce", DefaultPageLimits())
		expectCode(t, err, CodeNotPublic)
	})

	t.Run("dial-time check rejects non-public sockets", func(t *testing.T) {
		h := newHarness(t, mux, rebinding)
		strict := New(withAllowedPortsForTests(h.port))
		dialer := strict.newDialer(DefaultPageLimits())
		for _, address := range []string{
			net.JoinHostPort(loopbackAddress.String(), strconv.Itoa(h.port)),
			net.JoinHostPort("::1", strconv.Itoa(h.port)),
			net.JoinHostPort(privateAddress.String(), strconv.Itoa(h.port)),
		} {
			connection, err := dialer.DialContext(context.Background(), dialNetwork, address)
			if connection != nil {
				connection.Close()
			}
			expectCode(t, err, CodeNotPublic)
		}
	})

	t.Run("loopback is not permitted without the test option", func(t *testing.T) {
		h := newHarness(t, mux, rebinding)
		certificates := h.server.Client().Transport.(*http.Transport).TLSClientConfig.RootCAs
		strict := New(WithResolver(newFakeResolver(rebinding)), WithRootCAs(certificates), withAllowedPortsForTests(h.port))
		_, err := strict.Get(context.Background(), h.baseURL+"/ok", DefaultPageLimits(), "")
		expectCode(t, err, CodeNotPublic)
	})
}

func TestErrorDetailNeverContainsQuery(t *testing.T) {
	const secret = "s3cr3t-token-value"
	h := newHarness(t, http.NotFoundHandler(), nil)
	urls := []string{
		h.baseURL + "/missing?session=" + secret,
		h.urlFor("unknown."+testHost, "/?session="+secret),
		h.urlFor("internal."+testHost, "/?session="+secret),
		"https://10.0.0.1/?session=" + secret,
		"http://example.com/?session=" + secret,
	}
	for _, target := range urls {
		_, err := h.fetcher.Get(context.Background(), target, DefaultPageLimits(), "")
		if err == nil {
			t.Fatalf("expected an error for %s", target)
		}
		if strings.Contains(err.Error(), secret) || strings.Contains(err.Error(), "session") {
			t.Fatalf("error leaks query: %v", err)
		}
	}
}

func TestPerHostConcurrencyIsOne(t *testing.T) {
	var active, peak atomic.Int32
	handler := http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		current := active.Add(1)
		for {
			observed := peak.Load()
			if current <= observed || peak.CompareAndSwap(observed, current) {
				break
			}
		}
		time.Sleep(50 * time.Millisecond)
		active.Add(-1)
		fmt.Fprint(writer, "ok")
	})
	h := newHarness(t, handler, nil)

	const parallelRequests = 4
	var group sync.WaitGroup
	for range parallelRequests {
		group.Go(func() {
			if _, err := h.get(t, "/", DefaultPageLimits()); err != nil {
				t.Error(err)
			}
		})
	}
	group.Wait()
	if peak.Load() != perHostConcurrency {
		t.Fatalf("peak concurrent requests to one host = %d", peak.Load())
	}
}

func TestHostSlotRespectsContext(t *testing.T) {
	fetcher := New()
	release, err := fetcher.acquireHost(context.Background(), testHost)
	if err != nil {
		t.Fatal(err)
	}
	defer release()
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	if _, err := fetcher.acquireHost(ctx, testHost); err == nil {
		t.Fatal("second acquisition must wait and then honour the context")
	}
}
