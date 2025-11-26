package utils

import (
	"strings"
	"testing"
)

func TestGenerateSKU(t *testing.T) {
	asin := "B08N5WRWNW"
	prefix := "TEST_"
	suffix := "_END"

	tests := []struct {
		name     string
		strategy int
		want     string
	}{
		{
			name:     "ASIN Only",
			strategy: StrategyASINOnly,
			want:     "TEST_B08N5WRWNW_END",
		},
		{
			name:     "Hash Strategy",
			strategy: StrategyHash,
			want:     "TEST_B08N5WRWNW",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := GenerateSKU(asin, tt.strategy, prefix, suffix)

			// 检查前缀和后缀
			if !strings.HasPrefix(got, prefix) {
				t.Errorf("GenerateSKU() = %v, want prefix %v", got, prefix)
			}

			if tt.strategy == StrategyASINOnly && !strings.HasSuffix(got, suffix) {
				t.Errorf("GenerateSKU() = %v, want suffix %v", got, suffix)
			}

			// 检查包含ASIN
			if !strings.Contains(got, asin) {
				t.Errorf("GenerateSKU() = %v, should contain ASIN %v", got, asin)
			}
		})
	}
}

func TestGenerateSKUByStrategy(t *testing.T) {
	asin := "B08N5WRWNW"

	// 测试不同策略
	strategies := []int{StrategyASINOnly, StrategyRandom, StrategyTimestamp, StrategyHash}

	for _, strategy := range strategies {
		t.Run("Strategy_"+string(rune(strategy+'0')), func(t *testing.T) {
			sku := GenerateSKUByStrategy(asin, strategy, "", "")

			// 所有策略都应该包含原始ASIN
			if !strings.Contains(sku, asin) {
				t.Errorf("GenerateSKUByStrategy() = %v, should contain ASIN %v", sku, asin)
			}

			// 检查长度（除了仅ASIN策略外，其他都应该更长）
			if strategy != StrategyASINOnly && len(sku) <= len(asin) {
				t.Errorf("GenerateSKUByStrategy() = %v, should be longer than ASIN for strategy %d", sku, strategy)
			}
		})
	}
}

func TestGenerateSKUConsistency(t *testing.T) {
	asin := "B08N5WRWNW"

	// 哈希策略应该产生一致的结果
	sku1 := GenerateSKUByStrategy(asin, StrategyHash, "", "")
	sku2 := GenerateSKUByStrategy(asin, StrategyHash, "", "")

	if sku1 != sku2 {
		t.Errorf("Hash strategy should be consistent: %v != %v", sku1, sku2)
	}

	// ASIN Only策略应该产生一致的结果
	sku3 := GenerateSKUByStrategy(asin, StrategyASINOnly, "", "")
	sku4 := GenerateSKUByStrategy(asin, StrategyASINOnly, "", "")

	if sku3 != sku4 {
		t.Errorf("ASIN Only strategy should be consistent: %v != %v", sku3, sku4)
	}
}
