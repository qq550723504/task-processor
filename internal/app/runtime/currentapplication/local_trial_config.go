package currentapplication

import (
	"errors"
)

// LocalTrialConfig admits only the dedicated, new #36 loopback composition.
// Its database shares Store Center's target, but uses a separate runtime role.
type LocalTrialConfig struct {
	Enabled  bool           `json:"enabled"`
	Database DatabaseConfig `json:"database"`
}

func (trial *LocalTrialConfig) validate(cfg *Config) error {
	if trial == nil {
		return nil
	}
	if cfg == nil || !trial.Enabled || cfg.StoreCenter == nil || !cfg.StoreCenter.Enabled || cfg.ProductAcquisitionDatabase != nil || cfg.ProductAgent != nil || cfg.AIWorkbench != nil || cfg.ImageAgent != nil || cfg.BrowserCollector != nil {
		return errors.New("local trial requires its isolated Store Center and excludes provider execution")
	}
	if err := trial.Database.validate("localTrial.database"); err != nil {
		return err
	}
	issuer, err := validateLoopbackURL("issuerURL", cfg.Identity.IssuerURL)
	if err != nil {
		return err
	}
	if trial.Database.Host != "127.0.0.1" || trial.Database.Port != 5433 ||
		trial.Database.User != "issue36_trial_runtime" || trial.Database.Database != "store_center" || trial.Database.MaxConnections > 4 ||
		!sameDatabaseTarget(trial.Database, cfg.StoreCenter.Database) || trial.Database.Password == cfg.StoreCenter.Database.Password ||
		issuer.Scheme != "https" {
		return errors.New("local trial requires its dedicated narrow role, common Store Center database and loopback HTTPS identity")
	}
	return nil
}
