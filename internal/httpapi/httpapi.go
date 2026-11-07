// Package httpapi holds the JSON conventions and middleware shared by every
// API handler: one error shape, bounded request bodies, security headers.
package httpapi

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/netip"
	"runtime/debug"
	"strconv"
	"strings"
	"time"
)

const (
	requestIDBytes         = 8
	forwardedForHeader     = "X-Forwarded-For"
	contentTypeJSON        = "application/json; charset=utf-8"
	internalErrorCode      = "internal_error"
	internalErrorMessage   = "Something went wrong on the server. Please try again."
	contentSecurityPolicy  = "default-src 'self'; img-src 'self' blob: data:; style-src 'self' 'unsafe-inline'; script-src 'self'; connect-src 'self'; font-src 'self'; object-src 'none'; frame-ancestors 'none'; base-uri 'none'; form-action 'self'"
	strictTransportHeader  = "max-age=31536000"
	permissionsPolicyValue = "camera=(self), microphone=(), geolocation=(), payment=(), usb=()"
)

// Error is the single error shape returned by the API.
type Error struct {
	Status  int               `json:"-"`
	Code    string            `json:"code"`
	Message string            `json:"message"`
	Fields  map[string]string `json:"fields,omitempty"`
	Details any               `json:"details,omitempty"`
}

func (e *Error) Error() string {
	return e.Code + ": " + e.Message
}

func NewError(status int, code string, message string) *Error {
	return &Error{Status: status, Code: code, Message: message}
}

func FieldError(field string, code string, message string) *Error {
	return &Error{Status: http.StatusBadRequest, Code: code, Message: message, Fields: map[string]string{field: message}}
}

var (
	ErrNotFound        = NewError(http.StatusNotFound, "not_found", "That could not be found.")
	ErrUnauthenticated = NewError(http.StatusUnauthorized, "unauthenticated", "Please sign in.")
	ErrForbidden       = NewError(http.StatusForbidden, "forbidden", "You do not have access to that.")
	ErrBadOrigin       = NewError(http.StatusForbidden, "bad_origin", "This request came from an unexpected site.")
	ErrBadCSRF         = NewError(http.StatusForbidden, "bad_csrf", "Your session token is missing or out of date. Reload the page.")
	ErrReauthenticate  = NewError(http.StatusForbidden, "reauthentication_required", "Confirm your password to continue.")
	ErrBodyTooLarge    = NewError(http.StatusRequestEntityTooLarge, "too_large", "The request is too large.")
	ErrInvalidJSON     = NewError(http.StatusBadRequest, "invalid_request", "The request could not be read.")
)

func WriteJSON(writer http.ResponseWriter, status int, value any) {
	writer.Header().Set("Content-Type", contentTypeJSON)
	writer.Header().Set("Cache-Control", "no-store")
	writer.WriteHeader(status)
	if value == nil {
		return
	}
	json.NewEncoder(writer).Encode(value)
}

// WriteError writes err as JSON; unknown errors are logged and hidden.
func WriteError(writer http.ResponseWriter, request *http.Request, err error) {
	var apiError *Error
	if errors.As(err, &apiError) {
		WriteJSON(writer, apiError.Status, apiError)
		return
	}
	var tooLarge *http.MaxBytesError
	if errors.As(err, &tooLarge) {
		WriteJSON(writer, ErrBodyTooLarge.Status, ErrBodyTooLarge)
		return
	}
	slog.Error("request failed", "request_id", RequestID(request.Context()), "path", request.URL.Path, "error", err)
	WriteJSON(writer, http.StatusInternalServerError, NewError(http.StatusInternalServerError, internalErrorCode, internalErrorMessage))
}

// DecodeJSON reads a bounded JSON body and rejects unknown fields, so a client
// can never set fields (owner, role) that a handler does not explicitly read.
func DecodeJSON(writer http.ResponseWriter, request *http.Request, limit int64, destination any) error {
	request.Body = http.MaxBytesReader(writer, request.Body, limit)
	decoder := json.NewDecoder(request.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(destination); err != nil {
		var tooLarge *http.MaxBytesError
		if errors.As(err, &tooLarge) {
			return ErrBodyTooLarge
		}
		return &Error{Status: http.StatusBadRequest, Code: ErrInvalidJSON.Code, Message: ErrInvalidJSON.Message}
	}
	if _, err := decoder.Token(); !errors.Is(err, io.EOF) {
		return ErrInvalidJSON
	}
	return nil
}

// Handler adapts handlers that return errors.
type Handler func(http.ResponseWriter, *http.Request) error

func (handler Handler) ServeHTTP(writer http.ResponseWriter, request *http.Request) {
	if err := handler(writer, request); err != nil {
		WriteError(writer, request, err)
	}
}

type requestIDKey struct{}

func RequestID(ctx context.Context) string {
	value, _ := ctx.Value(requestIDKey{}).(string)
	return value
}

// Base wraps every request: request ID, panic recovery, security headers and
// a log line that never includes the query string.
func Base(secureOrigin bool, next http.Handler) http.Handler {
	return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		started := time.Now()
		identifier := newRequestID()
		request = request.WithContext(context.WithValue(request.Context(), requestIDKey{}, identifier))

		headers := writer.Header()
		headers.Set("Content-Security-Policy", contentSecurityPolicy)
		headers.Set("X-Content-Type-Options", "nosniff")
		headers.Set("Referrer-Policy", "same-origin")
		headers.Set("X-Frame-Options", "DENY")
		headers.Set("Cross-Origin-Opener-Policy", "same-origin")
		headers.Set("Cross-Origin-Resource-Policy", "same-origin")
		headers.Set("Permissions-Policy", permissionsPolicyValue)
		if secureOrigin {
			headers.Set("Strict-Transport-Security", strictTransportHeader)
		}

		recorder := &statusRecorder{ResponseWriter: writer, status: http.StatusOK}
		defer func() {
			if recovered := recover(); recovered != nil {
				slog.Error("panic", "request_id", identifier, "path", request.URL.Path, "panic", fmt.Sprint(recovered), "stack", string(debug.Stack()))
				WriteJSON(recorder, http.StatusInternalServerError, NewError(http.StatusInternalServerError, internalErrorCode, internalErrorMessage))
			}
			slog.Info("request", "request_id", identifier, "method", request.Method, "path", request.URL.Path, "status", recorder.status, "duration_ms", time.Since(started).Milliseconds())
		}()
		next.ServeHTTP(recorder, request)
	})
}

type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (r *statusRecorder) WriteHeader(status int) {
	r.status = status
	r.ResponseWriter.WriteHeader(status)
}

func (r *statusRecorder) Flush() {
	if flusher, ok := r.ResponseWriter.(http.Flusher); ok {
		flusher.Flush()
	}
}

func (r *statusRecorder) Unwrap() http.ResponseWriter {
	return r.ResponseWriter
}

func newRequestID() string {
	random := make([]byte, requestIDBytes)
	rand.Read(random)
	return hex.EncodeToString(random)
}

// ClientAddress returns the peer address, or the last X-Forwarded-For hop
// when the peer is a configured trusted proxy. Arbitrary senders cannot spoof it.
func ClientAddress(request *http.Request, trustedProxies []netip.Prefix) string {
	host, _, err := net.SplitHostPort(request.RemoteAddr)
	if err != nil {
		host = request.RemoteAddr
	}
	peer, err := netip.ParseAddr(host)
	if err != nil {
		return host
	}
	if !isTrusted(peer.Unmap(), trustedProxies) {
		return peer.Unmap().String()
	}
	forwarded := request.Header.Values(forwardedForHeader)
	if len(forwarded) == 0 {
		return peer.Unmap().String()
	}
	hops := strings.Split(forwarded[len(forwarded)-1], ",")
	lastHop := strings.TrimSpace(hops[len(hops)-1])
	if forwardedAddress, err := netip.ParseAddr(lastHop); err == nil {
		return forwardedAddress.Unmap().String()
	}
	return peer.Unmap().String()
}

func isTrusted(address netip.Addr, trustedProxies []netip.Prefix) bool {
	for _, prefix := range trustedProxies {
		if prefix.Contains(address) {
			return true
		}
	}
	return false
}

// PathID parses the {id} path segment.
func PathID(request *http.Request) (int64, error) {
	return parseID(request.PathValue("id"))
}

// QueryID parses an optional numeric query parameter (0 when absent).
func QueryID(request *http.Request, name string) (int64, error) {
	value := request.URL.Query().Get(name)
	if value == "" {
		return 0, nil
	}
	return parseID(value)
}

func parseID(value string) (int64, error) {
	identifier, err := strconv.ParseInt(value, 10, 64)
	if err != nil || identifier <= 0 {
		return 0, ErrNotFound
	}
	return identifier, nil
}
