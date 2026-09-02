package listingkit

import "fmt"

func validateServiceConfig(config *ServiceConfig) error {
	if config == nil {
		return fmt.Errorf("config cannot be nil")
	}
	if config.Core.Repository == nil {
		return fmt.Errorf("repository cannot be nil")
	}
	if config.Core.ProductSnapshotReader == nil {
		return fmt.Errorf("product snapshot reader cannot be nil")
	}
	return nil
}

func prepareServiceConfig(config *ServiceConfig) *ServiceConfig {
	if config == nil {
		return nil
	}
	config.applyDefaults()
	return config
}

func buildService(config *ServiceConfig) *service {
	svc := newServiceWithConfig(config)
	svc.initializeCollaborators()
	return svc
}
