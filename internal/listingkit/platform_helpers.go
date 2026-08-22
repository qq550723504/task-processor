package listingkit

import (
	common "task-processor/internal/publishing/common"
)

func firstNonEmpty(values ...string) string {
	return common.FirstNonEmpty(values...)
}

func uniqueStrings(values []string) []string {
	return common.UniqueStrings(values)
}
