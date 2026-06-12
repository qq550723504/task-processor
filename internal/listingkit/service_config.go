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
	svc.requestDefaults = buildGenerateRequestDefaults(config)
	svc.taskDeps = buildTaskDependencies(config)
	svc.studioDeps = buildStudioDependencies(config)
	svc.submission = buildSubmissionCollaborators()
	svc.adminDeps = buildAdminDependencies(config)
	svc.submissionDeps = buildSubmissionDependencies(config)
	svc.workflowDeps = buildWorkflowDependencies(config)
	svc.sheinRuntimeDeps = buildSheinRuntimeDependencies(config)
	svc.supportDeps = buildSupportDependencies(config)
	return svc
}
