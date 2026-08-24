package task

import "strings"

type SourcePlatform string
type TargetPlatform string

const (
	SourcePlatformAmazon SourcePlatform = "amazon"
	SourcePlatform1688   SourcePlatform = "1688"

	TargetPlatformShein  TargetPlatform = "shein"
	TargetPlatformTemu   TargetPlatform = "temu"
	TargetPlatformAmazon TargetPlatform = "amazon"
)

// NormalizePlatform returns the canonical representation used by task routes
// and persistence: trimmed, lower-case platform names.
func NormalizePlatform(value string) string {
	return strings.ToLower(strings.TrimSpace(value))
}
