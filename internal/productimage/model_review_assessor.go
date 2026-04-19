package productimage

import "context"

type modelReviewAssessor struct {
	model ImageReviewModel
}

func NewModelReviewAssessor(model ImageReviewModel) ReviewAssessor {
	return &modelReviewAssessor{model: model}
}

func (a *modelReviewAssessor) Assess(ctx context.Context, source *SourceBundle, audits []ImageAudit, candidates *ImageCandidateSet, result *ImageProcessResult) (*ReviewDecision, error) {
	decision := &ReviewDecision{}
	if a.model != nil {
		var reviewContext *ProductContext
		if source != nil {
			reviewContext = source.Context
		}
		modelResult, err := a.model.Review(ctx, &ReviewModelRequest{
			Source:  source,
			Result:  result,
			Context: reviewContext,
		})
		if err != nil {
			return nil, err
		}
		if modelResult != nil && modelResult.Decision != nil {
			decision = &ReviewDecision{
				NeedsReview: modelResult.Decision.NeedsReview,
				Reasons:     append([]string(nil), modelResult.Decision.Reasons...),
			}
		}
	}

	if result != nil && result.Quality != nil && result.Quality.OverallScore < 0.65 {
		decision.NeedsReview = true
		decision.Reasons = append(decision.Reasons, "rule_validation_guard: overall quality below threshold")
	}
	decision.Reasons = uniqueStrings(decision.Reasons)
	return decision, nil
}
