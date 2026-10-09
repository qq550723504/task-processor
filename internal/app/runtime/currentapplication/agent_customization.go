package currentapplication

import "errors"

func (cfg *Config) validateAgentCustomization() error {
	db := cfg.AgentCustomizationDatabase
	if db == nil {
		return nil
	}
	if err := db.validate("agentCustomizationDatabase"); err != nil {
		return err
	}
	if db.User != "agent_customization_runtime" || db.MaxConnections > 4 {
		return errors.New("agent customization requires its restricted role and at most four connections")
	}
	others := []*DatabaseConfig{&cfg.SourceAccountDatabase, cfg.CommercialOwnerDatabase, cfg.MoneyOwnerDatabase, cfg.ProductAcquisitionDatabase, cfg.NotificationCenterDatabase}
	if cfg.Ecoservices != nil {
		others = append(others, &cfg.Ecoservices.Database)
	}
	if cfg.StoreCenter != nil {
		others = append(others, &cfg.StoreCenter.Database)
	}
	if cfg.LocalTrial != nil {
		others = append(others, &cfg.LocalTrial.Database)
	}
	if cfg.Knowledge != nil {
		others = append(others, &cfg.Knowledge.Database)
	}
	if cfg.Membership != nil {
		others = append(others, &cfg.Membership.Database)
	}
	if cfg.Referrals.Enabled {
		others = append(others, &cfg.Referrals.Database)
	}
	if cfg.AIWorkbench != nil {
		others = append(others, &cfg.AIWorkbench.Database)
	}
	if cfg.ProductAgent != nil {
		others = append(others, &cfg.ProductAgent.Database, &cfg.ProductAgent.ReviewDatabase, &cfg.ProductAgent.AssetDatabase)
	}
	if cfg.ImageAgent != nil {
		others = append(others, &cfg.ImageAgent.Database)
	}
	if cfg.SupplyChain != nil {
		others = append(others, &cfg.SupplyChain.AssetDatabase)
	}
	if cfg.AccountAuditUsage != nil {
		others = append(others, &cfg.AccountAuditUsage.Image, &cfg.AccountAuditUsage.Product)
	}
	for _, other := range others {
		if other != nil && db.Host == other.Host && db.Port == other.Port && db.Database == other.Database {
			return errors.New("agent customization requires a dedicated database")
		}
	}
	return nil
}
