package config

import "time"

// Security and resource limits from the investigation (§9.2, §10.3, §10.4).
const (
	PasswordMinimumLength      = 10
	PasswordMaximumLength      = 1024
	PasswordHashMemoryKiB      = 64 * 1024
	PasswordHashIterations     = 2
	PasswordHashParallelism    = 1
	ConcurrentPasswordHashes   = 2
	SessionIdleTimeout         = 24 * time.Hour
	SessionAbsoluteLifetime    = 30 * 24 * time.Hour
	SecondFactorPendingTimeout = 5 * time.Minute
	ReauthenticationWindow     = 10 * time.Minute
	SetupCodeLifetime          = 60 * time.Minute
	RecoveryCapabilityLifetime = 15 * time.Minute
	WebAuthnChallengeLifetime  = 5 * time.Minute
	RecoveryCodeCount          = 10
	TOTPAllowedSkewSteps       = 1
	TOTPPeriodSeconds          = 30
	LoginFailuresBeforeDelay   = 5
	LoginDelayBase             = 1 * time.Second
	LoginDelayMaximum          = 5 * time.Minute
	// PasskeyLoginChallengesMaximum bounds unanswered anonymous passkey
	// sign-in challenges kept at once; each expires after
	// WebAuthnChallengeLifetime.
	PasskeyLoginChallengesMaximum = 200
)

const (
	ItemNameMaximumLength   = 200
	ItemTextMaximumLength   = 10000
	ItemShortTextMaximum    = 200
	GroupNameMaximumLength  = 80
	PageSizeDefault         = 50
	PageSizeMaximum         = 200
	DocumentsPageSize       = 100
	RecentlyAddedCount      = 8
	IdempotencyKeyLifetime  = 24 * time.Hour
	SearchQueryMaximumBytes = 200
)

const (
	AttachmentMaximumBytes    = 20 << 20
	AttachmentsPerItemMaximum = 20
	ImageMaximumPixels        = 25_000_000
	PreviewMaximumEdge        = 1600
	ThumbnailMaximumEdge      = 400
	PreviewJPEGQuality        = 85
	OrphanBlobGracePeriod     = 24 * time.Hour
	MinimumFreeDiskBytes      = 256 << 20
)

const (
	ImportTotalBudget        = 20 * time.Second
	ImportMaximumAttempts    = 2
	ImportLeaseDuration      = 60 * time.Second
	ImportDraftLifetime      = 7 * 24 * time.Hour
	ImportImageCandidates    = 5
	ImportWorkerPollInterval = 500 * time.Millisecond
)

const (
	BackupMaximumEntries    = 1_000_000
	BackupMaximumTotalBytes = 200 << 30
	BackupScryptWorkFactor  = 18
	RestoreUploadMaximum    = 200 << 30
	// A restore upload may take longer than ReadTimeout, as long as data keeps
	// arriving: it ends after RestoreUploadIdleTimeout without any, or after
	// RestoreUploadMaximumDuration in total.
	RestoreUploadIdleTimeout     = 2 * time.Minute
	RestoreUploadMaximumDuration = 12 * time.Hour
	// RestoreFinishMaximumDuration bounds the work after the live data is
	// swapped (opening and migrating it, loading its key, signing everyone
	// out), and separately the check that the previous data opens again after
	// a rollback. The server owns it, so a dropped browser can't cut it short.
	RestoreFinishMaximumDuration = 10 * time.Minute
	// DownloadMaximumDuration bounds sending one prepared backup or export,
	// which can take longer than WriteTimeout on a slow connection.
	DownloadMaximumDuration = 12 * time.Hour
)
