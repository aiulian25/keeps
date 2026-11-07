// Package secrets manages the instance key and authenticated encryption of
// reversible credentials such as TOTP seeds. The key lives in /data/keys; it
// protects against database-only leaks, not theft of the whole data volume.
package secrets

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
)

const (
	keyBytes      = 32
	directoryMode = 0o700
	fileMode      = 0o600
)

var ErrCiphertext = errors.New("encrypted value is damaged or was made with a different instance key")

// Keys can be reloaded in place after a restore, so every holder of the
// pointer sees the restored instance key.
type Keys struct {
	mutex sync.RWMutex
	aead  cipher.AEAD
}

// LoadOrCreate reads the instance key, creating it on first start.
func LoadOrCreate(path string) (*Keys, error) {
	key, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		key, err = createKey(path)
	}
	if err != nil {
		return nil, err
	}
	if len(key) != keyBytes {
		return nil, fmt.Errorf("instance key %s has the wrong length", path)
	}
	return FromBytes(key)
}

func FromBytes(key []byte) (*Keys, error) {
	aead, err := newAEAD(key)
	if err != nil {
		return nil, err
	}
	return &Keys{aead: aead}, nil
}

func newAEAD(key []byte) (cipher.AEAD, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	return cipher.NewGCM(block)
}

// Reload replaces the key with the one stored at path.
func (k *Keys) Reload(path string) error {
	key, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	if len(key) != keyBytes {
		return fmt.Errorf("instance key %s has the wrong length", path)
	}
	aead, err := newAEAD(key)
	if err != nil {
		return err
	}
	k.mutex.Lock()
	k.aead = aead
	k.mutex.Unlock()
	return nil
}

func createKey(path string) ([]byte, error) {
	if err := os.MkdirAll(filepath.Dir(path), directoryMode); err != nil {
		return nil, err
	}
	key := make([]byte, keyBytes)
	if _, err := rand.Read(key); err != nil {
		return nil, err
	}
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, fileMode)
	if err != nil {
		return nil, err
	}
	if _, err := file.Write(key); err != nil {
		file.Close()
		return nil, err
	}
	if err := file.Sync(); err != nil {
		file.Close()
		return nil, err
	}
	return key, file.Close()
}

// Seal encrypts plaintext; purpose binds the ciphertext to its use.
func (k *Keys) Seal(plaintext []byte, purpose string) ([]byte, error) {
	k.mutex.RLock()
	defer k.mutex.RUnlock()
	nonce := make([]byte, k.aead.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return nil, err
	}
	return k.aead.Seal(nonce, nonce, plaintext, []byte(purpose)), nil
}

func (k *Keys) Open(ciphertext []byte, purpose string) ([]byte, error) {
	k.mutex.RLock()
	defer k.mutex.RUnlock()
	nonceSize := k.aead.NonceSize()
	if len(ciphertext) < nonceSize {
		return nil, ErrCiphertext
	}
	plaintext, err := k.aead.Open(nil, ciphertext[:nonceSize], ciphertext[nonceSize:], []byte(purpose))
	if err != nil {
		return nil, ErrCiphertext
	}
	return plaintext, nil
}
