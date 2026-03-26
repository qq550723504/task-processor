package productenrich

import (
	"context"
	"testing"
)

func TestQualityScorer_CalculateScore(t *testing.T) {
	ctx := context.Background()

	cases := []struct {
		name       string
		cfg        *QualityScorerConfig
		validation *ValidationResult
		wantErr    bool
		wantMin    float64
		wantMax    float64
	}{
		{
			name:    "nil validation returns error",
			cfg:     nil,
			wantErr: true,
		},
		{
			name: "only images no scraped",
			cfg:  &QualityScorerConfig{ImageWeight: 0.4, TextWeight: 0.4, ScrapedWeight: 0.2},
			validation: &ValidationResult{
				ImageScore:   100,
				TextScore:    0,
				ScrapedScore: 0,
			},
			// scrapedWeight 重分配后 imageWeight≈0.533, textWeight≈0.467
			// score = 100*0.533 + 0*0.467 = 53.3
			wantMin: 50, wantMax: 60,
		},
		{
			name: "full data high quality",
			cfg:  &QualityScorerConfig{ImageWeight: 0.4, TextWeight: 0.4, ScrapedWeight: 0.2},
			validation: &ValidationResult{
				ImageScore:   100,
				TextScore:    100,
				ScrapedScore: 60,
			},
			// 100*0.4 + 100*0.4 + 60*0.2 = 92
			wantMin: 91, wantMax: 93,
		},
		{
			name: "score capped at 100",
			cfg:  &QualityScorerConfig{ImageWeight: 0.4, TextWeight: 0.4, ScrapedWeight: 0.2},
			validation: &ValidationResult{
				ImageScore:   100,
				TextScore:    100,
				ScrapedScore: 100,
			},
			wantMin: 99, wantMax: 100,
		},
		{
			name: "zero weights uses defaults",
			cfg:  &QualityScorerConfig{},
			validation: &ValidationResult{
				ImageScore: 80,
				TextScore:  60,
			},
			wantMin: 0, wantMax: 100,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			scorer := NewQualityScorer(tc.cfg)
			score, err := scorer.CalculateScore(ctx, tc.validation)
			if tc.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if score < tc.wantMin || score > tc.wantMax {
				t.Errorf("score %.2f not in [%.2f, %.2f]", score, tc.wantMin, tc.wantMax)
			}
		})
	}
}
