package listingkit

import (
	"strings"
	"testing"
)

func TestTaskGenerationQueueReadServiceBoundaryGuardrails(t *testing.T) {
	t.Parallel()

	source := readExactMethodSource(t, "task_generation_service.go", "func (s *taskGenerationService) GetTaskGenerationQueue(")

	assertSourceOccurrenceCount(t, source, "buildTaskGenerationQueueReadSnapshotPhase(s).run(", 1)
	assertSourceOccurrenceCount(t, source, "buildTaskGenerationQueueReadPagePhase().run(", 1)
	assertSourceOccurrenceCount(t, source, "buildTaskGenerationQueueReadResponsePhase().run(", 1)
	assertSourceOrderedContains(t, source, []string{
		"buildTaskGenerationQueueReadSnapshotPhase(s).run(",
		"buildTaskGenerationQueueReadPagePhase().run(",
		"buildTaskGenerationQueueReadResponsePhase().run(",
	})
	assertSourceExcludesAll(t, source, []string{
		"getCurrentListingKitResult(",
		"withListingKitResultGenerationAndReview(",
		"buildGenerationQueuePage(",
		"filterGenerationQueueItems(",
		"sortGenerationQueueItems(",
		"paginateGenerationQueueItems(",
		"attachReviewSummaryToGenerationQueuePage(",
		"buildGenerationQueueDeltaToken(",
		"isGenerationReviewReadNotModified(",
		"applyGenerationConditionalStateToQueuePage(",
	})
}

func TestTaskGenerationQueueReadSnapshotPhaseBoundary(t *testing.T) {
	t.Parallel()

	source := readTaskGenerationSourceFile(t, "task_generation_queue_read_snapshot.go")

	assertSourceContainsAll(t, source, []string{
		"p.service.repo.GetTask(",
		"p.service.listAssetGenerationTasks(",
		"p.service.listGenerationReviews(",
		"withListingKitResultGenerationAndReview(",
		"queue:  reviewedResult.AssetGenerationQueue",
	})
	assertSourceExcludesAll(t, source, []string{
		"buildGenerationQueuePage(",
		"filterGenerationQueueItems(",
		"sortGenerationQueueItems(",
		"paginateGenerationQueueItems(",
		"attachReviewSummaryToGenerationQueuePage(",
		"buildGenerationQueueDeltaToken(",
		"isGenerationReviewReadNotModified(",
		"applyGenerationConditionalStateToQueuePage(",
	})
}

func TestTaskGenerationQueueReadPagePhaseBoundary(t *testing.T) {
	t.Parallel()

	source := readTaskGenerationSourceFile(t, "task_generation_queue_read_page.go")

	assertSourceContainsAll(t, source, []string{
		"buildGenerationQueuePage(",
		"resolveGenerationQueuePage(",
		"resolveGenerationQueuePageSize(",
		"filterGenerationQueueItems(",
		"sortGenerationQueueItems(",
		"paginateGenerationQueueItems(",
		"attachReviewSummaryToGenerationQueuePage(",
	})
	assertSourceExcludesAll(t, source, []string{
		"p.service.repo.GetTask(",
		"listAssetGenerationTasks(",
		"listGenerationReviews(",
		"withListingKitResultGenerationAndReview(",
		"buildGenerationQueueDeltaToken(",
		"isGenerationReviewReadNotModified(",
		"applyGenerationConditionalStateToQueuePage(",
	})
}

func TestTaskGenerationQueueReadResponsePhaseBoundary(t *testing.T) {
	t.Parallel()

	source := readTaskGenerationSourceFile(t, "task_generation_queue_read_response.go")

	assertSourceContainsAll(t, source, []string{
		"buildGenerationQueueDeltaToken(",
		"isGenerationReviewReadNotModified(",
		"applyGenerationConditionalStateToQueuePage(",
	})
	assertSourceExcludesAll(t, source, []string{
		"p.service.repo.GetTask(",
		"listAssetGenerationTasks(",
		"listGenerationReviews(",
		"withListingKitResultGenerationAndReview(",
		"buildGenerationQueuePage(",
		"filterGenerationQueueItems(",
		"sortGenerationQueueItems(",
		"paginateGenerationQueueItems(",
		"attachReviewSummaryToGenerationQueuePage(",
	})
}

func assertSourceOrderedContains(t *testing.T, source string, needles []string) {
	t.Helper()

	offset := 0
	for _, needle := range needles {
		idx := strings.Index(source[offset:], needle)
		if idx < 0 {
			t.Fatalf("source missing ordered segment %q", needle)
		}
		offset += idx + len(needle)
	}
}
