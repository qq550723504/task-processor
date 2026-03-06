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
			want:     "TEST_", // 哈希策略只返回哈希值，不包含ASIN
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := GenerateSKU(asin, tt.strategy, prefix, suffix)

			// 检查前缀
			if !strings.HasPrefix(got, prefix) {
				t.Errorf("GenerateSKU() = %v, want prefix %v", got, prefix)
			}

			if tt.strategy == StrategyASINOnly {
				// ASIN Only策略应该包含ASIN和后缀
				if !strings.HasSuffix(got, suffix) {
					t.Errorf("GenerateSKU() = %v, want suffix %v", got, suffix)
				}
				if !strings.Contains(got, asin) {
					t.Errorf("GenerateSKU() = %v, should contain ASIN %v", got, asin)
				}
			} else if tt.strategy == StrategyHash {
				// 哈希策略应该包含后缀，但不包含原始ASIN
				if !strings.HasSuffix(got, suffix) {
					t.Errorf("GenerateSKU() = %v, want suffix %v", got, suffix)
				}
				// 验证哈希值长度（8位哈希 + 前缀 + 后缀）
				expectedLen := len(prefix) + 8 + len(suffix)
				if len(got) != expectedLen {
					t.Errorf("GenerateSKU() length = %v, want %v", len(got), expectedLen)
				}
			}
		})
	}
}

func TestGenerateSKUByStrategy(t *testing.T) {
	asin := "B08N5WRWNW"

	tests := []struct {
		name      string
		strategy  int
		checkASIN bool // 是否应该包含原始ASIN
	}{
		{
			name:      "Strategy_0_ASIN_Only",
			strategy:  StrategyASINOnly,
			checkASIN: true,
		},
		{
			name:      "Strategy_1_Random",
			strategy:  StrategyRandom,
			checkASIN: true,
		},
		{
			name:      "Strategy_2_Timestamp",
			strategy:  StrategyTimestamp,
			checkASIN: true,
		},
		{
			name:      "Strategy_3_Hash",
			strategy:  StrategyHash,
			checkASIN: false, // 哈希策略不包含原始ASIN
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sku := GenerateSKUByStrategy(asin, tt.strategy, "", "")

			if tt.checkASIN {
				// 应该包含原始ASIN
				if !strings.Contains(sku, asin) {
					t.Errorf("GenerateSKUByStrategy() = %v, should contain ASIN %v", sku, asin)
				}

				// 检查长度（除了仅ASIN策略外，其他都应该更长）
				if tt.strategy != StrategyASINOnly && len(sku) <= len(asin) {
					t.Errorf("GenerateSKUByStrategy() = %v, should be longer than ASIN for strategy %d", sku, tt.strategy)
				}
			} else {
				// 哈希策略：应该是8位哈希值
				if len(sku) != 8 {
					t.Errorf("GenerateSKUByStrategy() = %v, hash strategy should return 8 characters", sku)
				}
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
