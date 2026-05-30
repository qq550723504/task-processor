package listingkit

import "testing"

func TestTaskGenerationReviewReadServiceBoundary(t *testing.T) {
	t.Parallel()

	sessionSource := readExactMethodSource(t, "task_generation_service.go", "func (s *taskGenerationService) GetTaskGenerationReviewSession(")
	assertSourceOccurrenceCount(t, sessionSource, "buildTaskGenerationReviewReadSnapshotPhase(s).run(", 1)
	assertSourceOccurrenceCount(t, sessionSource, "buildTaskGenerationReviewSessionReadPhase().run(", 1)
	assertSourceExcludesAll(t, sessionSource, []string{
		"getCurrentListingKitResult(",
		"buildGenerationReviewSession(",
		"buildGenerationReviewReadDeltaToken(",
		"isGenerationReviewReadNotModified(",
		"normalizeGenerationActionResponseMode(",
		"buildGenerationReviewSessionBaseQuery(",
		"buildGenerationReviewSessionPatch(",
		"applyGenerationConditionalStateToReviewSessionResponse(",
		"resolveGenerationReviewPreviewResponse(",
		"resolveGenerationReviewPreviewRevisionStatus(",
		"applyGenerationConditionalStateToReviewPreviewResponse(",
	})

	previewSource := readExactMethodSource(t, "task_generation_service.go", "func (s *taskGenerationService) GetTaskGenerationReviewPreview(")
	assertSourceOccurrenceCount(t, previewSource, "buildTaskGenerationReviewReadSnapshotPhase(s).run(", 1)
	assertSourceOccurrenceCount(t, previewSource, "buildTaskGenerationReviewPreviewReadPhase().run(", 1)
	assertSourceExcludesAll(t, previewSource, []string{
		"getCurrentListingKitResult(",
		"buildGenerationReviewSession(",
		"buildGenerationReviewReadDeltaToken(",
		"isGenerationReviewReadNotModified(",
		"normalizeGenerationActionResponseMode(",
		"buildGenerationReviewSessionBaseQuery(",
		"buildGenerationReviewSessionPatch(",
		"applyGenerationConditionalStateToReviewSessionResponse(",
		"resolveGenerationReviewPreviewResponse(",
		"resolveGenerationReviewPreviewRevisionStatus(",
		"applyGenerationConditionalStateToReviewPreviewResponse(",
		"buildGenerationScenePresetSummary(",
	})
}

func TestTaskGenerationReviewReadSnapshotPhaseBoundary(t *testing.T) {
	t.Parallel()

	source := readTaskGenerationSourceFile(t, "task_generation_review_read_snapshot.go")

	assertSourceContainsAll(t, source, []string{
		"getCurrentListingKitResult(",
		"result.AssetGenerationQueue",
	})
	assertSourceExcludesAll(t, source, []string{
		"buildGenerationReviewSession(",
		"buildGenerationReviewReadDeltaToken(",
		"isGenerationReviewReadNotModified(",
		"normalizeGenerationActionResponseMode(",
		"applyGenerationConditionalStateToReviewSessionResponse(",
		"resolveGenerationReviewPreviewResponse(",
		"resolveGenerationReviewPreviewRevisionStatus(",
		"applyGenerationConditionalStateToReviewPreviewResponse(",
		"buildGenerationScenePresetSummary(",
	})
}

func TestTaskGenerationReviewSessionReadPhaseBoundary(t *testing.T) {
	t.Parallel()

	source := readTaskGenerationSourceFile(t, "task_generation_review_session_read.go")

	assertSourceContainsAll(t, source, []string{
		"buildGenerationReviewSession(",
		"buildGenerationReviewReadDeltaToken(",
		"normalizeGenerationActionResponseMode(",
		"isGenerationReviewReadNotModified(",
		"buildGenerationReviewSessionBaseQuery(",
		"buildGenerationReviewSessionPatch(",
		"applyGenerationConditionalStateToReviewSessionResponse(",
	})
	assertSourceExcludesAll(t, source, []string{
		"buildTaskGenerationReviewReadSnapshotPhase(",
		"getCurrentListingKitResult(",
		"resolveGenerationReviewPreviewResponse(",
		"resolveGenerationReviewPreviewRevisionStatus(",
		"applyGenerationConditionalStateToReviewPreviewResponse(",
		"buildGenerationScenePresetSummary(",
	})
}

func TestTaskGenerationReviewPreviewReadPhaseBoundary(t *testing.T) {
	t.Parallel()

	source := readTaskGenerationSourceFile(t, "task_generation_review_preview_read.go")

	assertSourceContainsAll(t, source, []string{
		"buildGenerationReviewSession(",
		"buildGenerationReviewReadDeltaToken(",
		"isGenerationReviewReadNotModified(",
		"resolveGenerationReviewPreviewResponse(",
		"resolveGenerationReviewPreviewRevisionStatus(",
		"applyGenerationConditionalStateToReviewPreviewResponse(",
		"buildGenerationScenePresetSummary(",
	})
	assertSourceExcludesAll(t, source, []string{
		"buildTaskGenerationReviewReadSnapshotPhase(",
		"getCurrentListingKitResult(",
		"normalizeGenerationActionResponseMode(",
		"buildGenerationReviewSessionBaseQuery(",
		"buildGenerationReviewSessionPatch(",
		"applyGenerationConditionalStateToReviewSessionResponse(",
	})
}
