package productimage_test

import (
	"context"
	"testing"

	"task-processor/internal/productimage"
)

type reviewModelStub struct {
	result *productimage.ReviewModelResult
}

func (s *reviewModelStub) Review(_ context.Context, _ *productimage.ReviewModelRequest) (*productimage.ReviewModelResult, error) {
	return s.result, nil
}

func TestModelReviewAssessorAppliesModelDecisionAndRuleGuards(t *testing.T) {
	assessor := productimage.NewModelReviewAssessor(&reviewModelStub{
		result: &productimage.ReviewModelResult{
			Decision:   &productimage.ReviewDecision{NeedsReview: false},
			Confidence: 0.93,
		},
	})

	result := &productimage.ImageProcessResult{
		Quality: &productimage.QualityAssessment{OverallScore: 0.41},
	}

	decision, err := assessor.Assess(context.Background(), &productimage.SourceBundle{}, nil, nil, result)
	if err != nil {
		t.Fatalf("Assess() error = %v", err)
	}
	if !decision.NeedsReview {
		t.Fatalf("decision = %+v, want forced review from rule guard", decision)
	}
	if len(decision.Reasons) != 1 || decision.Reasons[0] != "rule_validation_guard: overall quality below threshold" {
		t.Fatalf("reasons = %+v", decision.Reasons)
	}
}

func TestFallbackPolicyDefaultsToNoLocalSceneFallback(t *testing.T) {
	policy := productimage.DefaultFallbackPolicy()
	if policy.AllowLocalSceneFallback {
		t.Fatalf("policy = %+v, want local scene fallback disabled by default", policy)
	}
}
