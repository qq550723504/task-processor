package listingkit

import "strings"

func generationExecutionQualityLabel(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "renderer_output":
		return "Renderer Output"
	case "stub_fallback":
		return "Stub Fallback"
	case "pipeline_output":
		return "Pipeline Output"
	case "alias_output":
		return "Alias Output"
	case "exact_asset":
		return "Exact Asset"
	case "fallback_asset":
		return "Fallback Asset"
	case "missing":
		return "Missing"
	case "queued":
		return "Queued"
	case "running":
		return "Running"
	case "failed":
		return "Failed"
	default:
		return ""
	}
}
