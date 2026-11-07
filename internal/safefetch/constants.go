package safefetch

import "time"

// UserAgent identifies Keeps honestly to retailers without exposing the owner.
const UserAgent = "Keeps/1.0 (+self-hosted inventory; link import)"

const (
	httpsScheme      = "https"
	defaultHTTPSPort = 443
	acceptLanguage   = "en-GB,en;q=0.8"
	resolverNetwork  = "ip"
	dialNetwork      = "tcp"
	localhostName    = "localhost"
	localhostSuffix  = ".localhost"
	hexNumberPrefix  = "0x"
	labelSeparator   = "."
	hostDetailJoiner = ": "
)

const (
	headerUserAgent      = "User-Agent"
	headerAccept         = "Accept"
	headerAcceptLanguage = "Accept-Language"
	headerContentType    = "Content-Type"
	headerLocation       = "Location"
)

const (
	pageTotalTimeout      = 20 * time.Second
	imageTotalTimeout     = 20 * time.Second
	connectTimeout        = 10 * time.Second
	pageMaxBodyBytes      = 5 << 20
	imageMaxBodyBytes     = 10 << 20
	defaultMaxRedirects   = 3
	defaultMaxHeaderBytes = 64 << 10
	perHostConcurrency    = 1
	maxHostnameLength     = 253
	maxLabelLength        = 63
)

const (
	reasonMalformedURL         = "malformed URL"
	reasonSchemeNotHTTPS       = "only https URLs are supported"
	reasonUserinfo             = "credentials in URLs are not allowed"
	reasonEmptyHost            = "missing host"
	reasonInvalidHost          = "host is not a valid DNS name"
	reasonNonASCIIHost         = "internationalised host names must be entered in punycode (xn--) form"
	reasonNumericHost          = "numeric host forms are not allowed"
	reasonZonedAddress         = "scoped IPv6 addresses are not allowed"
	reasonInvalidPort          = "invalid port"
	reasonPortNotAllowed       = "only port 443 is allowed"
	reasonNonPublicHost        = "host is not a public internet address"
	reasonNonPublicResolved    = "host resolves to a non-public address"
	reasonDialRejected         = "connection to a non-public address refused"
	reasonResolveFailed        = "host could not be resolved"
	reasonPrivateDocument      = "signed private document links cannot be fetched; upload the file instead"
	reasonTooManyRedirects     = "too many redirects"
	reasonRedirectNoLocation   = "redirect without a location"
	reasonLoginRequired        = "the page requires signing in"
	reasonBlocked              = "the site refused the request"
	reasonNotFound             = "the page was not found"
	reasonUnexpectedStatus     = "unexpected response status"
	reasonBodyTooLarge         = "response body exceeds the size limit"
	reasonTimedOut             = "the request timed out"
	reasonTransportFailure     = "the request failed"
	reasonUnparseableDialRoute = "unparseable dial address"
)

// loginPathMarkers flag redirects that land on a sign-in wall.
var loginPathMarkers = []string{"login", "signin", "account"}

// loginPageMarkers flag 403 bodies that are really sign-in forms.
var loginPageMarkers = []string{`type="password"`, `name="password"`, "sign in", "log in"}
