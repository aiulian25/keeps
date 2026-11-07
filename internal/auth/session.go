package auth

import (
	"context"
	"database/sql"
	"errors"
	"net/http"
	"time"

	"github.com/aiulian25/keeps/internal/audit"
	"github.com/aiulian25/keeps/internal/config"
	"github.com/aiulian25/keeps/internal/store"
)

const (
	secureCookieName      = "__Host-keeps_session"
	developmentCookieName = "keeps_session"
	lastSeenWriteInterval = time.Minute
	maximumUserAgentBytes = 200
)

var errNoSession = errors.New("no valid session")

type sessionRecord struct {
	ID                   int64
	UserID               int64
	HouseholdID          int64
	Username             string
	CSRFToken            string
	AwaitingSecondFactor bool
	LastSeen             time.Time
	Expires              time.Time
	ReauthenticatedAt    string
}

func (s *Service) cookieName() string {
	if s.configuration.SecureCookies() {
		return secureCookieName
	}
	return developmentCookieName
}

// createSession stores a new session and returns its bearer token. Only the
// token's SHA-256 verifier is stored.
func (s *Service) createSession(ctx context.Context, executor audit.Executor, userID int64, awaitingSecondFactor bool, userAgent string) (string, time.Time, error) {
	token, err := randomToken()
	if err != nil {
		return "", time.Time{}, err
	}
	csrfToken, err := randomToken()
	if err != nil {
		return "", time.Time{}, err
	}
	now := time.Now()
	expires := now.Add(config.SessionAbsoluteLifetime)
	if awaitingSecondFactor {
		expires = now.Add(config.SecondFactorPendingTimeout)
	}
	if len(userAgent) > maximumUserAgentBytes {
		userAgent = userAgent[:maximumUserAgentBytes]
	}
	reauthenticated := sql.NullString{String: store.FormatTime(now), Valid: !awaitingSecondFactor}
	_, err = executor.ExecContext(ctx, `INSERT INTO sessions
		(user_id, token_hash, csrf_token, awaiting_second_factor, user_agent, created_at, last_seen_at, expires_at, reauthenticated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		userID, tokenVerifier(token), csrfToken, awaitingSecondFactor, userAgent,
		store.FormatTime(now), store.FormatTime(now), store.FormatTime(expires), reauthenticated)
	return token, expires, err
}

func (s *Service) lookupSession(ctx context.Context, token string) (sessionRecord, error) {
	var record sessionRecord
	var lastSeen, expires string
	var reauthenticated sql.NullString
	err := s.store.Database.QueryRowContext(ctx, `SELECT s.id, s.user_id, u.household_id, u.username, s.csrf_token,
			s.awaiting_second_factor, s.last_seen_at, s.expires_at, s.reauthenticated_at
		FROM sessions s JOIN users u ON u.id = s.user_id WHERE s.token_hash = ?`, tokenVerifier(token)).
		Scan(&record.ID, &record.UserID, &record.HouseholdID, &record.Username, &record.CSRFToken,
			&record.AwaitingSecondFactor, &lastSeen, &expires, &reauthenticated)
	if errors.Is(err, sql.ErrNoRows) {
		return sessionRecord{}, errNoSession
	}
	if err != nil {
		return sessionRecord{}, err
	}
	record.LastSeen, _ = store.ParseTime(lastSeen)
	record.Expires, _ = store.ParseTime(expires)
	record.ReauthenticatedAt = reauthenticated.String

	now := time.Now()
	idleExpired := now.Sub(record.LastSeen) > config.SessionIdleTimeout
	if now.After(record.Expires) || idleExpired {
		s.store.Database.ExecContext(ctx, `DELETE FROM sessions WHERE id = ?`, record.ID)
		return sessionRecord{}, errNoSession
	}
	if now.Sub(record.LastSeen) > lastSeenWriteInterval {
		s.store.Database.ExecContext(ctx, `UPDATE sessions SET last_seen_at = ? WHERE id = ?`, store.FormatTime(now), record.ID)
	}
	return record, nil
}

func (s *Service) sessionFromRequest(request *http.Request) (sessionRecord, error) {
	cookie, err := request.Cookie(s.cookieName())
	if err != nil || cookie.Value == "" {
		return sessionRecord{}, errNoSession
	}
	return s.lookupSession(request.Context(), cookie.Value)
}

func (s *Service) setSessionCookie(writer http.ResponseWriter, token string, expires time.Time) {
	http.SetCookie(writer, &http.Cookie{
		Name:     s.cookieName(),
		Value:    token,
		Path:     "/",
		Expires:  expires,
		HttpOnly: true,
		Secure:   s.configuration.SecureCookies(),
		SameSite: http.SameSiteLaxMode,
	})
}

// ClearAuthCookie removes the session cookie (after a restore signs everyone out).
func (s *Service) ClearAuthCookie(writer http.ResponseWriter) {
	s.clearSessionCookie(writer)
}

func (s *Service) clearSessionCookie(writer http.ResponseWriter) {
	http.SetCookie(writer, &http.Cookie{
		Name:     s.cookieName(),
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   s.configuration.SecureCookies(),
		SameSite: http.SameSiteLaxMode,
	})
}

// revokeSessions deletes every session of the user except keepSessionID
// (zero revokes all).
func revokeSessions(ctx context.Context, executor audit.Executor, userID int64, keepSessionID int64) error {
	_, err := executor.ExecContext(ctx, `DELETE FROM sessions WHERE user_id = ? AND id != ?`, userID, keepSessionID)
	return err
}

func (s *Service) markReauthenticated(ctx context.Context, sessionID int64) error {
	_, err := s.store.Database.ExecContext(ctx, `UPDATE sessions SET reauthenticated_at = ? WHERE id = ?`, store.Now(), sessionID)
	return err
}

// RecentlyReauthenticated reports whether the principal confirmed their
// identity within the reauthentication window.
func RecentlyReauthenticated(principal Principal) bool {
	if principal.ReauthenticatedAt == "" {
		return false
	}
	moment, err := store.ParseTime(principal.ReauthenticatedAt)
	if err != nil {
		return false
	}
	return time.Since(moment) <= config.ReauthenticationWindow
}

// PurgeExpiredSessions removes expired rows; called periodically.
func (s *Service) PurgeExpiredSessions(ctx context.Context) error {
	now := time.Now()
	_, err := s.store.Database.ExecContext(ctx, `DELETE FROM sessions WHERE expires_at < ? OR last_seen_at < ?`,
		store.FormatTime(now), store.FormatTime(now.Add(-config.SessionIdleTimeout)))
	if err != nil {
		return err
	}
	_, err = s.store.Database.ExecContext(ctx, `DELETE FROM webauthn_challenges WHERE expires_at < ?`, store.FormatTime(now))
	return err
}
