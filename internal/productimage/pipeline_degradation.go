package productimage

import (
	"context"
	"errors"
	"fmt"
	"time"
)

type needsReviewStageFailure struct {
	stage  string
	reason string
	cause  error
}

func newNeedsReviewStageFailure(stage string, err error, reason string) error {
	return &needsReviewStageFailure{
		stage:  stage,
		reason: reason,
		cause:  err,
	}
}

func (e *needsReviewStageFailure) Error() string {
	if e == nil || e.cause == nil {
		return ""
	}
	return e.cause.Error()
}

func (e *needsReviewStageFailure) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.cause
}

func asNeedsReviewStageFailure(err error) (*needsReviewStageFailure, bool) {
	var target *needsReviewStageFailure
	if !errors.As(err, &target) {
		return nil, false
	}
	return target, true
}

func (s *PipelineState) ensureReview() *ReviewDecision {
	s.ensureResult()
	if s.Result.Review == nil {
		s.Result.Review = &ReviewDecision{}
	}
	return s.Result.Review
}

func (s *PipelineState) markNeedsReviewStage(stage string, durationMS int64, reason string) {
	review := s.ensureReview()
	review.NeedsReview = true
	review.Reasons = uniqueStrings(append(review.Reasons, reason))
	s.Result.StageSummaries = append(s.Result.StageSummaries, ImageStageSummary{
		Stage:      stage,
		Outcome:    "needs_review",
		DurationMS: durationMS,
		Message:    reason,
	})
}

func (s *service) runNeedsReviewRecoveryStages(ctx context.Context, state *PipelineState, failedStage string) error {
	followupStages := s.needsReviewRecoveryStages(failedStage)
	for _, stage := range followupStages {
		startedAt := time.Now()
		if err := stage.Run(ctx, state); err != nil {
			durationMS := time.Since(startedAt).Milliseconds()
			state.ensureResult()
			state.Result.StageSummaries = append(state.Result.StageSummaries, ImageStageSummary{
				Stage:      stage.Name(),
				Outcome:    "failed",
				DurationMS: durationMS,
				Message:    err.Error(),
			})
			return fmt.Errorf("%s failed after %dms: %w", stage.Name(), durationMS, err)
		}
		state.ensureResult()
		state.Result.StageSummaries = append(state.Result.StageSummaries, ImageStageSummary{
			Stage:      stage.Name(),
			Outcome:    "success",
			DurationMS: time.Since(startedAt).Milliseconds(),
		})
	}
	return nil
}

func (s *service) needsReviewRecoveryStages(failedStage string) []Stage {
	var followupStages []Stage
	switch failedStage {
	case "extract_subject":
		followupStages = append(followupStages,
			stageFunc{name: "cleanup_image", run: s.runCleanupStage},
			stageFunc{name: "render_white_bg", run: s.runWhiteBgStage},
			stageFunc{name: "render_gallery", run: s.runGalleryStage},
		)
	case "render_white_bg":
		followupStages = append(followupStages,
			stageFunc{name: "render_gallery", run: s.runGalleryStage},
		)
	case "render_gallery":
	default:
	}
	followupStages = append(followupStages,
		stageFunc{name: "assess_quality", run: s.runQualityStage},
		stageFunc{name: "assess_ip_risk", run: s.runIPRiskStage},
		stageFunc{name: "assess_review", run: s.runReviewStage},
		stageFunc{name: "publish_assets", run: s.runPublishStage},
	)
	return followupStages
}
