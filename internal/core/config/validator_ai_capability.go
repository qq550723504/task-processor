package config

import "strings"

func ValidateAICapabilityConfig(aiCapability *AICapabilityConfig) []error {
	if aiCapability == nil {
		return nil
	}

	mode := strings.ToLower(strings.TrimSpace(aiCapability.StudioImageRoutingMode))
	switch mode {
	case "legacy", "shadow", "active":
		if aiCapability.ProductImageSceneEnabled && len(normalizedTenantIDs(aiCapability.ProductImageSceneAllowedTenantIDs)) == 0 {
			return []error{&ValidationError{
				Field:   "aiCapability.productImageSceneAllowedTenantIDs",
				Message: "must contain at least one tenant ID when product image scene governance is enabled",
				Hint:    "set TASK_PROCESSOR_AI_CAPABILITY_PRODUCT_IMAGE_SCENE_ALLOWED_TENANT_IDS",
			}}
		}
		if aiCapability.ProductEnrichTextEnabled && len(normalizedTenantIDs(aiCapability.ProductEnrichTextAllowedTenantIDs)) == 0 {
			return []error{&ValidationError{
				Field:   "aiCapability.productEnrichTextAllowedTenantIDs",
				Message: "must contain at least one tenant ID when product enrich text governance is enabled",
				Hint:    "set TASK_PROCESSOR_AI_CAPABILITY_PRODUCT_ENRICH_TEXT_ALLOWED_TENANT_IDS",
			}}
		}
		if aiCapability.ProductEnrichVisionEnabled && len(normalizedTenantIDs(aiCapability.ProductEnrichVisionAllowedTenantIDs)) == 0 {
			return []error{&ValidationError{
				Field:   "aiCapability.productEnrichVisionAllowedTenantIDs",
				Message: "must contain at least one tenant ID when product enrich vision governance is enabled",
				Hint:    "set TASK_PROCESSOR_AI_CAPABILITY_PRODUCT_ENRICH_VISION_ALLOWED_TENANT_IDS",
			}}
		}
		if aiCapability.ProductEnrichListingEnabled && len(normalizedTenantIDs(aiCapability.ProductEnrichListingAllowedTenantIDs)) == 0 {
			return []error{&ValidationError{
				Field:   "aiCapability.productEnrichListingAllowedTenantIDs",
				Message: "must contain at least one tenant ID when product enrich listing governance is enabled",
				Hint:    "set TASK_PROCESSOR_AI_CAPABILITY_PRODUCT_ENRICH_LISTING_ALLOWED_TENANT_IDS",
			}}
		}
		return nil
	default:
		return []error{&ValidationError{
			Field:   "aiCapability.studioImageRoutingMode",
			Message: "must be one of legacy, shadow, or active",
			Hint:    "set TASK_PROCESSOR_AI_CAPABILITY_STUDIO_IMAGE_ROUTING_MODE or aiCapability.studioImageRoutingMode",
		}}
	}
}

func normalizedTenantIDs(ids []string) []string {
	seen := make(map[string]struct{}, len(ids))
	result := make([]string, 0, len(ids))
	for _, id := range ids {
		id = strings.TrimSpace(id)
		if id == "" {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		result = append(result, id)
	}
	return result
}
