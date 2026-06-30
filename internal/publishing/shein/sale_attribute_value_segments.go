package shein

import sheinmarketpub "task-processor/internal/marketplace/shein/publishing"

func comparableAttributeSegments(value string) []string {
	return sheinmarketpub.ComparableAttributeSegments(value)
}
