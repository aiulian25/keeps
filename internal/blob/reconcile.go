package blob

import (
	"io/fs"
	"os"
	"path/filepath"
	"time"
)

// Reconcile deletes blob files and abandoned temporary uploads older than
// grace that are not referenced. Every blob deletion in Keeps goes through
// here, so a backup that started less than grace ago always finds its files.
func (s *Store) Reconcile(isReferenced func(key string) (bool, error), grace time.Duration) (removed int, err error) {
	cutoff := time.Now().Add(-grace)

	temporaryEntries, err := os.ReadDir(s.temporary)
	if err != nil {
		return 0, err
	}
	for _, entry := range temporaryEntries {
		info, infoErr := entry.Info()
		if infoErr != nil || info.ModTime().After(cutoff) {
			continue
		}
		if os.Remove(filepath.Join(s.temporary, entry.Name())) == nil {
			removed++
		}
	}

	walkErr := filepath.WalkDir(s.root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil || entry.IsDir() {
			return err
		}
		key := entry.Name()
		if _, pathErr := s.Path(key); pathErr != nil {
			return nil
		}
		info, err := entry.Info()
		if err != nil || info.ModTime().After(cutoff) {
			return err
		}
		referenced, err := isReferenced(key)
		if err != nil || referenced {
			return err
		}
		if os.Remove(path) == nil {
			removed++
		}
		return nil
	})
	return removed, walkErr
}
