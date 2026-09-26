package currentapplication

import (
	"errors"
	"time"

	"task-processor/internal/agent"

	"task-processor/internal/integration/agent/grsaitext"
)

// ProductAgentConfig is an explicit local trial option. Credential values stay
// in the existing Organization AI credential owner, never in this option.
type ProductAgentConfig struct {
	Enabled                bool                      `json:"enabled"`
	Database               DatabaseConfig            `json:"database"`
	ReviewDatabase         DatabaseConfig            `json:"reviewDatabase"`
	AssetDatabase          DatabaseConfig            `json:"assetDatabase"`
	AllowedOrganizationIDs []string                  `json:"allowedOrganizationIds"`
	TextPolicy             grsaitext.AgentTextPolicy `json:"textPolicy"`
	Steps                  int                       `json:"steps"`
	ModelCalls             int                       `json:"modelCalls"`
	Tokens                 int64                     `json:"tokens"`
	CostMicros             int64                     `json:"costMicros"`
	RuntimeSeconds         int                       `json:"runtimeSeconds"`
}

func (p *ProductAgentConfig) Limits() agent.Limits {
	return agent.Limits{Steps: p.Steps, ModelCalls: p.ModelCalls, Tokens: p.Tokens, CostMicros: p.CostMicros, Currency: p.TextPolicy.Currency, Runtime: time.Duration(p.RuntimeSeconds) * time.Second}
}

func (p *ProductAgentConfig) validate(cfg *Config) error {
	if !p.Enabled {
		return nil
	}
	if cfg.ProductAcquisitionDatabase == nil || cfg.CommercialOwnerDatabase == nil {
		return errors.New("product agent requires acquisition and current commercial owner")
	}
	for name, db := range map[string]DatabaseConfig{"database": p.Database, "reviewDatabase": p.ReviewDatabase, "assetDatabase": p.AssetDatabase} {
		if err := db.validate("productAgent." + name); err != nil {
			return err
		}
		if db.MaxConnections > 8 {
			return errors.New("product agent pools must use at most 8 connections")
		}
	}
	product := cfg.ProductAcquisitionDatabase
	if p.ReviewDatabase.Host != product.Host || p.ReviewDatabase.Port != product.Port || p.ReviewDatabase.Database != product.Database {
		return errors.New("product agent review must use the current Product database")
	}
	if !p.Limits().Valid() || p.RuntimeSeconds > 120 || p.RuntimeSeconds < 1 || p.Steps > 16 || p.ModelCalls > 8 || p.TextPolicy.PolicyVersion != "title-review-v1" {
		return errors.New("product agent limits or title policy invalid")
	}
	if len(p.AllowedOrganizationIDs) == 0 || len(p.AllowedOrganizationIDs) > 64 {
		return errors.New("product agent organization allowlist required")
	}
	seen := map[string]bool{}
	for _, id := range p.AllowedOrganizationIDs {
		if !agent.ValidID(id) || seen[id] {
			return errors.New("product agent organization allowlist invalid")
		}
		seen[id] = true
	}
	return nil
}
