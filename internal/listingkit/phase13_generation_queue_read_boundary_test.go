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
		"repo.GetTask(",
		"listAssetGenerationTasks(",
		"listGenerationReviews(",
		"getCurrentListingKitResult(",
		"withListingKitResultGenerationAndReview(",
		"buildGenerationQueuePage(",
		"filterGenerationQueueItems(",
		"sortGenerationQueueItems(",
		"paginateGenerationQueueItems(",
		"attachReviewSummaryToGenerationQueuePage(",
		"buildGenerationQueueDeltaToken(",
		"listinggeneration.IsReadNotModified(",
		"applyGenerationConditionalStateToQueuePage(",
	})
}

func TestTaskGenerationQueueReadSnapshotPhaseBoundary(t *testing.T) {
	t.Parallel()

	source := readExactMethodSource(t, "task_generation_queue_read_snapshot.go", "func (p *taskGenerationQueueReadSnapshotPhase) run(")

	assertSourceContainsAll(t, source, []string{
		"p.service.repo.GetTask(",
		"p.service.listAssetGenerationTasks(",
		"p.service.listGenerationReviews(",
		"withListingKitResultGenerationAndReview(",
		"AssetGenerationQueue",
	})
	assertSourceExcludesAll(t, source, []string{
		"getCurrentListingKitResult(",
		"buildGenerationQueuePage(",
		"filterGenerationQueueItems(",
		"sortGenerationQueueItems(",
		"paginateGenerationQueueItems(",
		"attachReviewSummaryToGenerationQueuePage(",
		"buildGenerationQueueDeltaToken(",
		"listinggeneration.IsReadNotModified(",
		"applyGenerationConditionalStateToQueuePage(",
	})
}

func TestTaskGenerationQueueReadPagePhaseBoundary(t *testing.T) {
	t.Parallel()

	source := readExactMethodSource(t, "task_generation_queue_read_page.go", "func (p *taskGenerationQueueReadPagePhase) run(")

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
		"getCurrentListingKitResult(",
		"p.service.repo.GetTask(",
		"listAssetGenerationTasks(",
		"listGenerationReviews(",
		"withListingKitResultGenerationAndReview(",
		"buildGenerationQueueDeltaToken(",
		"listinggeneration.IsReadNotModified(",
		"applyGenerationConditionalStateToQueuePage(",
	})
}

func TestTaskGenerationQueueReadResponsePhaseBoundary(t *testing.T) {
	t.Parallel()

	source := readExactMethodSource(t, "task_generation_queue_read_response.go", "func (p *taskGenerationQueueReadResponsePhase) run(")

	assertSourceContainsAll(t, source, []string{
		"buildGenerationQueueDeltaToken(",
		"listinggeneration.IsReadNotModified(",
		"applyGenerationConditionalStateToQueuePage(",
	})
	assertSourceExcludesAll(t, source, []string{
		"getCurrentListingKitResult(",
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
