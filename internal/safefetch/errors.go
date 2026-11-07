package safefetch

import (
	"context"
	"errors"
	"net"
)

type ErrorCode string

const (
	CodeInvalidURL          ErrorCode = "invalid_url"
	CodeNotPublic           ErrorCode = "not_public"
	CodeNeedsLogin          ErrorCode = "needs_login"
	CodeBlocked             ErrorCode = "blocked"
	CodeTimeout             ErrorCode = "timeout"
	CodeTooLarge            ErrorCode = "too_large"
	CodeNotFound            ErrorCode = "not_found"
	CodePrivateDocumentLink ErrorCode = "private_document_link"
	CodeFetchFailed         ErrorCode = "fetch_failed"
)

// Error is the only error type Get returns. Detail names at most the host,
// never a path or query string, so it is safe to log and show.
type Error struct {
	Code   ErrorCode
	Detail string
}

func (e *Error) Error() string {
	return string(e.Code) + hostDetailJoiner + e.Detail
}

func newError(code ErrorCode, host string, reason string) *Error {
	if host == "" {
		return &Error{Code: code, Detail: reason}
	}
	return &Error{Code: code, Detail: host + hostDetailJoiner + reason}
}

// classify discards the underlying error text because net/http embeds the
// full request URL (including its query) in it.
func classify(err error, host string) *Error {
	var fetchError *Error
	if errors.As(err, &fetchError) {
		return fetchError
	}
	if isTimeout(err) {
		return newError(CodeTimeout, host, reasonTimedOut)
	}
	return newError(CodeFetchFailed, host, reasonTransportFailure)
}

func isTimeout(err error) bool {
	if errors.Is(err, context.DeadlineExceeded) {
		return true
	}
	var networkError net.Error
	return errors.As(err, &networkError) && networkError.Timeout()
}
