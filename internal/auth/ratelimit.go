package auth

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"time"

	"github.com/aiulian25/keeps/internal/config"
	"github.com/aiulian25/keeps/internal/store"
)

const (
	userSubjectPrefix   = "user:"
	clientSubjectPrefix = "client:"
	setupSubjectPrefix  = "setup:"
	codeSubjectPrefix   = "code:"
	// passkeyBeginSubjectPrefix counts passkey sign-in starts per client;
	// a completed passkey sign-in clears it.
	passkeyBeginSubjectPrefix = "passkey-begin:"
)

// ErrRateLimited carries how long the caller must wait.
type ErrRateLimited struct {
	RetryAfter time.Duration
}

func (e *ErrRateLimited) Error() string {
	return "too many attempts"
}

func userSubject(username string) string {
	// Trimmed like the account lookup, so every accepted spelling of one
	// username shares the same counter.
	return userSubjectPrefix + strings.ToLower(strings.TrimSpace(username))
}

func clientSubject(address string) string {
	return clientSubjectPrefix + address
}

// checkRateLimit returns ErrRateLimited when any subject is in a delay
// period. Delays grow progressively but always expire, so an attacker cannot
// lock the owner out permanently.
func (s *Service) checkRateLimit(ctx context.Context, subjects ...string) error {
	now := time.Now()
	for _, subject := range subjects {
		var blockedUntil string
		err := s.store.Database.QueryRowContext(ctx, `SELECT blocked_until FROM login_failures WHERE subject = ?`, subject).Scan(&blockedUntil)
		if errors.Is(err, sql.ErrNoRows) {
			continue
		}
		if err != nil {
			return err
		}
		until, _ := store.ParseTime(blockedUntil)
		if now.Before(until) {
			return &ErrRateLimited{RetryAfter: until.Sub(now).Round(time.Second)}
		}
	}
	return nil
}

func (s *Service) recordFailure(ctx context.Context, subjects ...string) error {
	for _, subject := range subjects {
		var failures int
		err := s.store.Database.QueryRowContext(ctx, `SELECT failures FROM login_failures WHERE subject = ?`, subject).Scan(&failures)
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			return err
		}
		failures++
		blockedUntil := time.Now().Add(delayAfter(failures))
		_, err = s.store.Database.ExecContext(ctx, `INSERT INTO login_failures (subject, failures, blocked_until) VALUES (?, ?, ?)
			ON CONFLICT(subject) DO UPDATE SET failures = excluded.failures, blocked_until = excluded.blocked_until`,
			subject, failures, store.FormatTime(blockedUntil))
		if err != nil {
			return err
		}
	}
	return nil
}

func (s *Service) clearFailures(ctx context.Context, subjects ...string) {
	for _, subject := range subjects {
		s.store.Database.ExecContext(ctx, `DELETE FROM login_failures WHERE subject = ?`, subject)
	}
}

func delayAfter(failures int) time.Duration {
	excess := failures - config.LoginFailuresBeforeDelay
	if excess < 0 {
		return 0
	}
	delay := config.LoginDelayBase
	for step := 0; step < excess && delay < config.LoginDelayMaximum; step++ {
		delay *= 2
	}
	return min(delay, config.LoginDelayMaximum)
}
