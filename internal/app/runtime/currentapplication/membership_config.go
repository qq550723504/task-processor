package currentapplication

import (
	"errors"
	"task-processor/internal/authidentity"
)

// MembershipConfig is opt-in current application configuration. Credentials
// are never included in diagnostic messages or protocol receipts.
type MembershipConfig struct {
	Database       DatabaseConfig `json:"database"`
	ProviderOrigin string         `json:"providerOrigin"`
	ReadToken      string         `json:"readToken"`
	WriteToken     string         `json:"writeToken"`
}

func (cfg MembershipConfig) validate(identity IdentityConfig) error {
	origin, err := validateLoopbackURL("membership.providerOrigin", cfg.ProviderOrigin)
	if err != nil {
		return errors.New("membership provider origin is invalid")
	}
	issuer, err := validateLoopbackURL("issuerURL", identity.IssuerURL)
	if err != nil || origin.Scheme != issuer.Scheme || origin.Host != issuer.Host || (origin.Path != "" && origin.Path != "/") || origin.RawPath != "" {
		return errors.New("membership provider must use the identity provider origin")
	}
	if !authidentity.IsBoundedIdentifier(identity.ProjectID) || !boundedValue(cfg.ReadToken, 4096) || !boundedValue(cfg.WriteToken, 4096) || cfg.ReadToken == cfg.WriteToken {
		return errors.New("membership requires bounded identity project and separate provider credentials")
	}
	if err := cfg.Database.validate("membership.database"); err != nil {
		return err
	}
	if cfg.Database.User != "organization_membership_runtime" {
		return errors.New("membership requires its dedicated runtime database role")
	}
	return nil
}
