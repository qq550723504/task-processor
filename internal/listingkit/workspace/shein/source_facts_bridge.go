package shein

import sheinmarketplace "task-processor/internal/marketplace/shein/workspace"

// SourceFactsReady evaluates whether source-derived SHEIN submit facts are ready for submit.
func SourceFactsReady(metadata map[string]string) (bool, string) {
	return sheinmarketplace.SourceFactsReady(metadata)
}
