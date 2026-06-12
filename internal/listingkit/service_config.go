package listingkit

func NewService(config *ServiceConfig) (Service, error) {
	if err := validateServiceConfig(config); err != nil {
		return nil, err
	}
	return buildService(prepareServiceConfig(config)), nil
}

func newServiceWithConfig(config *ServiceConfig) *service {
	defaultSettings := defaultSheinSettings(config.Shein.SheinDefaultStoreID, config.Shein.SheinPricingPolicy)
	svc := newServiceBase(config, defaultSettings)
	applyServiceDependencyGroups(svc, config)
	return svc
}
