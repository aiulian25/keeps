package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"filippo.io/age"

	"github.com/aiulian25/keeps/internal/backup"
	"github.com/aiulian25/keeps/internal/config"
	"github.com/aiulian25/keeps/internal/server"
)

const (
	fingerprintBytes  = 6
	maximumSecretSize = 4096
)

type secretFlags struct {
	passphraseFile  *string
	passphraseStdin *bool
	recipient       *string
	identityFile    *string
}

func addSecretFlags(flags *flag.FlagSet, encrypting bool) secretFlags {
	secrets := secretFlags{
		passphraseFile:  flags.String("passphrase-file", "", "read the backup passphrase from this file"),
		passphraseStdin: flags.Bool("passphrase-stdin", false, "read the backup passphrase from standard input"),
	}
	if encrypting {
		secrets.recipient = flags.String("recipient", "", "encrypt to this age public key (age1…) instead of a passphrase")
		return secrets
	}
	secrets.identityFile = flags.String("identity-file", "", "decrypt with this age identity file instead of a passphrase")
	return secrets
}

// parseFileArgument accepts flags before or after the single file argument.
func parseFileArgument(flags *flag.FlagSet, arguments []string, usage string) (string, error) {
	if err := flags.Parse(arguments); err != nil {
		return "", err
	}
	if flags.NArg() == 0 {
		return "", errors.New(usage)
	}
	file := flags.Arg(0)
	if err := flags.Parse(flags.Args()[1:]); err != nil {
		return "", err
	}
	if flags.NArg() != 0 {
		return "", errors.New(usage)
	}
	return file, nil
}

func (s secretFlags) passphrase() (string, error) {
	var source io.Reader
	switch {
	case *s.passphraseStdin:
		source = os.Stdin
	case *s.passphraseFile != "":
		file, err := os.Open(*s.passphraseFile)
		if err != nil {
			return "", err
		}
		defer file.Close()
		source = file
	default:
		return "", errors.New("give --passphrase-file, --passphrase-stdin, or a key option")
	}
	content, err := io.ReadAll(io.LimitReader(source, maximumSecretSize))
	passphrase := strings.TrimRight(string(content), "\r\n")
	if err == nil && passphrase == "" {
		err = errors.New("the passphrase is empty")
	}
	return passphrase, err
}

func (s secretFlags) recipients() ([]age.Recipient, error) {
	if s.recipient != nil && *s.recipient != "" {
		recipient, err := age.ParseX25519Recipient(*s.recipient)
		return []age.Recipient{recipient}, err
	}
	passphrase, err := s.passphrase()
	if err != nil {
		return nil, err
	}
	recipient, err := backup.PassphraseRecipient(passphrase)
	return []age.Recipient{recipient}, err
}

func (s secretFlags) identity() (age.Identity, error) {
	if s.identityFile != nil && *s.identityFile != "" {
		file, err := os.Open(*s.identityFile)
		if err != nil {
			return nil, err
		}
		defer file.Close()
		identities, err := age.ParseIdentities(file)
		if err != nil || len(identities) == 0 {
			return nil, errors.New("no age identity found in the identity file")
		}
		return identities[0], nil
	}
	passphrase, err := s.passphrase()
	if err != nil {
		return nil, err
	}
	return age.NewScryptIdentity(passphrase)
}

// backupCommand writes an encrypted backup into /data/backups (or --output).
func backupCommand(arguments []string) error {
	flags := flag.NewFlagSet("backup", flag.ContinueOnError)
	output := flags.String("output", "", "file to write (default: /data/backups/keeps-backup-<time>.keeps.age)")
	secrets := addSecretFlags(flags, true)
	if err := flags.Parse(arguments); err != nil {
		return err
	}
	recipients, err := secrets.recipients()
	if err != nil {
		return err
	}
	ctx := context.Background()
	application, err := openApplication(ctx)
	if err != nil {
		return err
	}
	defer application.Close()

	path := *output
	if path == "" {
		path = filepath.Join(application.Configuration.Directory(config.BackupDirectoryName), "keeps-backup-"+time.Now().UTC().Format("20060102T150405Z")+".keeps.age")
	}
	file, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_EXCL, config.FileMode)
	if err != nil {
		return err
	}
	manifest, err := application.WriteBackup(ctx, file, recipients...)
	closeErr := file.Close()
	if err == nil {
		err = closeErr
	}
	if err != nil {
		os.Remove(path)
		return err
	}
	fmt.Printf("Backup written: %s (%d files)\n", path, len(manifest.Files))
	fmt.Println("Copy it somewhere off this machine. It cannot be opened without the passphrase or key.")
	if !manifest.Complete() {
		printMissing(manifest)
		return errors.New("the backup was kept, but it is INCOMPLETE: the files above were missing from storage or no longer match what was uploaded")
	}
	return nil
}

// printMissing lists every file an archive lacks or holds damaged, with the
// items it belongs to.
func printMissing(manifest backup.Manifest) {
	problems := len(manifest.Missing) + len(manifest.Damaged)
	fmt.Printf("INCOMPLETE: %d missing and %d damaged referenced files:\n", len(manifest.Missing), len(manifest.Damaged))
	for _, missing := range manifest.MissingFiles {
		items := "not linked to an item"
		if len(missing.Items) > 0 {
			items = strings.Join(missing.Items, ", ")
		}
		problem := missing.Problem
		if problem == "" {
			problem = backup.ProblemMissing
		}
		fmt.Printf("  - %s (%s, %s) — %s\n", missing.Filename, missing.Copy, problem, items)
	}
	if unnamed := problems - len(manifest.MissingFiles); unnamed > 0 {
		fmt.Printf("  - %d more not described in the database\n", unnamed)
	}
}

func verifyBackupCommand(arguments []string) error {
	flags := flag.NewFlagSet("verify-backup", flag.ContinueOnError)
	secrets := addSecretFlags(flags, false)
	path, err := parseFileArgument(flags, arguments, "usage: keeps verify-backup <file> (--passphrase-file <file> | --passphrase-stdin | --identity-file <file>)")
	if err != nil {
		return err
	}
	identity, err := secrets.identity()
	if err != nil {
		return err
	}
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()
	manifest, err := backup.Verify(context.Background(), file, identity, os.TempDir())
	if err != nil {
		return fmt.Errorf("backup is NOT usable: %w", err)
	}
	if !manifest.Complete() {
		fmt.Printf("Backup usable but INCOMPLETE: created %s by Keeps %s, schema %d, %d files, all checksums match.\n", manifest.CreatedAt, manifest.AppVersion, manifest.SchemaVersion, len(manifest.Files))
		printMissing(manifest)
		return errors.New("restoring it needs --accept-incomplete, and the files above will be gone")
	}
	fmt.Printf("Backup OK and complete: created %s by Keeps %s, schema %d, %d files, all checksums match.\n", manifest.CreatedAt, manifest.AppVersion, manifest.SchemaVersion, len(manifest.Files))
	return nil
}

// restoreCommand restores onto a stopped (or brand new) instance.
func restoreCommand(arguments []string) error {
	flags := flag.NewFlagSet("restore", flag.ContinueOnError)
	secrets := addSecretFlags(flags, false)
	acceptIncomplete := flags.Bool("accept-incomplete", false, "restore even if some photos or documents are missing from the backup")
	path, err := parseFileArgument(flags, arguments, "usage: keeps restore <file> (--passphrase-file <file> | --passphrase-stdin | --identity-file <file>) [--accept-incomplete]")
	if err != nil {
		return err
	}
	configuration, err := loadConfiguration()
	if err != nil {
		return err
	}
	release, err := server.AcquireInstanceLock(configuration)
	if err != nil {
		return err
	}
	defer release()
	identity, err := secrets.identity()
	if err != nil {
		return err
	}
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()
	ctx := context.Background()
	for _, name := range []string{config.BlobDirectoryName, config.KeyDirectoryName, config.TempDirectoryName, config.BackupDirectoryName} {
		if err := os.MkdirAll(configuration.Directory(name), config.DirectoryMode); err != nil {
			return err
		}
	}
	staging, manifest, err := backup.Stage(ctx, configuration.DataDirectory, file, identity)
	if err != nil {
		return fmt.Errorf("nothing was changed: %w", err)
	}
	if !manifest.Complete() && !*acceptIncomplete {
		os.RemoveAll(staging)
		printMissing(manifest)
		return errors.New("nothing was changed. Restoring brings back everything else but not these files; run again with --accept-incomplete to restore anyway")
	}
	rollback, err := backup.Swap(configuration.DataDirectory, staging)
	if err != nil {
		return err
	}
	application, err := server.Open(ctx, configuration)
	if err != nil {
		if undoErr := backup.Undo(configuration.DataDirectory, rollback); undoErr != nil {
			return fmt.Errorf("restored data could not be opened (%w), and putting the previous data back also failed (%v). Do not start Keeps. The previous data is kept in %s; see docs/recovery.md", err, undoErr, rollback)
		}
		previous, openErr := server.Open(ctx, configuration)
		if openErr != nil {
			return fmt.Errorf("restored data could not be opened (%w). The previous data was moved back but does not open either (%v); a copy is kept in %s. See docs/recovery.md", err, openErr, rollback)
		}
		previous.Close()
		backup.DiscardRollback(rollback)
		return fmt.Errorf("restored data could not be opened, previous data put back and checked: %w", err)
	}
	defer application.Close()
	if err := application.FinishRestore(ctx); err != nil {
		return err
	}
	fmt.Printf("Restored backup from %s (%d files). Everyone has been signed out.\nPrevious data kept in %s; delete it once you have checked the restore.\n", manifest.CreatedAt, len(manifest.Files), rollback)
	if !manifest.Complete() {
		printMissing(manifest)
	}
	return nil
}

func instanceFingerprint(configuration config.Configuration) (string, error) {
	key, err := os.ReadFile(filepath.Join(configuration.Directory(config.KeyDirectoryName), config.InstanceKeyFileName))
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(key)
	return hex.EncodeToString(sum[:fingerprintBytes]), nil
}

// recoverAdminCommand issues a one-use browser recovery code. It requires
// the server to be stopped and the instance fingerprint to be confirmed.
func recoverAdminCommand(arguments []string) error {
	flags := flag.NewFlagSet("recover-admin", flag.ContinueOnError)
	confirm := flags.String("confirm-instance", "", "the instance fingerprint printed by this command")
	if err := flags.Parse(arguments); err != nil {
		return err
	}
	configuration, err := loadConfiguration()
	if err != nil {
		return err
	}
	release, err := server.AcquireInstanceLock(configuration)
	if err != nil {
		return err
	}
	defer release()
	fingerprint, err := instanceFingerprint(configuration)
	if err != nil {
		return err
	}
	if *confirm != fingerprint {
		return fmt.Errorf("this will sign everyone out and let you set a new administrator password.\nInstance fingerprint: %s\nRun again with --confirm-instance %s to continue", fingerprint, fingerprint)
	}
	ctx := context.Background()
	application, err := server.Open(ctx, configuration)
	if err != nil {
		return err
	}
	defer application.Close()
	code, err := application.Auth.IssueRecoveryCapability(ctx)
	if err != nil {
		return err
	}
	fmt.Printf("Recovery code: %s\nStart Keeps, open %s/recover and enter this code to set a new password.\nIt works once and expires in %s. The authenticator and passkeys are removed; you can set them up again.\n",
		code, configuration.Origin.String(), config.RecoveryCapabilityLifetime)
	return nil
}
