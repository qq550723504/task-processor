package currentapplication

import (
	"errors"
	"net"
	"strconv"
)

type StoreObservationsConfig struct {
	TemporalAddress   string `json:"temporalAddress"`
	TemporalNamespace string `json:"temporalNamespace"`
}

func (c *Config) validateStoreObservations() error {
	if c.StoreCenter == nil || c.StoreCenter.Observations == nil {
		return nil
	}
	s := c.StoreCenter.Observations
	if !c.StoreCenter.Enabled || len(c.StoreCenter.OfficialApplications) == 0 || c.Identity.TenantDirectoryToken == "" {
		return errors.New("Store observations require current Store, official applications and exact membership authorization")
	}
	host, port, err := net.SplitHostPort(s.TemporalAddress)
	p, portErr := strconv.Atoi(port)
	if err != nil || portErr != nil || net.ParseIP(host) == nil || !net.ParseIP(host).IsLoopback() || p < 1 || p > 65535 || !boundedValue(s.TemporalNamespace, 128) {
		return errors.New("Store observations require literal loopback Temporal address and bounded namespace")
	}
	return nil
}
