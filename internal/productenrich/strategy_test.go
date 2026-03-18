package productenrich

import (
	"context"
	"testing"
)

func TestStrategySelector_SelectStrategy(t *testing.T) {
	ctx := context.Background()
	sel := NewStrategySelector(nil) // 使用默认阈值 80/60/50

	cases := []struct {
		name  string
		score float64
		want  ProcessingStrategy
	}{
		{"score 100 → full", 100, StrategyFull},
		{"score 80 → full", 80, StrategyFull},
		{"score 79 → basic", 79, StrategyBasic},
		{"score 60 → basic", 60, StrategyBasic},
		{"score 59 → minimal", 59, StrategyMinimal},
		{"score 50 → minimal", 50, StrategyMinimal},
		{"score 49 → reject", 49, StrategyReject},
		{"score 0 → reject", 0, StrategyReject},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := sel.SelectStrategy(ctx, tc.score)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tc.want {
				t.Errorf("SelectStrategy(%.0f) = %q, want %q", tc.score, got, tc.want)
			}
		})
	}
}

func TestStrategySelector_GetStrategyDetails(t *testing.T) {
	sel := NewStrategySelector(nil)

	strategies := []ProcessingStrategy{StrategyFull, StrategyBasic, StrategyMinimal, StrategyReject}
	for _, s := range strategies {
		t.Run(string(s), func(t *testing.T) {
			details, err := sel.GetStrategyDetails(s)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if details.Strategy != s {
				t.Errorf("Strategy = %q, want %q", details.Strategy, s)
			}
			if details.Description == "" {
				t.Error("Description should not be empty")
			}
		})
	}

	t.Run("unknown strategy returns error", func(t *testing.T) {
		_, err := sel.GetStrategyDetails("unknown")
		if err == nil {
			t.Fatal("expected error for unknown strategy")
		}
	})
}
