package config

import "testing"

func TestGenerationPriceConfigurationHasNoDefaultOrPartialReadiness(t *testing.T) {
	for _, p := range []ImageAgentGenerationConfig{{}, {PriceVersion: "v1"}, {PointsPerImage: 1}, {PriceVersion: "v1", PointsPerImage: -1}, {PriceVersion: " v1", PointsPerImage: 1}, {PriceVersion: "v\n1", PointsPerImage: 1}} {
		if p.Configured() {
			t.Fatalf("invalid configuration marked ready: %#v", p)
		}
	}
	if !(ImageAgentGenerationConfig{PriceVersion: "v1", PointsPerImage: 12}).Configured() {
		t.Fatal("explicit valid price was not configured")
	}
}
