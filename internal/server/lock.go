package server

import (
	"errors"
	"os"
	"path/filepath"
	"syscall"

	"github.com/aiulian25/keeps/internal/config"
)

const lockFileName = "keeps.lock"

var ErrInstanceRunning = errors.New("Keeps is running. Stop it first (docker compose stop keeps), then run this command again")

// AcquireInstanceLock takes an exclusive lock on the data directory. The
// server holds it while running; restore and recover-admin need it too, so
// they cannot change data underneath a live server.
func AcquireInstanceLock(configuration config.Configuration) (func(), error) {
	file, err := os.OpenFile(filepath.Join(configuration.DataDirectory, lockFileName), os.O_CREATE|os.O_RDWR, config.FileMode)
	if err != nil {
		return nil, err
	}
	if err := syscall.Flock(int(file.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		file.Close()
		return nil, ErrInstanceRunning
	}
	return func() {
		syscall.Flock(int(file.Fd()), syscall.LOCK_UN)
		file.Close()
	}, nil
}
