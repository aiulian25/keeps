package safefetch

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"io"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"
)

type Limits struct {
	TotalTimeout   time.Duration
	ConnectTimeout time.Duration
	MaxBodyBytes   int64
	MaxRedirects   int
	MaxHeaderBytes int64
}

func DefaultPageLimits() Limits {
	return Limits{
		TotalTimeout:   pageTotalTimeout,
		ConnectTimeout: connectTimeout,
		MaxBodyBytes:   pageMaxBodyBytes,
		MaxRedirects:   defaultMaxRedirects,
		MaxHeaderBytes: defaultMaxHeaderBytes,
	}
}

func DefaultImageLimits() Limits {
	return Limits{
		TotalTimeout:   imageTotalTimeout,
		ConnectTimeout: connectTimeout,
		MaxBodyBytes:   imageMaxBodyBytes,
		MaxRedirects:   defaultMaxRedirects,
		MaxHeaderBytes: defaultMaxHeaderBytes,
	}
}

type Resolver interface {
	LookupNetIP(ctx context.Context, network, host string) ([]netip.Addr, error)
}

type Response struct {
	FinalURL    *url.URL
	StatusCode  int
	ContentType string
	Body        []byte
}

// Fetcher is safe for concurrent use and allows one in-flight request per host.
type Fetcher struct {
	resolver         Resolver
	rootCertificates *x509.CertPool
	allowedPorts     map[int]bool
	allowLoopback    bool

	hostSlotsMutex sync.Mutex
	// ponytail: entries are never evicted; fine for one self-hosted worker, add LRU if host cardinality grows.
	hostSlots map[string]chan struct{}
}

type Option func(*Fetcher)

func WithResolver(resolver Resolver) Option {
	return func(f *Fetcher) { f.resolver = resolver }
}

func WithRootCAs(pool *x509.CertPool) Option {
	return func(f *Fetcher) { f.rootCertificates = pool }
}

// withAllowedPortsForTests and withInsecureAllowLoopbackForTests are
// deliberately unexported: only this package's tests can loosen the policy.
func withAllowedPortsForTests(ports ...int) Option {
	return func(f *Fetcher) {
		for _, port := range ports {
			f.allowedPorts[port] = true
		}
	}
}

func withInsecureAllowLoopbackForTests() Option {
	return func(f *Fetcher) { f.allowLoopback = true }
}

func New(options ...Option) *Fetcher {
	fetcher := &Fetcher{
		resolver:     net.DefaultResolver,
		allowedPorts: map[int]bool{defaultHTTPSPort: true},
		hostSlots:    map[string]chan struct{}{},
	}
	for _, option := range options {
		option(fetcher)
	}
	return fetcher
}

func (f *Fetcher) Get(ctx context.Context, rawURL string, limits Limits, acceptHeader string) (*Response, error) {
	current, err := normaliseURL(rawURL, f.allowedPorts)
	if err != nil {
		return nil, err
	}
	ctx, cancel := context.WithTimeout(ctx, limits.TotalTimeout)
	defer cancel()
	client := f.newClient(limits)

	for redirectCount := 0; ; redirectCount++ {
		response, next, err := f.fetchOnce(ctx, client, current, limits, acceptHeader)
		if err != nil {
			return nil, err
		}
		if next == nil {
			return response, nil
		}
		if redirectCount == limits.MaxRedirects {
			return nil, newError(CodeFetchFailed, current.Hostname(), reasonTooManyRedirects)
		}
		current = next
	}
}

// fetchOnce performs a single hop. It returns either a final response or the
// already-validated next URL to follow.
func (f *Fetcher) fetchOnce(ctx context.Context, client *http.Client, target *url.URL, limits Limits, acceptHeader string) (*Response, *url.URL, error) {
	host := target.Hostname()
	release, err := f.acquireHost(ctx, host)
	if err != nil {
		return nil, nil, classify(err, host)
	}
	defer release()

	request, err := http.NewRequestWithContext(ctx, http.MethodGet, target.String(), nil)
	if err != nil {
		return nil, nil, newError(CodeInvalidURL, host, reasonMalformedURL)
	}
	request.Header.Set(headerUserAgent, UserAgent)
	request.Header.Set(headerAcceptLanguage, acceptLanguage)
	if acceptHeader != "" {
		request.Header.Set(headerAccept, acceptHeader)
	}

	response, err := client.Do(request)
	if err != nil {
		return nil, nil, classify(err, host)
	}
	defer response.Body.Close()

	if isRedirectStatus(response.StatusCode) {
		next, err := f.redirectTarget(target, response)
		return nil, next, err
	}
	if !isSuccessStatus(response.StatusCode) {
		return nil, nil, statusError(response, host, limits)
	}
	body, err := readLimited(response.Body, limits.MaxBodyBytes, host)
	if err != nil {
		return nil, nil, err
	}
	return &Response{
		FinalURL:    target,
		StatusCode:  response.StatusCode,
		ContentType: response.Header.Get(headerContentType),
		Body:        body,
	}, nil, nil
}

func (f *Fetcher) redirectTarget(current *url.URL, response *http.Response) (*url.URL, error) {
	host := current.Hostname()
	location := response.Header.Get(headerLocation)
	if location == "" {
		return nil, newError(CodeFetchFailed, host, reasonRedirectNoLocation)
	}
	reference, err := url.Parse(location)
	if err != nil {
		return nil, newError(CodeInvalidURL, host, reasonMalformedURL)
	}
	next, err := normaliseURL(current.ResolveReference(reference).String(), f.allowedPorts)
	if err != nil {
		return nil, err
	}
	if containsAny(strings.ToLower(next.Path), loginPathMarkers) {
		return nil, newError(CodeNeedsLogin, next.Hostname(), reasonLoginRequired)
	}
	return next, nil
}

func statusError(response *http.Response, host string, limits Limits) error {
	switch response.StatusCode {
	case http.StatusUnauthorized:
		return newError(CodeNeedsLogin, host, reasonLoginRequired)
	case http.StatusForbidden:
		return forbiddenError(response, host, limits)
	case http.StatusTooManyRequests, http.StatusServiceUnavailable:
		return newError(CodeBlocked, host, reasonBlocked)
	case http.StatusNotFound, http.StatusGone:
		return newError(CodeNotFound, host, reasonNotFound)
	}
	return newError(CodeFetchFailed, host, reasonUnexpectedStatus+" "+strconv.Itoa(response.StatusCode))
}

func forbiddenError(response *http.Response, host string, limits Limits) error {
	body, err := readLimited(response.Body, limits.MaxBodyBytes, host)
	looksLikeLogin := err == nil && containsAny(strings.ToLower(string(body)), loginPageMarkers)
	if looksLikeLogin {
		return newError(CodeNeedsLogin, host, reasonLoginRequired)
	}
	return newError(CodeBlocked, host, reasonBlocked)
}

// readLimited bounds the decoded body: net/http transparently gunzips when it
// added Accept-Encoding itself, so this also stops decompression bombs.
func readLimited(reader io.Reader, maxBytes int64, host string) ([]byte, error) {
	body, err := io.ReadAll(io.LimitReader(reader, maxBytes+1))
	if err != nil {
		return nil, classify(err, host)
	}
	if int64(len(body)) > maxBytes {
		return nil, newError(CodeTooLarge, host, reasonBodyTooLarge)
	}
	return body, nil
}

func (f *Fetcher) newClient(limits Limits) *http.Client {
	dialer := f.newDialer(limits)
	transport := &http.Transport{
		Proxy: nil,
		DialContext: func(ctx context.Context, network, address string) (net.Conn, error) {
			return f.dialPublic(ctx, dialer, address)
		},
		TLSClientConfig: &tls.Config{
			MinVersion: tls.VersionTLS12,
			RootCAs:    f.rootCertificates,
		},
		TLSHandshakeTimeout:    limits.ConnectTimeout,
		DisableKeepAlives:      true,
		DisableCompression:     false,
		MaxResponseHeaderBytes: limits.MaxHeaderBytes,
	}
	return &http.Client{
		Transport: transport,
		CheckRedirect: func(*http.Request, []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
}

func (f *Fetcher) newDialer(limits Limits) *net.Dialer {
	return &net.Dialer{Timeout: limits.ConnectTimeout, Control: f.checkDialTarget}
}

// dialPublic is the only resolution path: the hostname is resolved once here,
// every answer must be public, and only those vetted IPs are dialled. TLS still
// verifies against the URL hostname because net/http sets ServerName from it.
func (f *Fetcher) dialPublic(ctx context.Context, dialer *net.Dialer, address string) (net.Conn, error) {
	host, port, err := net.SplitHostPort(address)
	if err != nil {
		return nil, newError(CodeInvalidURL, "", reasonMalformedURL)
	}
	candidates, err := f.resolvePublic(ctx, host)
	if err != nil {
		return nil, err
	}
	var lastError error
	for _, candidate := range candidates {
		connection, err := dialer.DialContext(ctx, dialNetwork, net.JoinHostPort(candidate.String(), port))
		if err == nil {
			return connection, nil
		}
		lastError = err
	}
	return nil, classify(lastError, host)
}

func (f *Fetcher) resolvePublic(ctx context.Context, host string) ([]netip.Addr, error) {
	if literal, err := netip.ParseAddr(host); err == nil {
		return f.requirePermitted(host, []netip.Addr{literal})
	}
	addresses, err := f.resolver.LookupNetIP(ctx, resolverNetwork, host)
	if err != nil && isTimeout(err) {
		return nil, newError(CodeTimeout, host, reasonTimedOut)
	}
	if err != nil || len(addresses) == 0 {
		return nil, newError(CodeFetchFailed, host, reasonResolveFailed)
	}
	return f.requirePermitted(host, addresses)
}

func (f *Fetcher) requirePermitted(host string, addresses []netip.Addr) ([]netip.Addr, error) {
	for _, address := range addresses {
		if !f.isPermitted(address) {
			return nil, newError(CodeNotPublic, host, reasonNonPublicResolved)
		}
	}
	return addresses, nil
}

// checkDialTarget runs on the socket just before connect, so it sees the
// address actually being dialled regardless of how it was obtained.
func (f *Fetcher) checkDialTarget(network, address string, _ syscall.RawConn) error {
	target, err := netip.ParseAddrPort(address)
	if err != nil {
		return newError(CodeNotPublic, "", reasonUnparseableDialRoute)
	}
	if !f.isPermitted(target.Addr()) || !f.allowedPorts[int(target.Port())] {
		return newError(CodeNotPublic, target.Addr().String(), reasonDialRejected)
	}
	return nil
}

func (f *Fetcher) isPermitted(address netip.Addr) bool {
	if f.allowLoopback && address.Unmap().IsLoopback() {
		return true
	}
	return isPublicAddress(address)
}

func (f *Fetcher) acquireHost(ctx context.Context, host string) (func(), error) {
	slot := f.hostSlot(host)
	select {
	case slot <- struct{}{}:
		return func() { <-slot }, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

func (f *Fetcher) hostSlot(host string) chan struct{} {
	f.hostSlotsMutex.Lock()
	defer f.hostSlotsMutex.Unlock()
	slot, exists := f.hostSlots[host]
	if !exists {
		slot = make(chan struct{}, perHostConcurrency)
		f.hostSlots[host] = slot
	}
	return slot
}

func isRedirectStatus(status int) bool {
	switch status {
	case http.StatusMovedPermanently, http.StatusFound, http.StatusSeeOther,
		http.StatusTemporaryRedirect, http.StatusPermanentRedirect:
		return true
	}
	return false
}

func isSuccessStatus(status int) bool {
	return status >= http.StatusOK && status < http.StatusMultipleChoices
}

func containsAny(text string, markers []string) bool {
	for _, marker := range markers {
		if strings.Contains(text, marker) {
			return true
		}
	}
	return false
}
