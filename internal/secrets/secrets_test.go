package secrets

import (
	"os"
	"path/filepath"
	"testing"
)

func TestKeyCreatedOnceAndSealRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "keys", "instance.key")
	first, err := LoadOrCreate(path)
	if err != nil {
		t.Fatal(err)
	}
	info, _ := os.Stat(path)
	if info.Mode().Perm() != fileMode {
		t.Fatalf("mode %v", info.Mode().Perm())
	}
	sealed, _ := first.Seal([]byte("seed"), "totp:1")
	second, _ := LoadOrCreate(path)
	opened, err := second.Open(sealed, "totp:1")
	if err != nil || string(opened) != "seed" {
		t.Fatalf("round trip failed: %v", err)
	}
	if _, err := second.Open(sealed, "totp:2"); err != ErrCiphertext {
		t.Fatal("ciphertext accepted for a different purpose")
	}
	sealed[len(sealed)-1] ^= 1
	if _, err := second.Open(sealed, "totp:1"); err != ErrCiphertext {
		t.Fatal("tampered ciphertext accepted")
	}
}
