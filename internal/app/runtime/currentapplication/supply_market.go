package currentapplication

import (
	"errors"
	"net"
	"net/url"
	"path/filepath"
	"strconv"
)

type SupplyMarketConfig struct {
	Storage KnowledgeStorageConfig `json:"storage"`
}
type PODConfig struct {
	AssetDatabase     DatabaseConfig `json:"assetDatabase"`
	TemporalAddress   string         `json:"temporalAddress"`
	TemporalNamespace string         `json:"temporalNamespace"`
	CredentialFile    string         `json:"credentialFile"`
	OSSHosts          []string       `json:"ossHosts"`
}

func (c *Config) validateSupplyMarket() error {
	if c.SupplyMarket == nil && c.POD == nil {
		return nil
	}
	if !c.ProductCollections || c.ProductAcquisitionDatabase == nil || c.Identity.TenantDirectoryToken == "" {
		return errors.New("supply market requires current Product collections and live membership authorization")
	}
	if c.SupplyMarket != nil {
		s := c.SupplyMarket.Storage
		if !boundedValue(s.Region, 128) || !boundedValue(s.Bucket, 256) || !boundedValue(s.AccessKeyID, 512) || !boundedValue(s.SecretAccessKey, 1024) || s.Mode != "aws" && s.Mode != "cos" || s.Mode == "cos" && !s.COSImmutableNonVersionedBucketPolicy {
			return errors.New("supply market requires private immutable qualification storage")
		}
		if s.Endpoint != "" {
			u, e := url.Parse(s.Endpoint)
			if e != nil || u.Hostname() == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" || u.Path != "" && u.Path != "/" || u.Scheme != "https" && u.Scheme != "http" {
				return errors.New("invalid qualification storage endpoint")
			}
			ip := net.ParseIP(u.Hostname())
			if u.Scheme == "http" && u.Hostname() != "localhost" && (ip == nil || !ip.IsPrivate() && !ip.IsLoopback()) {
				return errors.New("external qualification storage requires HTTPS")
			}
		}
	}
	if p := c.POD; p != nil {
		if c.SupplyMarket == nil || !filepath.IsAbs(p.CredentialFile) || len(p.CredentialFile) > 4096 || len(p.OSSHosts) == 0 || len(p.OSSHosts) > 8 {
			return errors.New("POD requires market, private credential file and qualified OSS hosts")
		}
		if err := p.AssetDatabase.validate("pod.assetDatabase"); err != nil {
			return err
		}
		if p.AssetDatabase.User != "supply_asset_runtime" || p.AssetDatabase.MaxConnections > 8 {
			return errors.New("POD requires bounded canonical Asset access")
		}
		owners := []*DatabaseConfig{&c.SourceAccountDatabase, c.ProductAcquisitionDatabase, c.CommercialOwnerDatabase, c.MoneyOwnerDatabase, c.NotificationCenterDatabase}
		if c.StoreCenter != nil {
			owners = append(owners, &c.StoreCenter.Database)
		}
		if c.Ecoservices != nil {
			owners = append(owners, &c.Ecoservices.Database)
		}
		if c.Knowledge != nil {
			owners = append(owners, &c.Knowledge.Database)
		}
		if c.Membership != nil {
			owners = append(owners, &c.Membership.Database)
		}
		if c.AIWorkbench != nil {
			owners = append(owners, &c.AIWorkbench.Database)
		}
		if c.ProductAgent != nil {
			owners = append(owners, &c.ProductAgent.Database, &c.ProductAgent.ReviewDatabase)
		}
		if c.Referrals.Enabled {
			owners = append(owners, &c.Referrals.Database)
		}
		if c.ToolMarket != nil {
			owners = append(owners, &c.ToolMarket.Database)
		}
		if c.ProjectCenter != nil {
			owners = append(owners, &c.ProjectCenter.Database)
		}
		for _, owner := range owners {
			if owner != nil && sameDatabaseTarget(p.AssetDatabase, *owner) {
				return errors.New("POD assets require their independently owned database")
			}
		}
		if c.SupplyChain != nil && !sameDatabaseTarget(p.AssetDatabase, c.SupplyChain.AssetDatabase) || c.ImageAgent != nil && !sameDatabaseTarget(p.AssetDatabase, c.ImageAgent.Database) || c.ProductAgent != nil && c.ProductAgent.Enabled && !sameDatabaseTarget(p.AssetDatabase, c.ProductAgent.AssetDatabase) {
			return errors.New("POD must use the current canonical Asset database")
		}
		host, port, e := net.SplitHostPort(p.TemporalAddress)
		ip := net.ParseIP(host)
		n, _ := strconv.Atoi(port)
		if e != nil || ip == nil || !ip.IsLoopback() || n < 1 || n > 65535 || !boundedValue(p.TemporalNamespace, 128) {
			return errors.New("POD workflow requires bounded loopback Temporal")
		}
		for _, host := range p.OSSHosts {
			u, e := url.Parse("https://" + host)
			if e != nil || u.Host != host || u.Hostname() == "" || u.Port() != "" || u.User != nil || u.Path != "" || u.RawQuery != "" || u.Fragment != "" {
				return errors.New("invalid qualified POD OSS host")
			}
		}
	}
	return nil
}
