package listingkit

import (
	"fmt"
	"strings"
)

func (s *service) SetTaskSubmitter(submitter TaskSubmitter) {
	s.taskSubmitter = submitter
}

func (s *service) ConfigureSheinPublishWorkflowClient(client SheinPublishWorkflowClient, enabled bool) {
	s.sheinPublishWorkflowClient = client
	s.sheinPublishWorkflowEnabled = enabled && client != nil
}

func ConfigureSheinPublishWorkflowClient(svc WorkflowClientConfigurer, client SheinPublishWorkflowClient, enabled bool) error {
	if svc == nil {
		return fmt.Errorf("listingkit service is nil")
	}
	svc.ConfigureSheinPublishWorkflowClient(client, enabled)
	return nil
}

func (s *service) ConfigureStandardProductWorkflowClient(client StandardProductWorkflowClient, enabled bool) {
	s.standardProductWorkflowClient = client
	s.standardProductWorkflowEnabled = enabled && client != nil
}

func ConfigureStandardProductWorkflowClient(svc WorkflowClientConfigurer, client StandardProductWorkflowClient, enabled bool) error {
	if svc == nil {
		return fmt.Errorf("listingkit service is nil")
	}
	svc.ConfigureStandardProductWorkflowClient(client, enabled)
	return nil
}

func (s *service) ConfigurePlatformAdaptWorkflowClient(client PlatformAdaptWorkflowClient, enabled bool) {
	s.platformAdaptWorkflowClient = client
	s.platformAdaptWorkflowEnabled = enabled && client != nil
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
		req.Options = &GenerateOptions{ProcessImages: true}
	} else if req.Options.Scene != nil {
		req.Options.ProcessImages = true
	}
	req.Platforms = normalizePlatforms(req.Platforms)
	if len(req.Platforms) == 0 {
		req.Platforms = []string{"amazon", "shein", "temu", "walmart"}
	}
}

func normalizePlatforms(platforms []string) []string {
	if len(platforms) == 0 {
		return nil
	}
	seen := map[string]struct{}{}
	result := make([]string, 0, len(platforms))
	for _, platform := range platforms {
		normalized := strings.ToLower(strings.TrimSpace(platform))
		switch normalized {
		case "amazon", "shein", "temu", "walmart":
			if _, ok := seen[normalized]; ok {
				continue
			}
			seen[normalized] = struct{}{}
			result = append(result, normalized)
		}
	}
	return result
}
