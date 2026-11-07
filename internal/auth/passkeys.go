package auth

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"net/netip"
	"strings"
	"time"

	"github.com/go-webauthn/webauthn/protocol"
	"github.com/go-webauthn/webauthn/webauthn"

	"github.com/aiulian25/keeps/internal/audit"
	"github.com/aiulian25/keeps/internal/config"
	"github.com/aiulian25/keeps/internal/store"
)

const (
	relyingPartyName         = "Keeps"
	ceremonyRegistration     = "registration"
	ceremonyLogin            = "login"
	passkeyNameMaximumLength = 60
	defaultPasskeyName       = "Passkey"
	localhostName            = "localhost"
)

var (
	ErrPasskeysUnavailable = errors.New("passkeys_unavailable")
	ErrPasskeyChallenge    = errors.New("passkey_challenge_expired")
	ErrPasskeyRejected     = errors.New("passkey_rejected")
)

// passkeyRelyingParty is nil when the origin cannot support WebAuthn
// (anything other than HTTPS, or http://localhost in development).
type passkeyRelyingParty struct {
	webauthn *webauthn.WebAuthn
}

func newPasskeyRelyingParty(configuration config.Configuration) (*passkeyRelyingParty, error) {
	origin := configuration.Origin
	_, ipErr := netip.ParseAddr(origin.Hostname())
	isIPAddress := ipErr == nil
	supported := (origin.Scheme == "https" || origin.Hostname() == localhostName) && !isIPAddress
	if !supported {
		// WebAuthn needs a domain name; on plain HTTP or an IP address Keeps
		// runs normally with passkeys turned off.
		return nil, nil
	}
	instance, err := webauthn.New(&webauthn.Config{
		RPID:          origin.Hostname(),
		RPDisplayName: relyingPartyName,
		RPOrigins:     []string{origin.String()},
		AuthenticatorSelection: protocol.AuthenticatorSelection{
			ResidentKey:      protocol.ResidentKeyRequirementRequired,
			UserVerification: protocol.VerificationRequired,
		},
		AttestationPreference: protocol.PreferNoAttestation,
	})
	if err != nil {
		return nil, err
	}
	return &passkeyRelyingParty{webauthn: instance}, nil
}

func (s *Service) PasskeysAvailable() bool {
	return s.passkeys != nil
}

type passkeyUser struct {
	id          int64
	householdID int64
	handle      []byte
	username    string
	displayName string
	credentials []webauthn.Credential
}

func (u *passkeyUser) WebAuthnID() []byte                         { return u.handle }
func (u *passkeyUser) WebAuthnName() string                       { return u.username }
func (u *passkeyUser) WebAuthnDisplayName() string                { return u.displayName }
func (u *passkeyUser) WebAuthnCredentials() []webauthn.Credential { return u.credentials }

func (s *Service) loadPasskeyUser(ctx context.Context, query string, argument any) (*passkeyUser, error) {
	user := &passkeyUser{}
	err := s.store.Database.QueryRowContext(ctx, `SELECT id, household_id, webauthn_handle, username, display_name FROM users WHERE `+query, argument).
		Scan(&user.id, &user.householdID, &user.handle, &user.username, &user.displayName)
	if err != nil {
		return nil, err
	}
	rows, err := s.store.Database.QueryContext(ctx, `SELECT credential_json FROM passkeys WHERE user_id = ?`, user.id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var encoded string
		if err := rows.Scan(&encoded); err != nil {
			return nil, err
		}
		var credential webauthn.Credential
		if err := json.Unmarshal([]byte(encoded), &credential); err != nil {
			return nil, err
		}
		user.credentials = append(user.credentials, credential)
	}
	return user, rows.Err()
}

func (s *Service) storeChallenge(ctx context.Context, userID int64, ceremony string, session *webauthn.SessionData) (string, error) {
	identifier, err := randomToken()
	if err != nil {
		return "", err
	}
	encoded, err := json.Marshal(session)
	if err != nil {
		return "", err
	}
	_, err = s.store.Database.ExecContext(ctx, `INSERT INTO webauthn_challenges (id, user_id, ceremony, session_json, expires_at) VALUES (?, nullif(?, 0), ?, ?, ?)`,
		identifier, userID, ceremony, string(encoded), store.FormatTime(time.Now().Add(config.WebAuthnChallengeLifetime)))
	return identifier, err
}

// takeChallenge loads and deletes a challenge so each one is usable once.
func (s *Service) takeChallenge(ctx context.Context, identifier string, ceremony string, userID int64) (webauthn.SessionData, error) {
	var encoded, expires string
	var storedUser sql.NullInt64
	err := s.store.Database.QueryRowContext(ctx, `DELETE FROM webauthn_challenges WHERE id = ? AND ceremony = ? RETURNING session_json, expires_at, user_id`,
		identifier, ceremony).Scan(&encoded, &expires, &storedUser)
	if err != nil {
		return webauthn.SessionData{}, ErrPasskeyChallenge
	}
	expiry, _ := store.ParseTime(expires)
	wrongUser := userID != 0 && storedUser.Int64 != userID
	if time.Now().After(expiry) || wrongUser {
		return webauthn.SessionData{}, ErrPasskeyChallenge
	}
	var session webauthn.SessionData
	err = json.Unmarshal([]byte(encoded), &session)
	return session, err
}

type CeremonyStart struct {
	ChallengeID string `json:"challengeId"`
	Options     any    `json:"options"`
}

func (s *Service) BeginPasskeyRegistration(ctx context.Context, principal Principal) (CeremonyStart, error) {
	if s.passkeys == nil {
		return CeremonyStart{}, ErrPasskeysUnavailable
	}
	user, err := s.loadPasskeyUser(ctx, "id = ?", principal.UserID)
	if err != nil {
		return CeremonyStart{}, err
	}
	exclusions := make([]protocol.CredentialDescriptor, 0, len(user.credentials))
	for _, credential := range user.credentials {
		exclusions = append(exclusions, credential.Descriptor())
	}
	creation, session, err := s.passkeys.webauthn.BeginRegistration(user, webauthn.WithExclusions(exclusions))
	if err != nil {
		return CeremonyStart{}, err
	}
	identifier, err := s.storeChallenge(ctx, principal.UserID, ceremonyRegistration, session)
	return CeremonyStart{ChallengeID: identifier, Options: creation}, err
}

func (s *Service) FinishPasskeyRegistration(ctx context.Context, principal Principal, challengeID string, name string, credentialJSON []byte) error {
	if s.passkeys == nil {
		return ErrPasskeysUnavailable
	}
	session, err := s.takeChallenge(ctx, challengeID, ceremonyRegistration, principal.UserID)
	if err != nil {
		return err
	}
	user, err := s.loadPasskeyUser(ctx, "id = ?", principal.UserID)
	if err != nil {
		return err
	}
	parsed, err := protocol.ParseCredentialCreationResponseBytes(credentialJSON)
	if err != nil {
		return ErrPasskeyRejected
	}
	credential, err := s.passkeys.webauthn.CreateCredential(user, session, parsed)
	if err != nil {
		return ErrPasskeyRejected
	}
	encoded, err := json.Marshal(credential)
	if err != nil {
		return err
	}
	name = cleanPasskeyName(name)
	return s.store.Transaction(ctx, func(transaction *sql.Tx) error {
		_, err := transaction.ExecContext(ctx, `INSERT INTO passkeys (user_id, credential_id, credential_json, name, created_at) VALUES (?, ?, ?, ?, ?)`,
			principal.UserID, credential.ID, string(encoded), name, store.Now())
		if err != nil {
			return err
		}
		return audit.Record(ctx, transaction, principal.HouseholdID, principal.UserID, audit.PasskeyAdded, name)
	})
}

func cleanPasskeyName(name string) string {
	name = strings.TrimSpace(name)
	if name == "" {
		return defaultPasskeyName
	}
	runes := []rune(name)
	if len(runes) > passkeyNameMaximumLength {
		return string(runes[:passkeyNameMaximumLength])
	}
	return name
}

// BeginPasskeyLogin starts an anonymous passkey sign-in. Starts are counted
// per client like failures, and unanswered challenges are capped, so
// repeated anonymous requests cannot grow the challenge table without bound.
func (s *Service) BeginPasskeyLogin(ctx context.Context, clientAddress string) (CeremonyStart, error) {
	if s.passkeys == nil {
		return CeremonyStart{}, ErrPasskeysUnavailable
	}
	subject := passkeyBeginSubjectPrefix + clientAddress
	if err := s.checkRateLimit(ctx, subject, clientSubject(clientAddress)); err != nil {
		return CeremonyStart{}, err
	}
	if err := s.admitLoginChallenge(ctx); err != nil {
		return CeremonyStart{}, err
	}
	if err := s.recordFailure(ctx, subject); err != nil {
		return CeremonyStart{}, err
	}
	assertion, session, err := s.passkeys.webauthn.BeginDiscoverableLogin(webauthn.WithUserVerification(protocol.VerificationRequired))
	if err != nil {
		return CeremonyStart{}, err
	}
	identifier, err := s.storeChallenge(ctx, 0, ceremonyLogin, session)
	return CeremonyStart{ChallengeID: identifier, Options: assertion}, err
}

// admitLoginChallenge removes expired challenges, then refuses a new
// anonymous one while the cap of unanswered ones is reached.
func (s *Service) admitLoginChallenge(ctx context.Context) error {
	if _, err := s.store.Database.ExecContext(ctx, `DELETE FROM webauthn_challenges WHERE expires_at < ?`, store.Now()); err != nil {
		return err
	}
	var outstanding int
	err := s.store.Database.QueryRowContext(ctx, `SELECT count(*) FROM webauthn_challenges WHERE ceremony = ? AND user_id IS NULL`, ceremonyLogin).Scan(&outstanding)
	if err != nil {
		return err
	}
	if outstanding >= config.PasskeyLoginChallengesMaximum {
		return &ErrRateLimited{RetryAfter: config.WebAuthnChallengeLifetime}
	}
	return nil
}

// FinishPasskeyLogin verifies a discoverable assertion with required user
// verification. A verified passkey satisfies the login policy on its own.
func (s *Service) FinishPasskeyLogin(ctx context.Context, challengeID string, credentialJSON []byte, userAgent string, clientAddress string) (SignInResult, error) {
	if s.passkeys == nil {
		return SignInResult{}, ErrPasskeysUnavailable
	}
	subject := clientSubject(clientAddress)
	if err := s.checkRateLimit(ctx, subject); err != nil {
		return SignInResult{}, err
	}
	session, err := s.takeChallenge(ctx, challengeID, ceremonyLogin, 0)
	if err != nil {
		return SignInResult{}, err
	}
	parsed, err := protocol.ParseCredentialRequestResponseBytes(credentialJSON)
	if err != nil {
		s.recordFailure(ctx, subject)
		return SignInResult{}, ErrPasskeyRejected
	}

	var matchedUser *passkeyUser
	findUser := func(rawID, userHandle []byte) (webauthn.User, error) {
		user, err := s.loadPasskeyUser(ctx, "webauthn_handle = ?", userHandle)
		if err != nil {
			return nil, err
		}
		matchedUser = user
		return user, nil
	}
	credential, err := s.passkeys.webauthn.ValidateDiscoverableLogin(findUser, session, parsed)
	if err != nil || matchedUser == nil {
		s.recordFailure(ctx, subject)
		return SignInResult{}, ErrPasskeyRejected
	}

	encoded, err := json.Marshal(credential)
	if err != nil {
		return SignInResult{}, err
	}
	var result SignInResult
	err = s.store.Transaction(ctx, func(transaction *sql.Tx) error {
		changed, err := transaction.ExecContext(ctx, `UPDATE passkeys SET credential_json = ?, last_used_at = ? WHERE credential_id = ? AND user_id = ?`,
			string(encoded), store.Now(), credential.ID, matchedUser.id)
		if err != nil {
			return err
		}
		if count, _ := changed.RowsAffected(); count != 1 {
			return ErrPasskeyRejected
		}
		token, expires, err := s.createSession(ctx, transaction, matchedUser.id, false, userAgent)
		if err != nil {
			return err
		}
		result = SignInResult{Token: token, Expires: expires}
		return audit.Record(ctx, transaction, matchedUser.householdID, matchedUser.id, audit.SignedIn, "passkey")
	})
	if err == nil {
		s.clearFailures(ctx, passkeyBeginSubjectPrefix+clientAddress)
	}
	return result, err
}

type PasskeySummary struct {
	ID         int64  `json:"id"`
	Name       string `json:"name"`
	CreatedAt  string `json:"createdAt"`
	LastUsedAt string `json:"lastUsedAt,omitempty"`
}

func (s *Service) ListPasskeys(ctx context.Context, userID int64) ([]PasskeySummary, error) {
	rows, err := s.store.Database.QueryContext(ctx, `SELECT id, name, created_at, coalesce(last_used_at, '') FROM passkeys WHERE user_id = ? ORDER BY created_at`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	passkeys := []PasskeySummary{}
	for rows.Next() {
		var summary PasskeySummary
		if err := rows.Scan(&summary.ID, &summary.Name, &summary.CreatedAt, &summary.LastUsedAt); err != nil {
			return nil, err
		}
		passkeys = append(passkeys, summary)
	}
	return passkeys, rows.Err()
}

func (s *Service) RenamePasskey(ctx context.Context, principal Principal, passkeyID int64, name string) error {
	_, err := s.store.Database.ExecContext(ctx, `UPDATE passkeys SET name = ? WHERE id = ? AND user_id = ?`, cleanPasskeyName(name), passkeyID, principal.UserID)
	return err
}

func (s *Service) RemovePasskey(ctx context.Context, principal Principal, passkeyID int64) error {
	return s.store.Transaction(ctx, func(transaction *sql.Tx) error {
		result, err := transaction.ExecContext(ctx, `DELETE FROM passkeys WHERE id = ? AND user_id = ?`, passkeyID, principal.UserID)
		if err != nil {
			return err
		}
		if changed, _ := result.RowsAffected(); changed != 1 {
			return sql.ErrNoRows
		}
		return audit.Record(ctx, transaction, principal.HouseholdID, principal.UserID, audit.PasskeyRemoved, "")
	})
}

// passkeyCredentialBody keeps the raw browser credential for the parser.
type passkeyCredentialBody struct {
	ChallengeID string          `json:"challengeId"`
	Name        string          `json:"name"`
	Credential  json.RawMessage `json:"credential"`
}

func (body passkeyCredentialBody) credentialBytes() []byte {
	return bytes.TrimSpace(body.Credential)
}
