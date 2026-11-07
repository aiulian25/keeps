package auth

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"
	"unicode/utf8"

	"golang.org/x/crypto/argon2"

	"github.com/aiulian25/keeps/internal/config"
)

const (
	passwordSaltBytes = 16
	passwordHashBytes = 32
	argonVersionField = "v=19"
	argonIdentifier   = "argon2id"
	encodedFieldCount = 6
)

var (
	ErrPasswordTooShort = errors.New("password_too_short")
	ErrPasswordTooLong  = errors.New("password_too_long")
	errMalformedHash    = errors.New("stored password hash is malformed")
)

type passwordParameters struct {
	memoryKiB   uint32
	iterations  uint32
	parallelism uint8
}

var currentParameters = passwordParameters{
	memoryKiB:   config.PasswordHashMemoryKiB,
	iterations:  config.PasswordHashIterations,
	parallelism: config.PasswordHashParallelism,
}

// ValidateNewPassword enforces length only: long passphrases, paste allowed,
// no composition rules and never silent truncation.
func ValidateNewPassword(password string) error {
	length := utf8.RuneCountInString(password)
	if length < config.PasswordMinimumLength {
		return ErrPasswordTooShort
	}
	if len(password) > config.PasswordMaximumLength {
		return ErrPasswordTooLong
	}
	return nil
}

// HashPassword returns a PHC-style Argon2id string. Concurrency is bounded by
// the service so a burst of logins cannot exhaust memory.
func (s *Service) HashPassword(ctx context.Context, password string) (string, error) {
	if err := s.acquireHashSlot(ctx); err != nil {
		return "", err
	}
	defer s.releaseHashSlot()

	salt := make([]byte, passwordSaltBytes)
	if _, err := rand.Read(salt); err != nil {
		return "", err
	}
	parameters := currentParameters
	hash := argon2.IDKey([]byte(password), salt, parameters.iterations, parameters.memoryKiB, parameters.parallelism, passwordHashBytes)
	return fmt.Sprintf("$%s$%s$m=%d,t=%d,p=%d$%s$%s", argonIdentifier, argonVersionField,
		parameters.memoryKiB, parameters.iterations, parameters.parallelism,
		base64.RawStdEncoding.EncodeToString(salt), base64.RawStdEncoding.EncodeToString(hash)), nil
}

func (s *Service) VerifyPassword(ctx context.Context, password string, encoded string) (bool, error) {
	if len(password) > config.PasswordMaximumLength {
		return false, nil
	}
	parameters, salt, expected, err := decodeHash(encoded)
	if err != nil {
		return false, err
	}
	if err := s.acquireHashSlot(ctx); err != nil {
		return false, err
	}
	defer s.releaseHashSlot()
	actual := argon2.IDKey([]byte(password), salt, parameters.iterations, parameters.memoryKiB, parameters.parallelism, uint32(len(expected)))
	return subtle.ConstantTimeCompare(actual, expected) == 1, nil
}

// dummyHash is verified when a username does not exist, so response timing
// does not reveal which usernames are valid.
func (s *Service) verifyAgainstDummy(ctx context.Context, password string) {
	s.dummyOnce.Do(func() {
		s.dummyHash, _ = s.HashPassword(ctx, "keeps-dummy-password-for-timing")
	})
	s.VerifyPassword(ctx, password, s.dummyHash)
}

func decodeHash(encoded string) (passwordParameters, []byte, []byte, error) {
	fields := strings.Split(encoded, "$")
	if len(fields) != encodedFieldCount || fields[1] != argonIdentifier || fields[2] != argonVersionField {
		return passwordParameters{}, nil, nil, errMalformedHash
	}
	var parameters passwordParameters
	if _, err := fmt.Sscanf(fields[3], "m=%d,t=%d,p=%d", &parameters.memoryKiB, &parameters.iterations, &parameters.parallelism); err != nil {
		return passwordParameters{}, nil, nil, errMalformedHash
	}
	salt, err := base64.RawStdEncoding.DecodeString(fields[4])
	if err != nil {
		return passwordParameters{}, nil, nil, errMalformedHash
	}
	hash, err := base64.RawStdEncoding.DecodeString(fields[5])
	if err != nil {
		return passwordParameters{}, nil, nil, errMalformedHash
	}
	return parameters, salt, hash, nil
}

func (s *Service) acquireHashSlot(ctx context.Context) error {
	select {
	case s.hashSlots <- struct{}{}:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (s *Service) releaseHashSlot() {
	<-s.hashSlots
}
