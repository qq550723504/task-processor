package studio

import (
	"strconv"
	"strings"
)

const defaultBatchNamePrefix = "批次"

// NextBatchName returns the next default studio batch name for a tenant.
func NextBatchName(existing []string) string {
	maxBatchNumber := 0
	for _, name := range existing {
		nextValue, ok := parseBatchNumber(name)
		if ok && nextValue > maxBatchNumber {
			maxBatchNumber = nextValue
		}
	}
	return defaultBatchNamePrefix + strconv.Itoa(maxBatchNumber+1)
}

func parseBatchNumber(name string) (int, bool) {
	trimmed := strings.TrimSpace(name)
	if !strings.HasPrefix(trimmed, defaultBatchNamePrefix) {
		return 0, false
	}
	value, err := strconv.Atoi(strings.TrimPrefix(trimmed, defaultBatchNamePrefix))
	if err != nil || value <= 0 {
		return 0, false
	}
	return value, true
}
