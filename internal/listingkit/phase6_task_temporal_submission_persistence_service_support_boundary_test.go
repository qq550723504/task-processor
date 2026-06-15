package listingkit

import "testing"

func TestTaskTemporalSubmissionPersistenceServiceSupportBoundary(t *testing.T) {
	t.Parallel()

	rootSource := readTaskGenerationSourceFile(t, "task_temporal_submission_persistence_service.go")
	assertSourceContainsAll(t, rootSource, []string{
		"func newTaskTemporalSubmissionPersistenceService(",
		"func (s *taskTemporalSubmissionPersistenceService) PersistSheinPublishSuccess(",
		"func (s *taskTemporalSubmissionPersistenceService) PersistSheinPublishFailure(",
		"service.resultRunner = submissiondomain.NewResultPersistenceService(",
	})
	assertSourceExcludesAll(t, rootSource, []string{
		"func (s *taskTemporalSubmissionPersistenceService) loadSheinSubmitPersistenceState(",
		"func (s *taskTemporalSubmissionPersistenceService) loadSheinPublishTaskState(",
		"func (s *taskTemporalSubmissionPersistenceService) persistTemporalSuccessResultAndPhase(",
		"func (s *taskTemporalSubmissionPersistenceService) completeTemporalSubmitAttempt(",
		"func (s *taskTemporalSubmissionPersistenceService) recordTemporalFailureState(",
		"func (s *taskTemporalSubmissionPersistenceService) finishSheinTemporalRemoteRefreshSuccess(",
	})

	supportSource := readTaskGenerationSourceFile(t, "task_temporal_submission_persistence_service_support.go")
	assertSourceContainsAll(t, supportSource, []string{
		"func (s *taskTemporalSubmissionPersistenceService) loadSheinSubmitPersistenceState(",
		"func (s *taskTemporalSubmissionPersistenceService) loadSheinPublishTaskState(",
		"func (s *taskTemporalSubmissionPersistenceService) persistTemporalSuccessResultAndPhase(",
		"func (s *taskTemporalSubmissionPersistenceService) completeTemporalSubmitAttempt(",
		"func (s *taskTemporalSubmissionPersistenceService) recordTemporalFailureState(",
		"func (s *taskTemporalSubmissionPersistenceService) finishSheinTemporalRemoteRefreshSuccess(",
	})
	assertSourceExcludesAll(t, supportSource, []string{
		"func newTaskTemporalSubmissionPersistenceService(",
		"func (s *taskTemporalSubmissionPersistenceService) PersistSheinPublishSuccess(",
		"func (s *taskTemporalSubmissionPersistenceService) PersistSheinPublishFailure(",
		"service.resultRunner = submissiondomain.NewResultPersistenceService(",
	})
}
