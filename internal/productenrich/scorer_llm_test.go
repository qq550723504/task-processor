package productenrich

import (
	"context"
	"errors"
	"testing"
)

// mockLLMScorer mock LLM 评分器
type mockLLMScorer struct {
	textScore   float64
	imageScore  float64
	err         error
	textPrompt  *PromptObservability
	imagePrompt *PromptObservability
}

func (m *mockLLMScorer) ScoreText(_ context.Context, _ string, base float64) (float64, error) {
	if m.err != nil {
		return base, m.err
	}
	return m.textScore, nil
}

func (m *mockLLMScorer) ScoreImage(_ context.Context, _ string, base float64) (float64, error) {
	if m.err != nil {
		return base, m.err
	}
	return m.imageScore, nil
}

func (m *mockLLMScorer) scoreTextResult(_ context.Context, _ string, base float64) (*llmScoreResult, error) {
	if m.err != nil {
		return &llmScoreResult{Score: base}, m.err
	}
	return &llmScoreResult{Score: m.textScore, Prompt: m.textPrompt}, nil
}

func (m *mockLLMScorer) scoreImageResult(_ context.Context, _ string, base float64) (*llmScoreResult, error) {
	if m.err != nil {
		return &llmScoreResult{Score: base}, m.err
	}
	return &llmScoreResult{Score: m.imageScore, Prompt: m.imagePrompt}, nil
}

func TestQualityScorer_LLMBranch(t *testing.T) {
	ctx := context.Background()

	t.Run("LLM scorer overrides base scores", func(t *testing.T) {
		llm := &mockLLMScorer{textScore: 90, imageScore: 95}
		scorer := NewQualityScorer(&QualityScorerConfig{
			ImageWeight:   0.4,
			TextWeight:    0.4,
			ScrapedWeight: 0.2,
			LLMScorer:     llm,
			EnableLLM:     true,
		})

		validation := &ValidationResult{
			ImageScore:   40, // 基础分低
			TextScore:    40,
			ScrapedScore: 60,
			ImageValidation: &ImageValidation{
				ValidImages: []ImageInfo{{URL: "https://example.com/img.jpg", IsValid: true}},
			},
			TextValidation: &TextValidation{
				Length:  100,
				RawText: "some product text",
			},
		}

		score, err := scorer.CalculateScore(ctx, validation)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		// LLM 覆盖后：95*0.4 + 90*0.4 + 60*0.2 = 86
		if score < 80 {
			t.Errorf("score = %.2f, expected >= 80 with LLM override", score)
		}
	})

	t.Run("LLM scorer error falls back to base score", func(t *testing.T) {
		llm := &mockLLMScorer{err: errTestLLM}
		scorer := NewQualityScorer(&QualityScorerConfig{
			ImageWeight: 0.5, TextWeight: 0.5,
			LLMScorer: llm, EnableLLM: true,
		})

		validation := &ValidationResult{
			ImageScore: 60,
			TextScore:  60,
			ImageValidation: &ImageValidation{
				ValidImages: []ImageInfo{{URL: "https://example.com/img.jpg"}},
			},
			TextValidation: &TextValidation{Length: 50, RawText: "text"},
		}

		score, err := scorer.CalculateScore(ctx, validation)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		// 降级到基础分：60*0.5 + 60*0.5 = 60（scrapedWeight 重分配后各0.5）
		if score < 55 || score > 65 {
			t.Errorf("score = %.2f, expected ~60 (fallback to base)", score)
		}
	})

	t.Run("LLM scorer prompt metadata survives quality scoring", func(t *testing.T) {
		llm := &mockLLMScorer{
			textScore:  90,
			imageScore: 95,
			textPrompt: &PromptObservability{
				PromptRef:     "productenrich.llm_scorer.text_scoring",
				PromptKey:     "productenrich.llm_scorer.text_scoring",
				PromptSource:  "registry",
				PromptVersion: "default",
			},
			imagePrompt: &PromptObservability{
				PromptRef:     "productenrich.llm_scorer.image_scoring",
				PromptKey:     "productenrich.llm_scorer.image_scoring",
				PromptSource:  "fallback",
				PromptVersion: "default",
			},
		}
		scorer := NewQualityScorer(&QualityScorerConfig{
			ImageWeight:   0.4,
			TextWeight:    0.4,
			ScrapedWeight: 0.2,
			LLMScorer:     llm,
			EnableLLM:     true,
		})

		validation := &ValidationResult{
			ImageScore:   40,
			TextScore:    40,
			ScrapedScore: 60,
			ImageValidation: &ImageValidation{
				ValidImages: []ImageInfo{{URL: "https://example.com/img.jpg", IsValid: true}},
			},
			TextValidation: &TextValidation{
				Length:  100,
				RawText: "some product text",
			},
		}

		_, err := scorer.CalculateScore(ctx, validation)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if validation.ImageScorePrompt == nil {
			t.Fatal("expected image score prompt metadata")
		}
		if validation.TextScorePrompt == nil {
			t.Fatal("expected text score prompt metadata")
		}
		if validation.ImageScorePrompt.PromptKey != "productenrich.llm_scorer.image_scoring" {
			t.Fatalf("ImageScorePrompt.PromptKey = %q", validation.ImageScorePrompt.PromptKey)
		}
		if validation.TextScorePrompt.PromptKey != "productenrich.llm_scorer.text_scoring" {
			t.Fatalf("TextScorePrompt.PromptKey = %q", validation.TextScorePrompt.PromptKey)
		}
		if validation.ImageScorePrompt.PromptSource != "fallback" {
			t.Fatalf("ImageScorePrompt.PromptSource = %q", validation.ImageScorePrompt.PromptSource)
		}
		if validation.TextScorePrompt.PromptSource != "registry" {
			t.Fatalf("TextScorePrompt.PromptSource = %q", validation.TextScorePrompt.PromptSource)
		}
	})
}

func TestJoinKeywords(t *testing.T) {
	cases := []struct {
		keywords []string
		want     string
	}{
		{nil, ""},
		{[]string{}, ""},
		{[]string{"red", "shoes"}, "red shoes"},
		{[]string{"a", "b", "c"}, "a b c"},
	}
	for _, tc := range cases {
		got := joinKeywords(tc.keywords)
		if got != tc.want {
			t.Errorf("joinKeywords(%v) = %q, want %q", tc.keywords, got, tc.want)
		}
	}
}

// errTestLLM 测试用错误
var errTestLLM = errors.New("llm error")
