package listingkit

import (
	"fmt"
	"net/url"
	"strings"
)

func (s *service) SetTaskSubmitter(submitter TaskSubmitter) {
	s.taskSubmitter = submitter
	s.taskDeps.taskSubmitter = submitter
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
	enabled = enabled && client != nil
	s.standardProductWorkflowClient = client
	s.standardProductWorkflowEnabled = enabled
	s.taskDeps.standardWorkflowClient = client
	s.taskDeps.standardWorkflowEnabled = enabled
}

func ConfigureStandardProductWorkflowClient(svc WorkflowClientConfigurer, client StandardProductWorkflowClient, enabled bool) error {
	if svc == nil {
		return fmt.Errorf("listingkit service is nil")
	}
	svc.ConfigureStandardProductWorkflowClient(client, enabled)
	return nil
}

func (s *service) ConfigurePlatformAdaptWorkflowClient(client PlatformAdaptWorkflowClient, enabled bool) {
	enabled = enabled && client != nil
	s.platformAdaptWorkflowClient = client
	s.platformAdaptWorkflowEnabled = enabled
	s.taskDeps.platformAdaptWorkflowClient = client
	s.taskDeps.platformAdaptWorkflowEnabled = enabled
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
	req.ImageURLs = normalizeGenerateRequestImageURLs(req.ImageURLs)
	if len(req.Platforms) == 0 {
		req.Platforms = []string{"amazon", "shein", "temu", "walmart"}
	}
}

func normalizeGenerateRequestImageURLs(urls []string) []string {
	if len(urls) == 0 {
		return nil
	}
	normalized := make([]string, 0, len(urls))
	for _, rawURL := range urls {
		trimmed := strings.TrimSpace(rawURL)
		if trimmed == "" {
			continue
		}
		if strings.HasPrefix(trimmed, "/api/v1/listing-kits/uploads/files/") {
			trimmed = absolutizeListingKitUploadedImageURL(trimmed)
		}
		normalized = append(normalized, trimmed)
	}
	return normalized
}

func absolutizeListingKitUploadedImageURL(rawURL string) string {
	trimmed := strings.TrimSpace(rawURL)
	if trimmed == "" {
		return ""
	}
	parsed, err := url.Parse(trimmed)
	if err == nil && parsed.IsAbs() {
		return trimmed
	}
	return "http://localhost:3000" + trimmed
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
