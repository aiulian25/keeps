package server

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"syscall"
	"time"

	"github.com/aiulian25/keeps/internal/auth"
	"github.com/aiulian25/keeps/internal/config"
	"github.com/aiulian25/keeps/internal/httpapi"
)

const (
	preparedFilePattern      = "download-*"
	preparedDownloadLifetime = 15 * time.Minute
	downloadIdentifierBytes  = 16
	downloadPathPrefix       = "/api/v1/downloads/"
)

var (
	errNotEnoughSpace = httpapi.NewError(http.StatusInsufficientStorage, "not_enough_space", "There isn't enough free space on the server to prepare this file. Free some space and try again. Nothing was changed.")
	errPrepareFailed  = httpapi.NewError(http.StatusInternalServerError, "prepare_failed", "Keeps couldn't prepare this file. Nothing was changed. Try again; if it keeps failing, check the server logs.")
	errDownloadGone   = httpapi.NewError(http.StatusNotFound, "download_expired", "This download has expired or was already used. Prepare it again.")
)

// preparedDownload is a finished file in the private temporary folder,
// waiting for the browser to fetch it once.
type preparedDownload struct {
	path        string
	filename    string
	contentType string
	userID      int64
	created     time.Time
}

// downloads holds files that were written completely before the browser was
// told they exist, so a failure is reported instead of becoming a partial file.
type downloads struct {
	mutex   sync.Mutex
	entries map[string]preparedDownload
}

// prepare writes the whole file first. On any error the partial file is
// removed and an error the owner can act on is returned.
func (d *downloads) prepare(directory string, userID int64, filename string, contentType string, write func(io.Writer) error) (string, error) {
	d.removeExpired(directory)
	file, err := os.CreateTemp(directory, preparedFilePattern)
	if err != nil {
		return "", preparationError(err)
	}
	err = write(file)
	if err == nil {
		err = file.Sync()
	}
	if closeErr := file.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		os.Remove(file.Name())
		return "", preparationError(err)
	}

	identifier := make([]byte, downloadIdentifierBytes)
	if _, err := rand.Read(identifier); err != nil {
		os.Remove(file.Name())
		return "", err
	}
	id := hex.EncodeToString(identifier)
	d.mutex.Lock()
	defer d.mutex.Unlock()
	if d.entries == nil {
		d.entries = map[string]preparedDownload{}
	}
	d.entries[id] = preparedDownload{path: file.Name(), filename: filename, contentType: contentType, userID: userID, created: time.Now()}
	return downloadPathPrefix + id, nil
}

func preparationError(err error) error {
	slog.Error("preparing a download failed", "error", err)
	if errors.Is(err, syscall.ENOSPC) || errors.Is(err, syscall.EDQUOT) {
		return errNotEnoughSpace
	}
	var apiError *httpapi.Error
	if errors.As(err, &apiError) {
		return apiError
	}
	return errPrepareFailed
}

// take hands a prepared file to exactly one request.
func (d *downloads) take(id string, userID int64) (preparedDownload, bool) {
	d.mutex.Lock()
	defer d.mutex.Unlock()
	entry, found := d.entries[id]
	if !found || entry.userID != userID || time.Since(entry.created) > preparedDownloadLifetime {
		return preparedDownload{}, false
	}
	delete(d.entries, id)
	return entry, true
}

// removeExpired deletes prepared files nobody fetched, including ones left by
// a previous run of the server.
func (d *downloads) removeExpired(directory string) {
	d.mutex.Lock()
	for id, entry := range d.entries {
		if time.Since(entry.created) > preparedDownloadLifetime {
			delete(d.entries, id)
		}
	}
	d.mutex.Unlock()
	leftovers, _ := filepath.Glob(filepath.Join(directory, preparedFilePattern))
	for _, path := range leftovers {
		info, err := os.Stat(path)
		if err == nil && time.Since(info.ModTime()) > preparedDownloadLifetime {
			os.Remove(path)
		}
	}
}

func (a *Application) handleDownload(writer http.ResponseWriter, request *http.Request) error {
	entry, found := a.downloads.take(request.PathValue("id"), auth.PrincipalFromRequest(request).UserID)
	if !found {
		return errDownloadGone
	}
	defer os.Remove(entry.path)
	file, err := os.Open(entry.path)
	if err != nil {
		return errDownloadGone
	}
	defer file.Close()
	// A large backup can take longer to send than ordinary responses may.
	http.NewResponseController(writer).SetWriteDeadline(time.Now().Add(config.DownloadMaximumDuration))
	attachmentHeaders(writer, entry.contentType, entry.filename)
	http.ServeContent(writer, request, entry.filename, entry.created, file)
	return nil
}

func (a *Application) temporaryDirectory() string {
	return a.Configuration.Directory(config.TempDirectoryName)
}
