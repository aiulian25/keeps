// Package server is the composition root: it prepares the data directory,
// opens every component, registers routes and runs background work.
package server

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"time"

	"github.com/aiulian25/keeps/internal/attachments"
	"github.com/aiulian25/keeps/internal/auth"
	"github.com/aiulian25/keeps/internal/blob"
	"github.com/aiulian25/keeps/internal/config"
	"github.com/aiulian25/keeps/internal/httpapi"
	"github.com/aiulian25/keeps/internal/importer"
	"github.com/aiulian25/keeps/internal/items"
	"github.com/aiulian25/keeps/internal/secrets"
	"github.com/aiulian25/keeps/internal/store"
	"github.com/aiulian25/keeps/internal/webui"
)

const (
	maintenanceInterval = time.Hour
	writeProbeFileName  = ".write-probe"
)

type Application struct {
	Configuration config.Configuration
	Store         *store.Store
	Blobs         *blob.Store
	Keys          *secrets.Keys
	Auth          *auth.Service
	Items         *items.Service
	Attachments   *attachments.Service
	Importer      *importer.Service
	Version       string
	modules       []Module
	background    sync.WaitGroup
	// backgroundParent and stopWorkers let a restore pause every worker and
	// start a fresh generation on the restored data.
	backgroundMutex  sync.Mutex
	backgroundParent context.Context
	stopWorkers      context.CancelFunc
	gate             gate
	blobLock         sync.RWMutex
	restoring        atomic.Bool
	downloads        downloads
}

// Module is a feature area that owns routes and optional background work.
type Module interface {
	Routes(mux *http.ServeMux, protect func(http.Handler) http.Handler)
}

// BackgroundModule runs until ctx is cancelled.
type BackgroundModule interface {
	RunBackground(ctx context.Context)
}

// ReferenceChecker reports whether a blob key is still referenced.
type ReferenceChecker interface {
	BlobReferenced(ctx context.Context, key string) (bool, error)
}

// Open prepares /data and opens the store, keys and authentication.
func Open(ctx context.Context, configuration config.Configuration) (*Application, error) {
	if err := prepareDataDirectory(configuration); err != nil {
		return nil, err
	}
	database, err := store.Open(ctx, configuration.DatabasePath(), configuration.Directory(config.BackupDirectoryName))
	if err != nil {
		return nil, err
	}
	keys, err := secrets.LoadOrCreate(filepath.Join(configuration.Directory(config.KeyDirectoryName), config.InstanceKeyFileName))
	if err != nil {
		database.Close()
		return nil, err
	}
	blobs, err := blob.New(configuration.Directory(config.BlobDirectoryName), configuration.Directory(config.TempDirectoryName))
	if err != nil {
		database.Close()
		return nil, err
	}
	authentication, err := auth.NewService(database, configuration, keys)
	if err != nil {
		database.Close()
		return nil, err
	}
	application := &Application{Configuration: configuration, Store: database, Blobs: blobs, Keys: keys, Auth: authentication}
	application.registerModules()
	return application, nil
}

func (a *Application) AddModule(module Module) {
	a.modules = append(a.modules, module)
}

func (a *Application) Close() error {
	return a.Store.Close()
}

// prepareDataDirectory fails early with an actionable message instead of
// crash-looping when the volume is not writable by the runtime user.
func prepareDataDirectory(configuration config.Configuration) error {
	probe := filepath.Join(configuration.DataDirectory, writeProbeFileName)
	file, err := os.OpenFile(probe, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, config.FileMode)
	if err != nil {
		return fmt.Errorf("the data directory %s is not writable by user %d:%d. Give that user ownership of the volume (see docs/install.md); do not use chmod 777: %w",
			configuration.DataDirectory, os.Getuid(), os.Getgid(), err)
	}
	file.Close()
	os.Remove(probe)
	for _, name := range []string{config.BlobDirectoryName, config.KeyDirectoryName, config.TempDirectoryName, config.BackupDirectoryName} {
		if err := os.MkdirAll(configuration.Directory(name), config.DirectoryMode); err != nil {
			return err
		}
	}
	return nil
}

func (a *Application) clientAddress(request *http.Request) string {
	return httpapi.ClientAddress(request, a.Configuration.TrustedProxies)
}

// Handler builds the full route table.
func (a *Application) Handler() (http.Handler, error) {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(writer http.ResponseWriter, request *http.Request) {
		writer.Write([]byte("ok"))
	})
	mux.HandleFunc("GET /readyz", a.handleReady)

	a.Auth.Routes(mux, a.clientAddress)
	a.dataRoutes(mux)
	mux.Handle("GET /api/v1/about", a.Auth.Protect(httpapi.Handler(a.handleAbout)))
	for _, module := range a.modules {
		module.Routes(mux, a.Auth.Protect)
	}
	mux.Handle("/api/", httpapi.Handler(func(http.ResponseWriter, *http.Request) error { return httpapi.ErrNotFound }))

	frontend, err := webui.Handler(a.Auth.ThemeFor)
	if err != nil {
		return nil, err
	}
	mux.Handle("/", frontend)
	return httpapi.Base(a.Configuration.SecureCookies(), a.gate.wrap(mux)), nil
}

// handleAbout tells a signed-in person which version is running. It stays
// behind sign-in so the exact release isn't shown to anyone who finds the page.
func (a *Application) handleAbout(writer http.ResponseWriter, _ *http.Request) error {
	httpapi.WriteJSON(writer, http.StatusOK, map[string]string{"version": a.Version})
	return nil
}

func (a *Application) handleReady(writer http.ResponseWriter, request *http.Request) {
	ready := a.Store.Database.PingContext(request.Context()) == nil && dataDirectoryWritable(a.Configuration)
	if !ready {
		writer.WriteHeader(http.StatusServiceUnavailable)
		writer.Write([]byte("not ready"))
		return
	}
	writer.Write([]byte("ready"))
}

func dataDirectoryWritable(configuration config.Configuration) bool {
	probe := filepath.Join(configuration.Directory(config.TempDirectoryName), writeProbeFileName)
	if err := os.WriteFile(probe, nil, config.FileMode); err != nil {
		return false
	}
	return os.Remove(probe) == nil
}

// Run serves HTTP until ctx is cancelled, then drains requests and
// background work within the shutdown grace period.
func (a *Application) Run(ctx context.Context) error {
	handler, err := a.Handler()
	if err != nil {
		return err
	}
	if code, err := a.Auth.EnsureSetupCode(ctx, false); err == nil && code != "" {
		announceSetupCode(code)
	}

	backgroundContext, stopBackground := context.WithCancel(context.Background())
	defer stopBackground()
	a.startBackground(backgroundContext)

	server := &http.Server{
		Addr:              config.ListenAddress,
		Handler:           handler,
		ReadHeaderTimeout: config.ReadHeaderTimeout,
		ReadTimeout:       config.ReadTimeout,
		WriteTimeout:      config.WriteTimeout,
		IdleTimeout:       config.IdleTimeout,
		MaxHeaderBytes:    config.MaxHeaderBytes,
	}
	serveErrors := make(chan error, 1)
	go func() {
		slog.Info("listening", "address", config.ListenAddress, "origin", a.Configuration.Origin.String(), "direct_tls", a.Configuration.DirectTLS())
		serveErrors <- a.listen(server)
	}()

	select {
	case err := <-serveErrors:
		return err
	case <-ctx.Done():
	}
	slog.Info("shutting down")
	shutdownContext, cancel := context.WithTimeout(context.Background(), config.ShutdownGrace)
	defer cancel()
	shutdownErr := server.Shutdown(shutdownContext)
	stopBackground()
	a.pauseBackground()
	return shutdownErr
}

func (a *Application) listen(server *http.Server) error {
	var err error
	if a.Configuration.DirectTLS() {
		server.TLSConfig = &tls.Config{MinVersion: tls.VersionTLS12}
		err = server.ListenAndServeTLS(a.Configuration.TLSCertificateFile, a.Configuration.TLSKeyFile)
	} else {
		err = server.ListenAndServe()
	}
	if errors.Is(err, http.ErrServerClosed) {
		return nil
	}
	return err
}

// startBackground starts every worker; they stop when parent is cancelled.
func (a *Application) startBackground(parent context.Context) {
	a.backgroundMutex.Lock()
	defer a.backgroundMutex.Unlock()
	a.backgroundParent = parent
	a.startWorkersLocked()
}

// pauseBackground cancels every worker and waits until all have returned, so
// none can write to the data that is about to be replaced.
func (a *Application) pauseBackground() {
	a.backgroundMutex.Lock()
	defer a.backgroundMutex.Unlock()
	if a.stopWorkers == nil {
		return
	}
	a.stopWorkers()
	a.stopWorkers = nil
	a.background.Wait()
}

// resumeBackground starts a fresh generation of workers on the current data.
// Workers recover their own interrupted state when they start.
func (a *Application) resumeBackground() {
	a.backgroundMutex.Lock()
	defer a.backgroundMutex.Unlock()
	if a.backgroundParent == nil || a.backgroundParent.Err() != nil || a.stopWorkers != nil {
		return
	}
	a.startWorkersLocked()
}

func (a *Application) startWorkersLocked() {
	ctx, cancel := context.WithCancel(a.backgroundParent)
	a.stopWorkers = cancel
	a.background.Add(1)
	go func() {
		defer a.background.Done()
		a.maintenanceLoop(ctx)
	}()
	for _, module := range a.modules {
		worker, ok := module.(BackgroundModule)
		if !ok {
			continue
		}
		a.background.Add(1)
		go func() {
			defer a.background.Done()
			worker.RunBackground(ctx)
		}()
	}
}

func (a *Application) maintenanceLoop(ctx context.Context) {
	ticker := time.NewTicker(maintenanceInterval)
	defer ticker.Stop()
	for {
		a.runMaintenance(ctx)
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

func (a *Application) runMaintenance(ctx context.Context) {
	a.blobLock.Lock()
	defer a.blobLock.Unlock()
	if err := a.Auth.PurgeExpiredSessions(ctx); err != nil {
		slog.Warn("session cleanup failed", "error", err)
	}
	removed, err := a.Blobs.Reconcile(func(key string) (bool, error) { return a.blobReferenced(ctx, key) }, config.OrphanBlobGracePeriod)
	if err != nil {
		slog.Warn("blob reconciliation failed", "error", err)
	}
	if removed > 0 {
		slog.Info("removed unreferenced files", "count", removed)
	}
}

func (a *Application) blobReferenced(ctx context.Context, key string) (bool, error) {
	for _, module := range a.modules {
		checker, ok := module.(ReferenceChecker)
		if !ok {
			continue
		}
		referenced, err := checker.BlobReferenced(ctx, key)
		if err != nil || referenced {
			return true, err
		}
	}
	return false, nil
}

func announceSetupCode(code string) {
	fmt.Fprintf(os.Stdout, "\n================ KEEPS SETUP ================\nOne-time setup code: %s\nOpen Keeps in your browser and enter this code to create the administrator.\nKeep it private. It expires in %s. Run 'keeps bootstrap-code' for a new one.\n=============================================\n\n",
		code, config.SetupCodeLifetime)
}
