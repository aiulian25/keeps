package backup

import (
	"archive/tar"
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"

	"filippo.io/age"

	"github.com/aiulian25/keeps/internal/config"
	"github.com/aiulian25/keeps/internal/store"
)

const (
	maximumManifestBytes = 64 << 20
	maximumKeyBytes      = 1 << 10
	stagedDirectoryMode  = 0o700
)

// allowedEntry is the only shape an archive path may have. Anything else
// (absolute paths, "..", unexpected names) rejects the whole archive.
var allowedEntry = regexp.MustCompile(`^(manifest\.json|keeps\.db|keys/instance\.key|blobs/([0-9a-f]{2})/([0-9a-f]{32}))$`)

var (
	ErrWrongSecret     = errors.New("backup_wrong_secret")
	ErrDamaged         = errors.New("backup_damaged")
	ErrIncompatible    = errors.New("backup_incompatible")
	ErrUnsafeEntry     = errors.New("backup_unsafe_entry")
	ErrArchiveTooLarge = errors.New("backup_too_large")
)

// Read decrypts and fully validates an archive. When extractTo is not empty
// every file is written there; nothing outside extractTo is ever touched.
func Read(ctx context.Context, source io.Reader, identity age.Identity, extractTo string) (Manifest, error) {
	return read(ctx, source, identity, extractTo, extractEverything)
}

func extractEverything(string) bool { return true }

func extractDatabaseOnly(name string) bool { return name == databasePath }

// read is Read with a choice of which entries to write to extractTo; every
// entry is still read and checked against the manifest.
func read(ctx context.Context, source io.Reader, identity age.Identity, extractTo string, extract func(name string) bool) (Manifest, error) {
	decrypted, err := age.Decrypt(source, identity)
	if err != nil {
		return Manifest{}, ErrWrongSecret
	}
	archive := tar.NewReader(decrypted)
	seen := map[string]FileEntry{}
	var manifest *Manifest
	var totalBytes int64

	for entries := 0; ; entries++ {
		if err := ctx.Err(); err != nil {
			return Manifest{}, err
		}
		if entries > config.BackupMaximumEntries {
			return Manifest{}, ErrArchiveTooLarge
		}
		header, err := archive.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return Manifest{}, ErrDamaged
		}
		match := allowedEntry.FindStringSubmatch(header.Name)
		isRegular := header.Typeflag == tar.TypeReg
		shardMatches := match == nil || match[2] == "" || match[2] == match[3][:2]
		if match == nil || !isRegular || !shardMatches || header.Size < 0 {
			return Manifest{}, ErrUnsafeEntry
		}
		if _, duplicate := seen[header.Name]; duplicate {
			return Manifest{}, ErrUnsafeEntry
		}
		totalBytes += header.Size
		if totalBytes > config.BackupMaximumTotalBytes || entryTooLarge(header) {
			return Manifest{}, ErrArchiveTooLarge
		}

		if header.Name == manifestPath {
			manifest, err = readManifest(archive)
			if err != nil {
				return Manifest{}, err
			}
			seen[header.Name] = FileEntry{}
			continue
		}
		destination := ""
		if extract(header.Name) {
			destination = extractTo
		}
		entry, err := copyEntry(archive, header, destination)
		if err != nil {
			return Manifest{}, err
		}
		seen[header.Name] = entry
	}
	if manifest == nil {
		return Manifest{}, ErrDamaged
	}
	return *manifest, checkManifest(*manifest, seen)
}

func entryTooLarge(header *tar.Header) bool {
	switch header.Name {
	case manifestPath:
		return header.Size > maximumManifestBytes
	case keyPath:
		return header.Size > maximumKeyBytes
	case databasePath:
		return false
	default:
		return header.Size > config.AttachmentMaximumBytes
	}
}

func readManifest(source io.Reader) (*Manifest, error) {
	var manifest Manifest
	if err := json.NewDecoder(io.LimitReader(source, maximumManifestBytes)).Decode(&manifest); err != nil {
		return nil, ErrDamaged
	}
	if manifest.Format != FormatVersion || manifest.SchemaVersion > store.LatestSchemaVersion() {
		return nil, ErrIncompatible
	}
	return &manifest, nil
}

func copyEntry(archive io.Reader, header *tar.Header, extractTo string) (FileEntry, error) {
	destination := io.Discard
	var file *os.File
	if extractTo != "" {
		target := filepath.Join(extractTo, filepath.FromSlash(header.Name))
		if err := os.MkdirAll(filepath.Dir(target), stagedDirectoryMode); err != nil {
			return FileEntry{}, err
		}
		created, err := os.OpenFile(target, os.O_WRONLY|os.O_CREATE|os.O_EXCL, archiveFileMode)
		if err != nil {
			return FileEntry{}, err
		}
		file = created
		destination = file
	}
	hash := sha256.New()
	written, err := io.Copy(io.MultiWriter(destination, hash), io.LimitReader(archive, header.Size))
	if file != nil {
		if syncErr := file.Sync(); err == nil {
			err = syncErr
		}
		file.Close()
	}
	if err != nil || written != header.Size {
		return FileEntry{}, ErrDamaged
	}
	return FileEntry{Path: header.Name, Size: written, SHA256: hex.EncodeToString(hash.Sum(nil))}, nil
}

// checkManifest requires every listed file to be present with the recorded
// hash, and no unlisted file to be present.
func checkManifest(manifest Manifest, seen map[string]FileEntry) error {
	listed := map[string]bool{manifestPath: true}
	for _, expected := range manifest.Files {
		listed[expected.Path] = true
		actual, found := seen[expected.Path]
		if !found || actual.SHA256 != expected.SHA256 || actual.Size != expected.Size {
			return fmt.Errorf("%w: %s", ErrDamaged, expected.Path)
		}
	}
	for path := range seen {
		if !listed[path] {
			return ErrUnsafeEntry
		}
	}
	if _, hasDatabase := seen[databasePath]; !hasDatabase {
		return ErrDamaged
	}
	if _, hasKey := seen[keyPath]; !hasKey {
		return ErrDamaged
	}
	return nil
}

// CheckDatabase runs SQLite's integrity check on an extracted database.
func CheckDatabase(ctx context.Context, path string) error {
	database, err := sql.Open("sqlite", "file:"+filepath.ToSlash(path)+"?mode=ro")
	if err != nil {
		return err
	}
	defer database.Close()
	var result string
	if err := database.QueryRowContext(ctx, `PRAGMA integrity_check`).Scan(&result); err != nil || result != "ok" {
		return ErrDamaged
	}
	return nil
}

// Verify checks an archive the way a restore would, without restoring it:
// every entry against the manifest, the database's integrity, and every file
// the database refers to (missing, or damaged against its upload record).
// Only the database is written, to a private folder in scratchDirectory that
// is always removed. The returned manifest says whether the archive is
// complete.
func Verify(ctx context.Context, source io.Reader, identity age.Identity, scratchDirectory string) (Manifest, error) {
	scratch, err := os.MkdirTemp(scratchDirectory, "keeps-verify-")
	if err != nil {
		return Manifest{}, err
	}
	defer os.RemoveAll(scratch)
	manifest, err := read(ctx, source, identity, scratch, extractDatabaseOnly)
	if err != nil {
		return Manifest{}, err
	}
	if err := CheckDatabase(ctx, filepath.Join(scratch, databasePath)); err != nil {
		return Manifest{}, err
	}
	if err := checkReferences(ctx, scratch, &manifest); err != nil {
		return Manifest{}, err
	}
	return manifest, nil
}
