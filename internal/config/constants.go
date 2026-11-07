package config

import "time"

// ListenAddress is fixed by the deployment contract: the container listens on
// port 6030 and Compose publishes it only on the host loopback interface.
const ListenAddress = "0.0.0.0:6030"

const HealthcheckURL = "http://127.0.0.1:6030/healthz"

const ApplicationName = "keeps"

const (
	ReadHeaderTimeout  = 10 * time.Second
	ReadTimeout        = 60 * time.Second
	WriteTimeout       = 10 * time.Minute
	IdleTimeout        = 2 * time.Minute
	ShutdownGrace      = 25 * time.Second
	HealthcheckTimeout = 3 * time.Second
	MaxHeaderBytes     = 64 << 10
	MaxJSONBodyBytes   = 1 << 20
)

const (
	DatabaseFileName     = "keeps.db"
	BlobDirectoryName    = "blobs"
	KeyDirectoryName     = "keys"
	TempDirectoryName    = "tmp"
	BackupDirectoryName  = "backups"
	StagingDirectoryName = "restore-staging"
	InstanceKeyFileName  = "instance.key"
	DirectoryMode        = 0o700
	FileMode             = 0o600
)
