package config

import "strings"

func ValidateAICapabilityConfig(aiCapability *AICapabilityConfig) []error {
	if aiCapability == nil {
		return nil
	}

	mode := strings.ToLower(strings.TrimSpace(aiCapability.StudioImageRoutingMode))
	switch mode {
	case "legacy", "shadow", "active":
		return nil
	default:
		return []error{&ValidationError{
			Field:   "aiCapability.studioImageRoutingMode",
			Message: "must be one of legacy, shadow, or active",
			Hint:    "set TASK_PROCESSOR_AI_CAPABILITY_STUDIO_IMAGE_ROUTING_MODE or aiCapability.studioImageRoutingMode",
		}}
	}
}
