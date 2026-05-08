package listingkit

import listinggeneration "task-processor/internal/listingkit/generation"

func generationExecutionQualityLabel(value string) string {
	return listinggeneration.ExecutionQualityLabel(value)
}
