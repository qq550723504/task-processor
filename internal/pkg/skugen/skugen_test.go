package skugen

import (
	"strings"
	"testing"
)

func TestGenerate(t *testing.T) {
	asin := "B08N5WRWNW"
	prefix := "TEST_"
	suffix := "_END"

	tests := []struct {
		name     string
		strategy int
	}{
		{name: "ASIN Only", strategy: StrategyASINOnly},
		{name: "Hash Strategy", strategy: StrategyHash},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Generate(asin, tt.strategy, prefix, suffix)

			if !strings.HasPrefix(got, prefix) {
				t.Errorf("Generate() = %v, want prefix %v", got, prefix)
			}
			if !strings.HasSuffix(got, suffix) {
				t.Errorf("Generate() = %v, want suffix %v", got, suffix)
			}

			if tt.strategy == StrategyASINOnly {
				if !strings.Contains(got, asin) {
					t.Errorf("Generate() = %v, should contain ASIN %v", got, asin)
				}
			} else if tt.strategy == StrategyHash {
				expectedLen := len(prefix) + 8 + len(suffix)
				if len(got) != expectedLen {
					t.Errorf("Generate() length = %v, want %v", len(got), expectedLen)
				}
			}
		})
	}
}

func TestGenerateByStrategy(t *testing.T) {
	asin := "B08N5WRWNW"

	tests := []struct {
		name      string
		strategy  int
		checkASIN bool
	}{
		{name: "Strategy_0_ASIN_Only", strategy: StrategyASINOnly, checkASIN: true},
		{name: "Strategy_1_Random", strategy: StrategyRandom, checkASIN: true},
		{name: "Strategy_2_Timestamp", strategy: StrategyTimestamp, checkASIN: true},
		{name: "Strategy_3_Hash", strategy: StrategyHash, checkASIN: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sku := GenerateByStrategy(asin, tt.strategy, "", "")

			if tt.checkASIN {
				if !strings.Contains(sku, asin) {
					t.Errorf("GenerateByStrategy() = %v, should contain ASIN %v", sku, asin)
				}
				if tt.strategy != StrategyASINOnly && len(sku) <= len(asin) {
					t.Errorf("GenerateByStrategy() = %v, should be longer than ASIN for strategy %d", sku, tt.strategy)
				}
			} else {
				if len(sku) != 8 {
					t.Errorf("GenerateByStrategy() = %v, hash strategy should return 8 characters", sku)
				}
			}
		})
	}
}

func TestGenerateConsistency(t *testing.T) {
	asin := "B08N5WRWNW"

	sku1 := GenerateByStrategy(asin, StrategyHash, "", "")
	sku2 := GenerateByStrategy(asin, StrategyHash, "", "")
	if sku1 != sku2 {
		t.Errorf("Hash strategy should be consistent: %v != %v", sku1, sku2)
	}

	sku3 := GenerateByStrategy(asin, StrategyASINOnly, "", "")
	sku4 := GenerateByStrategy(asin, StrategyASINOnly, "", "")
	if sku3 != sku4 {
		t.Errorf("ASIN Only strategy should be consistent: %v != %v", sku3, sku4)
	}
}
