// Package blob stores private files under opaque random keys. Files are
// written to a temporary file, synced, then atomically renamed into place, so
// a crash never leaves a partial file under a final key.
package blob

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"syscall"
)

const (
	keyRandomBytes   = 16
	shardPrefixBytes = 2
	directoryMode    = 0o700
	fileMode         = 0o600
	temporaryPattern = "upload-*"
)

var (
	ErrTooLarge    = errors.New("file is larger than the allowed size")
	ErrStorageFull = errors.New("not enough storage space")
	ErrInvalidKey  = errors.New("invalid storage key")
)

type Store struct {
	root      string
	temporary string
}

type Written struct {
	Key    string
	Size   int64
	SHA256 string
}

func New(root string, temporary string) (*Store, error) {
	for _, directory := range []string{root, temporary} {
		if err := os.MkdirAll(directory, directoryMode); err != nil {
			return nil, err
		}
	}
	return &Store{root: root, temporary: temporary}, nil
}

func (s *Store) Root() string {
	return s.root
}

// TemporaryFile creates a private file in the temporary area (on the data
// volume, not the small tmpfs).
func (s *Store) TemporaryFile() (*os.File, error) {
	return os.CreateTemp(s.temporary, temporaryPattern)
}

// Write streams source into a new blob, failing with ErrTooLarge when more
// than limit bytes arrive.
func (s *Store) Write(source io.Reader, limit int64) (Written, error) {
	temporary, err := s.TemporaryFile()
	if err != nil {
		return Written{}, classify(err)
	}
	temporaryPath := temporary.Name()
	committed := false
	defer func() {
		if !committed {
			os.Remove(temporaryPath)
		}
	}()

	hash := sha256.New()
	size, err := io.Copy(io.MultiWriter(temporary, hash), io.LimitReader(source, limit+1))
	if err != nil {
		temporary.Close()
		return Written{}, classify(err)
	}
	if size > limit {
		temporary.Close()
		return Written{}, ErrTooLarge
	}
	if err := temporary.Sync(); err != nil {
		temporary.Close()
		return Written{}, classify(err)
	}
	if err := temporary.Close(); err != nil {
		return Written{}, classify(err)
	}

	key, err := newKey()
	if err != nil {
		return Written{}, err
	}
	if err := s.moveInto(temporaryPath, key); err != nil {
		return Written{}, err
	}
	committed = true
	return Written{Key: key, Size: size, SHA256: hex.EncodeToString(hash.Sum(nil))}, nil
}

func (s *Store) moveInto(temporaryPath string, key string) error {
	finalPath, err := s.Path(key)
	if err != nil {
		return err
	}
	directory := filepath.Dir(finalPath)
	if err := os.MkdirAll(directory, directoryMode); err != nil {
		return classify(err)
	}
	if err := os.Chmod(temporaryPath, fileMode); err != nil {
		return classify(err)
	}
	if err := os.Rename(temporaryPath, finalPath); err != nil {
		return classify(err)
	}
	return syncDirectory(directory)
}

// Path maps a key to its file, rejecting anything that is not a key this
// package generated.
func (s *Store) Path(key string) (string, error) {
	validLength := len(key) == keyRandomBytes*2
	if !validLength || strings.Trim(key, "0123456789abcdef") != "" {
		return "", ErrInvalidKey
	}
	return filepath.Join(s.root, key[:shardPrefixBytes], key), nil
}

func (s *Store) Open(key string) (*os.File, error) {
	path, err := s.Path(key)
	if err != nil {
		return nil, err
	}
	return os.Open(path)
}

func (s *Store) Remove(key string) error {
	path, err := s.Path(key)
	if err != nil {
		return err
	}
	err = os.Remove(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	return err
}

func newKey() (string, error) {
	random := make([]byte, keyRandomBytes)
	if _, err := rand.Read(random); err != nil {
		return "", fmt.Errorf("random key: %w", err)
	}
	return hex.EncodeToString(random), nil
}

func syncDirectory(path string) error {
	directory, err := os.Open(path)
	if err != nil {
		return err
	}
	defer directory.Close()
	return directory.Sync()
}

func classify(err error) error {
	if errors.Is(err, syscall.ENOSPC) || errors.Is(err, syscall.EDQUOT) {
		return ErrStorageFull
	}
	return err
}
