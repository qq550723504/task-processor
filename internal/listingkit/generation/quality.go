package generation

import "strings"

func ExecutionQualityLabel(value string) string {
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

func QualityGrade(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "renderer_output", "exact_asset":
		return "ideal"
	case "pipeline_output", "alias_output":
		return "source_backed"
	case "stub_fallback", "fallback_asset", "queued", "running", "failed":
		return "provisional"
	case "missing":
		return "missing"
	default:
		return ""
	}
}

func QualityGradeLabel(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "ideal":
		return "Ideal"
	case "provisional":
		return "Provisional"
	case "source_backed":
		return "Source Backed"
	case "missing":
		return "Missing"
	default:
		return ""
	}
}
