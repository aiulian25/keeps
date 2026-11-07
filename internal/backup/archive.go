// Package backup writes encrypted, self-verifying archives of a Keeps
// instance (database snapshot, instance key and every referenced file) and
// restores them safely.
package backup

import (
	"archive/tar"
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"filippo.io/age"

	"github.com/aiulian25/keeps/internal/blob"
	"github.com/aiulian25/keeps/internal/config"
	"github.com/aiulian25/keeps/internal/store"
)

const (
	FormatVersion   = 1
	manifestPath    = "manifest.json"
	databasePath    = "keeps.db"
	keyPath         = "keys/instance.key"
	blobPrefix      = "blobs/"
	archiveFileMode = 0o600
	snapshotPattern = "snapshot-*.db"
)

type FileEntry struct {
	Path   string `json:"path"`
	Size   int64  `json:"size"`
	SHA256 string `json:"sha256"`
}

type Manifest struct {
	Format        int           `json:"format"`
	AppVersion    string        `json:"appVersion"`
	SchemaVersion int           `json:"schemaVersion"`
	CreatedAt     string        `json:"createdAt"`
	Files         []FileEntry   `json:"files"`
	Missing       []string      `json:"missing,omitempty"`
	MissingFiles  []MissingFile `json:"missingFiles,omitempty"`
}

// Complete reports whether every file the database refers to is in the archive.
func (m Manifest) Complete() bool {
	return len(m.Missing) == 0
}

// MissingFile names a referenced file that is not in the archive, in terms
// the owner recognises: the file name, which copy of it, and its items.
type MissingFile struct {
	Filename string   `json:"filename"`
	Copy     string   `json:"copy"`
	Items    []string `json:"items"`
}

const (
	CopyOriginal  = "original"
	CopyPreview   = "preview"
	CopyThumbnail = "thumbnail"
	itemSeparator = "\x1f"
)

// Sources is what a backup reads from.
type Sources struct {
	Store        *store.Store
	Blobs        *blob.Store
	KeyFile      string
	TemporaryDir string
	AppVersion   string
}

// Write streams an encrypted archive to destination. Nothing unencrypted
// leaves the data directory: the temporary database snapshot is written
// beside the live database and removed afterwards.
func Write(ctx context.Context, sources Sources, destination io.Writer, recipients ...age.Recipient) (Manifest, error) {
	snapshot, err := os.CreateTemp(sources.TemporaryDir, snapshotPattern)
	if err != nil {
		return Manifest{}, err
	}
	snapshotPath := snapshot.Name()
	snapshot.Close()
	os.Remove(snapshotPath)
	defer os.Remove(snapshotPath)

	if err := sources.Store.Snapshot(ctx, snapshotPath); err != nil {
		return Manifest{}, fmt.Errorf("database snapshot: %w", err)
	}
	schemaVersion, blobKeys, err := inspectSnapshot(ctx, snapshotPath)
	if err != nil {
		return Manifest{}, err
	}

	encrypted, err := age.Encrypt(destination, recipients...)
	if err != nil {
		return Manifest{}, err
	}
	archive := tar.NewWriter(encrypted)
	manifest := Manifest{Format: FormatVersion, AppVersion: sources.AppVersion, SchemaVersion: schemaVersion, CreatedAt: store.Now()}

	add := func(archivePath string, filePath string) error {
		entry, err := addFile(archive, archivePath, filePath)
		if err != nil {
			return err
		}
		manifest.Files = append(manifest.Files, entry)
		return nil
	}
	if err := add(databasePath, snapshotPath); err != nil {
		return Manifest{}, err
	}
	if err := add(keyPath, sources.KeyFile); err != nil {
		return Manifest{}, err
	}
	for _, key := range blobKeys {
		if err := ctx.Err(); err != nil {
			return Manifest{}, err
		}
		path, err := sources.Blobs.Path(key)
		if err != nil {
			manifest.Missing = append(manifest.Missing, key)
			continue
		}
		if _, statErr := os.Stat(path); statErr != nil {
			manifest.Missing = append(manifest.Missing, key)
			continue
		}
		if err := add(blobPrefix+key[:2]+"/"+key, path); err != nil {
			return Manifest{}, err
		}
	}
	manifest.MissingFiles, err = describeMissing(ctx, snapshotPath, manifest.Missing)
	if err != nil {
		return Manifest{}, err
	}

	encodedManifest, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return Manifest{}, err
	}
	header := &tar.Header{Name: manifestPath, Mode: archiveFileMode, Size: int64(len(encodedManifest)), ModTime: time.Now(), Typeflag: tar.TypeReg}
	if err := archive.WriteHeader(header); err != nil {
		return Manifest{}, err
	}
	if _, err := archive.Write(encodedManifest); err != nil {
		return Manifest{}, err
	}
	if err := archive.Close(); err != nil {
		return Manifest{}, err
	}
	return manifest, encrypted.Close()
}

func addFile(archive *tar.Writer, archivePath string, filePath string) (FileEntry, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return FileEntry{}, err
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		return FileEntry{}, err
	}
	header := &tar.Header{Name: archivePath, Mode: archiveFileMode, Size: info.Size(), ModTime: info.ModTime(), Typeflag: tar.TypeReg}
	if err := archive.WriteHeader(header); err != nil {
		return FileEntry{}, err
	}
	hash := sha256.New()
	written, err := io.Copy(io.MultiWriter(archive, hash), file)
	if err != nil {
		return FileEntry{}, err
	}
	if written != info.Size() {
		return FileEntry{}, fmt.Errorf("%s changed while it was being backed up", archivePath)
	}
	return FileEntry{Path: archivePath, Size: written, SHA256: hex.EncodeToString(hash.Sum(nil))}, nil
}

// inspectSnapshot reads the schema version and every file key referenced by
// the snapshot, so the archive matches the database exactly.
func inspectSnapshot(ctx context.Context, path string) (int, []string, error) {
	database, err := sql.Open("sqlite", "file:"+filepath.ToSlash(path)+"?mode=ro")
	if err != nil {
		return 0, nil, err
	}
	defer database.Close()
	var schemaVersion int
	if err := database.QueryRowContext(ctx, `SELECT coalesce(max(version), 0) FROM schema_migrations`).Scan(&schemaVersion); err != nil {
		return 0, nil, err
	}
	rows, err := database.QueryContext(ctx, `SELECT storage_key FROM attachments
		UNION SELECT preview_key FROM attachments WHERE preview_key IS NOT NULL
		UNION SELECT thumbnail_key FROM attachments WHERE thumbnail_key IS NOT NULL`)
	if err != nil {
		return 0, nil, err
	}
	defer rows.Close()
	var keys []string
	for rows.Next() {
		var key string
		if err := rows.Scan(&key); err != nil {
			return 0, nil, err
		}
		keys = append(keys, key)
	}
	return schemaVersion, keys, rows.Err()
}

// describeMissing turns missing storage keys into the files and items they
// belong to, read from the database that refers to them.
func describeMissing(ctx context.Context, databasePath string, missing []string) ([]MissingFile, error) {
	if len(missing) == 0 {
		return nil, nil
	}
	isMissing := map[string]bool{}
	for _, key := range missing {
		isMissing[key] = true
	}
	database, err := sql.Open("sqlite", "file:"+filepath.ToSlash(databasePath)+"?mode=ro")
	if err != nil {
		return nil, err
	}
	defer database.Close()
	rows, err := database.QueryContext(ctx, `SELECT a.storage_key, coalesce(a.preview_key, ''), coalesce(a.thumbnail_key, ''), a.filename,
		coalesce((SELECT group_concat(name, char(31)) FROM (
			SELECT i.name FROM item_attachments link JOIN items i ON i.id = link.item_id WHERE link.attachment_id = a.id
			UNION SELECT i.name FROM purchase_attachments link JOIN items i ON i.purchase_id = link.purchase_id WHERE link.attachment_id = a.id)), '')
		FROM attachments a ORDER BY a.id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var described []MissingFile
	for rows.Next() {
		var original, preview, thumbnail, filename, itemNames string
		if err := rows.Scan(&original, &preview, &thumbnail, &filename, &itemNames); err != nil {
			return nil, err
		}
		items := []string{}
		if itemNames != "" {
			items = strings.Split(itemNames, itemSeparator)
		}
		for _, copy := range []struct{ key, name string }{{original, CopyOriginal}, {preview, CopyPreview}, {thumbnail, CopyThumbnail}} {
			if copy.key != "" && isMissing[copy.key] {
				described = append(described, MissingFile{Filename: filename, Copy: copy.name, Items: items})
			}
		}
	}
	return described, rows.Err()
}

// PassphraseRecipient builds an age scrypt recipient with Keeps' work factor.
func PassphraseRecipient(passphrase string) (age.Recipient, error) {
	recipient, err := age.NewScryptRecipient(passphrase)
	if err != nil {
		return nil, err
	}
	recipient.SetWorkFactor(config.BackupScryptWorkFactor)
	return recipient, nil
}
