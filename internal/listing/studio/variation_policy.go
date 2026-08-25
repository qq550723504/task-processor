package studio

import "strings"

type VariationIntensity string

const (
	VariationIntensityLight  VariationIntensity = "light"
	VariationIntensityMedium VariationIntensity = "medium"
	VariationIntensityStrong VariationIntensity = "strong"
)

// NormalizeVariationIntensity keeps the supported variation vocabulary and
// falls back to medium for blank or unknown input.
func NormalizeVariationIntensity(value string) VariationIntensity {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case string(VariationIntensityLight):
		return VariationIntensityLight
	case string(VariationIntensityStrong):
		return VariationIntensityStrong
	default:
		return VariationIntensityMedium
	}
}
