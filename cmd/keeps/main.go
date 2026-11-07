package main

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/aiulian25/keeps/internal/auth"
	"github.com/aiulian25/keeps/internal/config"
	"github.com/aiulian25/keeps/internal/server"
)

// version is replaced at build time with -ldflags "-X main.version=<tag>".
var version = "dev"

const usage = `Usage: keeps <command>

Commands:
  serve            Run the Keeps server (default)
  healthcheck      Exit 0 if the local server is healthy
  bootstrap-code   Print a new one-time setup code (only before setup)
  backup           Write an encrypted backup (--passphrase-stdin or --recipient)
  verify-backup    Check that a backup file is complete and can be opened
  restore          Restore a backup (Keeps must be stopped)
  recover-admin    Issue a one-time code to reset the administrator (Keeps must be stopped)
  version          Print the version
`

type command func(arguments []string) error

func commands() map[string]command {
	return map[string]command{
		"serve":          func([]string) error { return serve() },
		"healthcheck":    func([]string) error { return healthcheck() },
		"bootstrap-code": func([]string) error { return bootstrapCode() },
		"backup":         backupCommand,
		"verify-backup":  verifyBackupCommand,
		"restore":        restoreCommand,
		"recover-admin":  recoverAdminCommand,
		"version":        func([]string) error { fmt.Println(version); return nil },
		"help":           func([]string) error { fmt.Print(usage); return nil },
	}
}

func main() {
	name := "serve"
	arguments := os.Args[1:]
	if len(arguments) > 0 {
		name, arguments = arguments[0], arguments[1:]
	}
	run, found := commands()[name]
	if !found {
		fmt.Fprintf(os.Stderr, "%s: unknown command %q\n\n%s", config.ApplicationName, name, usage)
		os.Exit(2)
	}
	if err := run(arguments); err != nil {
		fmt.Fprintf(os.Stderr, "%s: %v\n", config.ApplicationName, err)
		os.Exit(1)
	}
}

func loadConfiguration() (config.Configuration, error) {
	return config.Load(os.LookupEnv)
}

func openApplication(ctx context.Context) (*server.Application, error) {
	configuration, err := loadConfiguration()
	if err != nil {
		return nil, err
	}
	application, err := server.Open(ctx, configuration)
	if err != nil {
		return nil, err
	}
	application.Version = version
	return application, nil
}

func serve() error {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	configuration, err := loadConfiguration()
	if err != nil {
		return err
	}
	release, err := server.AcquireInstanceLock(configuration)
	if err != nil {
		return err
	}
	defer release()
	application, err := openApplication(ctx)
	if err != nil {
		return err
	}
	defer application.Close()
	return application.Run(ctx)
}

// healthcheck probes the server over loopback. In direct-TLS mode the
// certificate names the public hostname, not 127.0.0.1, so verification is
// skipped for this local liveness probe only; no credentials are sent.
func healthcheck() error {
	client := http.Client{Timeout: config.HealthcheckTimeout}
	address := config.HealthcheckURL
	if os.Getenv(config.EnvironmentTLSCertificate) != "" {
		address = strings.Replace(address, "http://", "https://", 1)
		client.Transport = &http.Transport{TLSClientConfig: &tls.Config{InsecureSkipVerify: true}}
	}
	response, err := client.Get(address)
	if err != nil {
		return err
	}
	response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return fmt.Errorf("unhealthy: HTTP %d", response.StatusCode)
	}
	return nil
}

func bootstrapCode() error {
	ctx := context.Background()
	application, err := openApplication(ctx)
	if err != nil {
		return err
	}
	defer application.Close()
	code, err := application.Auth.EnsureSetupCode(ctx, true)
	if errors.Is(err, auth.ErrAlreadyClaimed) {
		return errors.New("this Keeps is already set up; use 'keeps recover-admin' if you lost access")
	}
	if err != nil {
		return err
	}
	fmt.Printf("One-time setup code: %s\nIt replaces any earlier code and expires in %s.\n", code, config.SetupCodeLifetime)
	return nil
}
