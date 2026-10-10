package currentapplication

import "errors"

type ProjectCenterConfig struct {
	Database DatabaseConfig `json:"database"`
}

func (c *Config) validateProjectCenter() error {
	if c.ProjectCenter == nil {
		return nil
	}
	d := c.ProjectCenter.Database
	if e := d.validate("projectCenter.database"); e != nil {
		return e
	}
	if d.User != "ai_projects_runtime" || d.MaxConnections > 4 {
		return errors.New("project center requires its restricted independent owner")
	}
	owners := []*DatabaseConfig{&c.SourceAccountDatabase, c.CommercialOwnerDatabase, c.MoneyOwnerDatabase, c.ProductAcquisitionDatabase, c.NotificationCenterDatabase}
	if c.StoreCenter != nil {
		owners = append(owners, &c.StoreCenter.Database)
	}
	if c.Knowledge != nil {
		owners = append(owners, &c.Knowledge.Database)
	}
	if c.Membership != nil {
		owners = append(owners, &c.Membership.Database)
	}
	if c.Ecoservices != nil {
		owners = append(owners, &c.Ecoservices.Database)
	}
	if c.ToolMarket != nil {
		owners = append(owners, &c.ToolMarket.Database)
	}
	if c.AIWorkbench != nil {
		owners = append(owners, &c.AIWorkbench.Database)
	}
	if c.ProductAgent != nil {
		owners = append(owners, &c.ProductAgent.Database, &c.ProductAgent.ReviewDatabase, &c.ProductAgent.AssetDatabase)
	}
	if c.ImageAgent != nil {
		owners = append(owners, &c.ImageAgent.Database)
	}
	if c.SupplyChain != nil {
		owners = append(owners, &c.SupplyChain.AssetDatabase)
	}
	if c.Referrals.Enabled {
		owners = append(owners, &c.Referrals.Database)
	}
	if c.LocalTrial != nil && c.LocalTrial.Enabled {
		owners = append(owners, &c.LocalTrial.Database)
	}
	if c.AccountAuditUsage != nil {
		owners = append(owners, &c.AccountAuditUsage.Image, &c.AccountAuditUsage.Product)
	}
	for _, owner := range owners {
		if owner != nil && sameDatabaseTarget(d, *owner) {
			return errors.New("project center requires a dedicated database")
		}
	}
	return nil
}
