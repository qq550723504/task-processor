package listingkit

import "strings"

func generationQualityGrade(value string) string {
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

func generationQualityGradeLabel(value string) string {
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
