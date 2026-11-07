package auth

import (
	"crypto/subtle"
	"net/http"
	"strings"

	"github.com/aiulian25/keeps/internal/httpapi"
)

const (
	csrfHeader          = "X-CSRF-Token"
	originHeader        = "Origin"
	fetchSiteHeader     = "Sec-Fetch-Site"
	fetchSiteSameOrigin = "same-origin"
	// opaqueOrigin is sent by some browsers for form posts; Sec-Fetch-Site then decides.
	opaqueOrigin = "null"
)

const (
	csrfFormField    = "csrfToken"
	formContentType  = "application/x-www-form-urlencoded"
	maximumFormBytes = 64 << 10
)

// submittedCSRF reads the token from the header, or from a small urlencoded
// form body for native form posts that stream large downloads to disk.
func submittedCSRF(request *http.Request) string {
	if token := request.Header.Get(csrfHeader); token != "" {
		return token
	}
	if !strings.HasPrefix(request.Header.Get("Content-Type"), formContentType) {
		return ""
	}
	request.Body = http.MaxBytesReader(nil, request.Body, maximumFormBytes)
	return request.PostFormValue(csrfFormField)
}

func isSafeMethod(method string) bool {
	return method == http.MethodGet || method == http.MethodHead || method == http.MethodOptions
}

// originAllowed requires an exact Origin match with APP_ORIGIN for unsafe
// methods. Browsers without an Origin header must report same-origin fetch.
func (s *Service) originAllowed(request *http.Request) bool {
	if isSafeMethod(request.Method) {
		return true
	}
	origin := request.Header.Get(originHeader)
	sameOriginFetch := request.Header.Get(fetchSiteHeader) == fetchSiteSameOrigin
	if origin == "" || origin == opaqueOrigin {
		return sameOriginFetch
	}
	return origin == s.configuration.Origin.String()
}

// Public protects unauthenticated endpoints (sign-in, bootstrap) with the
// origin check only.
func (s *Service) Public(next http.Handler) http.Handler {
	return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if !s.originAllowed(request) {
			httpapi.WriteError(writer, request, httpapi.ErrBadOrigin)
			return
		}
		next.ServeHTTP(writer, request)
	})
}

// Protect requires the origin check, a fully signed-in session, and the
// session's CSRF token on every unsafe request.
func (s *Service) Protect(next http.Handler) http.Handler {
	return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if !s.originAllowed(request) {
			httpapi.WriteError(writer, request, httpapi.ErrBadOrigin)
			return
		}
		session, err := s.sessionFromRequest(request)
		if err != nil || session.AwaitingSecondFactor {
			httpapi.WriteError(writer, request, httpapi.ErrUnauthenticated)
			return
		}
		csrfMissing := subtle.ConstantTimeCompare([]byte(submittedCSRF(request)), []byte(session.CSRFToken)) != 1
		if !isSafeMethod(request.Method) && csrfMissing {
			httpapi.WriteError(writer, request, httpapi.ErrBadCSRF)
			return
		}
		principal := Principal{
			UserID:            session.UserID,
			HouseholdID:       session.HouseholdID,
			SessionID:         session.ID,
			Username:          session.Username,
			CSRFToken:         session.CSRFToken,
			ReauthenticatedAt: session.ReauthenticatedAt,
		}
		next.ServeHTTP(writer, request.WithContext(WithPrincipal(request.Context(), principal)))
	})
}

// RequireRecentReauthentication wraps a handler that changes security
// settings, exports or restores data.
func RequireRecentReauthentication(next httpapi.Handler) httpapi.Handler {
	return func(writer http.ResponseWriter, request *http.Request) error {
		if !RecentlyReauthenticated(PrincipalFromRequest(request)) {
			return httpapi.ErrReauthenticate
		}
		return next(writer, request)
	}
}
