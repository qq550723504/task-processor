package currentapplication

import (
	"errors"
	"net"
	"strconv"
)

// Supply uses the current Product and Store owners, the existing Asset owner,
// and the Temporal SDK. Presence is explicit admission; omitted stays closed.
type SupplyChainConfig struct {
	AssetDatabase     DatabaseConfig `json:"assetDatabase"`
	TemporalAddress   string         `json:"temporalAddress"`
	TemporalNamespace string         `json:"temporalNamespace"`
}

func (c *Config) validateSupplyChain() error {
	s := c.SupplyChain
	if s == nil {
		return nil
	}
	if !c.ProductCollections || c.ProductAcquisitionDatabase == nil || c.StoreCenter == nil || !c.StoreCenter.Enabled || len(c.StoreCenter.OfficialApplications) == 0 || c.Identity.TenantDirectoryToken == "" {
		return errors.New("supply chain requires collections, official Store applications and current membership authorization")
	}
	if err := s.AssetDatabase.validate("supplyChain.assetDatabase"); err != nil {
		return err
	}
	if s.AssetDatabase.User != "supply_asset_runtime" || s.AssetDatabase.MaxConnections > 8 {
		return errors.New("supply chain requires bounded supply_asset_runtime Asset access")
	}
	owners := []*DatabaseConfig{&c.SourceAccountDatabase, c.ProductAcquisitionDatabase, c.CommercialOwnerDatabase, c.MoneyOwnerDatabase, &c.StoreCenter.Database}
	if c.ProductAgent != nil {
		owners = append(owners, &c.ProductAgent.Database, &c.ProductAgent.ReviewDatabase)
	}
	if c.AIWorkbench != nil {
		owners = append(owners, &c.AIWorkbench.Database)
	}
	if c.Membership != nil {
		owners = append(owners, &c.Membership.Database)
	}
	if c.Referrals.Enabled {
		owners = append(owners, &c.Referrals.Database)
	}
	for _, owner := range owners {
		if owner != nil && sameDatabaseTarget(s.AssetDatabase, *owner) {
			return errors.New("supply assets require their independently owned database")
		}
	}
	if c.ImageAgent != nil && !sameDatabaseTarget(s.AssetDatabase, c.ImageAgent.Database) || c.ProductAgent != nil && c.ProductAgent.Enabled && !sameDatabaseTarget(s.AssetDatabase, c.ProductAgent.AssetDatabase) {
		return errors.New("supply must use the current canonical Asset database")
	}
	host, port, err := net.SplitHostPort(s.TemporalAddress)
	if err != nil || net.ParseIP(host) == nil || !net.ParseIP(host).IsLoopback() {
		return errors.New("supply Temporal address must use a literal loopback address")
	}
	p, err := strconv.Atoi(port)
	if err != nil || p < 1 || p > 65535 || !boundedValue(s.TemporalNamespace, 128) {
		return errors.New("supply Temporal namespace or port invalid")
	}
	return nil
}
