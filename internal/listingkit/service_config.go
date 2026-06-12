package listingkit

import (
	"fmt"
)

func NewService(config *ServiceConfig) (Service, error) {
	if config == nil {
		return nil, fmt.Errorf("config cannot be nil")
	}
	if config.Core.Repository == nil {
		return nil, fmt.Errorf("repository cannot be nil")
	}
	if config.Core.ProductService == nil {
		return nil, fmt.Errorf("product service cannot be nil")
	}
	config.applyDefaults()
	svc := newServiceWithConfig(config)
	svc.initializeCollaborators()
	return svc, nil
}

func newServiceWithConfig(config *ServiceConfig) *service {
	defaultSettings := defaultSheinSettings(config.Shein.SheinDefaultStoreID, config.Shein.SheinPricingPolicy)
	svc := newServiceBase(config, defaultSettings)
	applyServiceDependencyGroups(svc, config)
	return svc
}
