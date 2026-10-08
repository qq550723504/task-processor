package currentapplication

import "errors"

// StoreCenterConfig uses the independently owned native record database.
type StoreCenterConfig struct {
	Enabled              bool                            `json:"enabled"`
	Database             DatabaseConfig                  `json:"database"`
	OfficialApplications []OfficialStoreConnectionConfig `json:"officialApplications,omitempty"`
}

func (c *Config) validateStoreCenter() error {
	if c.StoreCenter == nil || !c.StoreCenter.Enabled {
		return nil
	}
	s := c.StoreCenter
	if err := validateOfficialApplications(s.OfficialApplications); err != nil {
		return err
	}
	if err := s.Database.validate("storeCenter.database"); err != nil {
		return err
	}
	if s.Database.User != "store_center_runtime" || s.Database.MaxConnections > 8 {
		return errors.New("store center requires narrow runtime roles and at most 8 connections per pool")
	}
	if c.CommercialOwnerDatabase == nil {
		return errors.New("store service requires its resource owner database")
	}
	others := []*DatabaseConfig{&c.SourceAccountDatabase, c.CommercialOwnerDatabase, c.MoneyOwnerDatabase, c.ProductAcquisitionDatabase}
	if c.Referrals.Enabled {
		others = append(others, &c.Referrals.Database)
	}
	if c.Membership != nil {
		others = append(others, &c.Membership.Database)
	}
	if c.ImageAgent != nil {
		others = append(others, &c.ImageAgent.Database)
	}
	if c.ProductAgent != nil && c.ProductAgent.Enabled {
		others = append(others, &c.ProductAgent.Database, &c.ProductAgent.ReviewDatabase, &c.ProductAgent.AssetDatabase)
	}
	for _, other := range others {
		if other != nil && sameDatabaseTarget(s.Database, *other) {
			return errors.New("store center requires a dedicated record database")
		}
	}
	return nil
}

func sameDatabaseTarget(a, b DatabaseConfig) bool {
	return a.Host == b.Host && a.Port == b.Port && a.Database == b.Database
}
