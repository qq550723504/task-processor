package config

import "time"

type ListingControlPlaneConfig struct {
	Enabled                    bool          `yaml:"enabled"`
	Platform                   string        `yaml:"platform"`
	LeaderLockKey              string        `yaml:"leaderLockKey"`
	LeaderLockTTL              time.Duration `yaml:"leaderLockTTL"`
	CycleTimeout               time.Duration `yaml:"cycleTimeout"`
	ScanInterval               time.Duration `yaml:"scanInterval"`
	BatchSize                  int           `yaml:"batchSize"`
	PerStoreBurst              int           `yaml:"perStoreBurst"`
	MaxQueuedPerStore          int           `yaml:"maxQueuedPerStore"`
	DryRun                     bool          `yaml:"dryRun"`
	EnableLegacyQuotaKeys      bool          `yaml:"enableLegacyQuotaKeys"`
	QuotaKeyTTLGrace           time.Duration `yaml:"quotaKeyTTLGrace"`
	PausedTaskRecoveryInterval time.Duration `yaml:"pausedTaskRecoveryInterval"`
}
