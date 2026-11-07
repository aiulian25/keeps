// Package auth implements the protected bootstrap, password sign-in, server
// side sessions, CSRF/origin checks, rate limits and optional second factors.
package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base32"
	"net/http"
	"sync"

	"github.com/aiulian25/keeps/internal/config"
	"github.com/aiulian25/keeps/internal/secrets"
	"github.com/aiulian25/keeps/internal/store"
)

type Service struct {
	store         *store.Store
	configuration config.Configuration
	keys          *secrets.Keys
	hashSlots     chan struct{}
	dummyOnce     sync.Once
	dummyHash     string
	passkeys      *passkeyRelyingParty
}

func NewService(database *store.Store, configuration config.Configuration, keys *secrets.Keys) (*Service, error) {
	service := &Service{
		store:         database,
		configuration: configuration,
		keys:          keys,
		hashSlots:     make(chan struct{}, config.ConcurrentPasswordHashes),
	}
	relyingParty, err := newPasskeyRelyingParty(configuration)
	if err != nil {
		return nil, err
	}
	service.passkeys = relyingParty
	return service, nil
}

// Principal is the signed-in user attached to a request.
type Principal struct {
	UserID            int64
	HouseholdID       int64
	SessionID         int64
	Username          string
	CSRFToken         string
	ReauthenticatedAt string
}

type principalKey struct{}

func WithPrincipal(ctx context.Context, principal Principal) context.Context {
	return context.WithValue(ctx, principalKey{}, principal)
}

// PrincipalFrom returns the authenticated principal; handlers behind
// RequireSession can rely on it being present.
func PrincipalFrom(ctx context.Context) Principal {
	principal, _ := ctx.Value(principalKey{}).(Principal)
	return principal
}

func PrincipalFromRequest(request *http.Request) Principal {
	return PrincipalFrom(request.Context())
}

var tokenEncoding = base32.StdEncoding.WithPadding(base32.NoPadding)

const (
	tokenBytes          = 32
	webauthnHandleBytes = 32
)

func randomBytes(count int) ([]byte, error) {
	random := make([]byte, count)
	_, err := rand.Read(random)
	return random, err
}

func randomToken() (string, error) {
	random, err := randomBytes(tokenBytes)
	if err != nil {
		return "", err
	}
	return tokenEncoding.EncodeToString(random), nil
}

func tokenVerifier(token string) []byte {
	sum := sha256.Sum256([]byte(token))
	return sum[:]
}
