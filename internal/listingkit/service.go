package listingkit

import (
	"context"
	"fmt"
	"strings"

	listingplatform "task-processor/internal/listing/platform"
)

func (s *service) SetTaskSubmitter(submitter TaskSubmitter) {
	s.taskDeps.taskSubmitter = submitter
}

func (s *service) ConfigureSheinPublishWorkflowClient(client SheinPublishWorkflowClient, enabled bool) {
	s.setSheinPublishWorkflowClient(client, enabled && client != nil)
}

func ConfigureSheinPublishWorkflowClient(svc WorkflowClientConfigurer, client SheinPublishWorkflowClient, enabled bool) error {
	if svc == nil {
		return fmt.Errorf("listingkit service is nil")
	}
	svc.ConfigureSheinPublishWorkflowClient(client, enabled)
	return nil
}

func (s *service) ConfigureStandardProductWorkflowClient(client StandardProductWorkflowClient, enabled bool) {
	s.setStandardProductWorkflowClient(client, enabled && client != nil)
}

func ConfigureStandardProductWorkflowClient(svc WorkflowClientConfigurer, client StandardProductWorkflowClient, enabled bool) error {
	if svc == nil {
		return fmt.Errorf("listingkit service is nil")
	}
	svc.ConfigureStandardProductWorkflowClient(client, enabled)
	return nil
}

func (s *service) ConfigurePlatformAdaptWorkflowClient(client PlatformAdaptWorkflowClient, enabled bool) {
	s.setPlatformAdaptWorkflowClient(client, enabled && client != nil)
}

func ConfigurePlatformAdaptWorkflowClient(svc WorkflowClientConfigurer, client PlatformAdaptWorkflowClient, enabled bool) error {
	if svc == nil {
		return fmt.Errorf("listingkit service is nil")
	}
	svc.ConfigurePlatformAdaptWorkflowClient(client, enabled)
	return nil
}

func (s *service) currentSheinSubmitSettings() SheinSettings {
	s.sheinSettingsMu.RLock()
	defer s.sheinSettingsMu.RUnlock()
	return s.sheinSettings
}

func (s *service) settingsHealthProbes() SettingsHealthProbes {
	if s == nil {
		return SettingsHealthProbes{}
	}
	return s.healthProbes
}

func (s *service) GetSettingsHealthProbes(context.Context) SettingsHealthProbes {
	return s.settingsHealthProbes()
}

func normalizeGenerateRequest(req *GenerateRequest) {
	if req == nil {
		return
	}
	req.Country = strings.ToUpper(strings.TrimSpace(req.Country))
	req.Language = strings.TrimSpace(req.Language)
	if req.Country == "" {
		req.Country = "US"
	}
	if req.Language == "" {
		req.Language = "en_US"
	}
	if req.Options == nil {
		req.Options = &GenerateOptions{}
	}
	req.Platforms = listingplatform.NormalizeSupportedPlatforms(req.Platforms)
	normalizeGenerateRequestSource(req)
	if len(req.Platforms) == 0 {
		req.Platforms = listingplatform.SupportedPlatforms()
	}
}

func (s *service) setSheinPublishWorkflowClient(client SheinPublishWorkflowClient, enabled bool) {
	s.submissionDeps.sheinPublishWorkflowClient = client
	s.submissionDeps.sheinPublishWorkflowEnabled = enabled
}

func (s *service) setStandardProductWorkflowClient(client StandardProductWorkflowClient, enabled bool) {
	s.taskDeps.standardWorkflowClient = client
	s.taskDeps.standardWorkflowEnabled = enabled
}

func (s *service) setPlatformAdaptWorkflowClient(client PlatformAdaptWorkflowClient, enabled bool) {
	s.taskDeps.platformAdaptWorkflowClient = client
	s.taskDeps.platformAdaptWorkflowEnabled = enabled
}
