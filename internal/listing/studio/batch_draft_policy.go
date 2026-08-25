package studio

import "strings"

const DefaultBatchDesignType = "material"

// NormalizeBatchDesignType preserves explicit design types and supplies the
// legacy default for blank batch selections.
func NormalizeBatchDesignType(designType string) string {
	if strings.TrimSpace(designType) == "" {
		return DefaultBatchDesignType
	}
	return designType
}

// ShouldDropCreateGenerationJobs reports whether client-provided generation
// jobs should be ignored while creating a new batch draft.
func ShouldDropCreateGenerationJobs(isCreate bool, generationJobCount int) bool {
	return isCreate && generationJobCount > 0
}

type BatchNameResolutionInput struct {
	RequestedName string
	ExistingName  string
	IsCreate      bool
	ExistingNames []string
}

// ResolveBatchName applies studio batch naming rules after the caller has
// loaded any tenant-scoped existing names needed to generate a default.
func ResolveBatchName(input BatchNameResolutionInput) string {
	if requested := strings.TrimSpace(input.RequestedName); requested != "" {
		return requested
	}
	if !input.IsCreate {
		if existing := strings.TrimSpace(input.ExistingName); existing != "" {
			return existing
		}
	}
	return NextBatchName(input.ExistingNames)
}

// NormalizeHotStyleReferenceImageURLs keeps the first distinct, non-empty
// reference URL because a batch draft has one active hot-style reference.
func NormalizeHotStyleReferenceImageURLs(values []string) []string {
	result := make([]string, 0, 1)
	seen := make(map[string]struct{}, len(values))
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed == "" {
			continue
		}
		if _, ok := seen[trimmed]; ok {
			continue
		}
		seen[trimmed] = struct{}{}
		result = append(result, trimmed)
		break
	}
	return result
}
