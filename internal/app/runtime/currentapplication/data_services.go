package currentapplication

import (
	"errors"
	"net"
	"path/filepath"
	"strconv"
	datahttp "task-processor/internal/dataservice/httpapi"
	"task-processor/internal/integration/acquisition/amazon"
)

type DataServicesConfig struct {
	Database          DatabaseConfig `json:"database"`
	TemporalAddress   string         `json:"temporalAddress"`
	TemporalNamespace string         `json:"temporalNamespace"`
	BrowserExecutable string         `json:"browserExecutable"`
	DriverDirectory   string         `json:"driverDirectory"`
	EnabledSites      []string       `json:"enabledSites"`
	TrustedProxyCIDRs []string       `json:"trustedProxyCIDRs,omitempty"`
}

func (c *Config) validateDataServices() error {
	d := c.DataServices
	if d == nil {
		return nil
	}
	if !c.ProductCollections || c.ProductAcquisitionDatabase == nil || c.CommercialOwnerDatabase == nil || c.Identity.TenantDirectoryToken == "" {
		return errors.New("data services require current Product collections, Resource and exact IAM authority")
	}
	if err := d.Database.validate("dataServices.database"); err != nil {
		return err
	}
	if d.Database.User != "data_services_runtime" || d.Database.MaxConnections > 4 || !sameDatabaseTarget(d.Database, *c.ProductAcquisitionDatabase) {
		return errors.New("data services require their bounded role in the same physical Product database")
	}
	for _, path := range []string{d.BrowserExecutable, d.DriverDirectory} {
		if !filepath.IsAbs(path) || !boundedValue(path, 4096) {
			return errors.New("data services require explicit absolute browser and driver paths")
		}
	}
	host, port, err := net.SplitHostPort(d.TemporalAddress)
	ip := net.ParseIP(host)
	n, _ := strconv.Atoi(port)
	if err != nil || ip == nil || !ip.IsLoopback() || n < 1 || n > 65535 || !boundedValue(d.TemporalNamespace, 128) {
		return errors.New("data services require bounded loopback Temporal")
	}
	if _, err := amazon.New(amazon.Options{ExecutablePath: d.BrowserExecutable, DriverDirectory: d.DriverDirectory, EnabledSites: d.EnabledSites}); err != nil {
		return errors.New("data services require explicit valid Amazon sites")
	}
	return datahttp.ValidateProxyCIDRs(d.TrustedProxyCIDRs)
}
