package productenrich

import (
	"context"
	"strings"
	"testing"
)

func TestEnhancementSuggester_GenerateSuggestions(t *testing.T) {
	ctx := context.Background()
	s := NewEnhancementSuggester()

	cases := []struct {
		name            string
		validation      *ValidationResult
		wantErr         bool
		wantRequiredLen int // 最少包含的必需建议数
		wantQualityHint string
	}{
		{
			name:    "nil validation returns error",
			wantErr: true,
		},
		{
			name: "low image and text score",
			validation: &ValidationResult{
				ImageScore:   40,
				TextScore:    20,
				QualityScore: 30,
			},
			// 图片<60 → 必需；文本<30 → 必需
			wantRequiredLen: 2,
		},
		{
			name: "high scores no required actions",
			validation: &ValidationResult{
				ImageScore:   100,
				TextScore:    100,
				ScrapedScore: 60,
				QualityScore: 92,
			},
			wantRequiredLen: 0,
			wantQualityHint: "高质量",
		},
		{
			// 图片60→保留60，文本60→保留60，无抓取 → 60*0.4 + 60*0.4 = 48 → 基础质量
			name: "medium scores basic quality",
			validation: &ValidationResult{
				ImageScore:   60,
				TextScore:    60,
				QualityScore: 55,
			},
			wantQualityHint: "基础质量",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			suggestion, err := s.GenerateSuggestions(ctx, tc.validation)
			if tc.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if len(suggestion.RequiredActions) < tc.wantRequiredLen {
				t.Errorf("RequiredActions len = %d, want >= %d", len(suggestion.RequiredActions), tc.wantRequiredLen)
			}
			if tc.wantQualityHint != "" && !strings.Contains(suggestion.EstimatedQuality, tc.wantQualityHint) {
				t.Errorf("EstimatedQuality = %q, want to contain %q", suggestion.EstimatedQuality, tc.wantQualityHint)
			}
		})
	}
}

func TestEnhancementSuggester_EstimateQuality_DynamicScore(t *testing.T) {
	s := &enhancementSuggester{}

	cases := []struct {
		name       string
		validation *ValidationResult
		wantHint   string
	}{
		{
			// 图片补到75，文本补到60，无抓取 → 75*0.4 + 60*0.4 = 54 → 基础质量
			name:       "low scores → basic quality after improvement",
			validation: &ValidationResult{ImageScore: 0, TextScore: 0, ScrapedScore: 0},
			wantHint:   "基础质量",
		},
		{
			// 图片已100，文本已100，抓取60 → 100*0.4 + 100*0.4 + 60*0.2 = 92 → 高质量
			name:       "high scores → high quality",
			validation: &ValidationResult{ImageScore: 100, TextScore: 100, ScrapedScore: 60},
			wantHint:   "高质量",
		},
		{
			// 图片75→保留75，文本60→保留60，无抓取 → 75*0.4 + 60*0.4 = 54 → 基础质量（≥50 <60）
			name:       "medium scores → basic quality",
			validation: &ValidationResult{ImageScore: 75, TextScore: 60, ScrapedScore: 0},
			wantHint:   "基础质量",
		},
		{
			// 图片85，文本85，抓取60 → 85*0.4 + 85*0.4 + 60*0.2 = 80 → 高质量
			// 图片85→保留85，文本85→保留85 → 85*0.4 + 85*0.4 + 60*0.2 = 80 → 高质量
			name:       "good scores with scraped → high quality",
			validation: &ValidationResult{ImageScore: 85, TextScore: 85, ScrapedScore: 60},
			wantHint:   "高质量",
		},
		{
			// 图片75，文本75，抓取60 → 75*0.4 + 75*0.4 + 60*0.2 = 72 → 中等质量
			name:       "medium scores with scraped → medium quality",
			validation: &ValidationResult{ImageScore: 75, TextScore: 75, ScrapedScore: 60},
			wantHint:   "中等质量",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := s.estimateQualityAfterImprovement(tc.validation)
			if !strings.Contains(got, tc.wantHint) {
				t.Errorf("estimateQualityAfterImprovement = %q, want to contain %q", got, tc.wantHint)
			}
		})
	}
}

func TestEnhancementSuggester_PrioritizeSuggestions(t *testing.T) {
	s := NewEnhancementSuggester()

	suggestions := []string{
		"扩充产品描述至 100 字符以上",
		"添加至少 3 张高质量产品图片",
		"修复错误: 图片验证失败",
	}

	sorted := s.PrioritizeSuggestions(suggestions)
	if len(sorted) != len(suggestions) {
		t.Fatalf("length mismatch: got %d, want %d", len(sorted), len(suggestions))
	}
	// 图片相关应排在最前
	if !strings.Contains(sorted[0], "图片") {
		t.Errorf("first suggestion should be image-related, got: %q", sorted[0])
	}
}
