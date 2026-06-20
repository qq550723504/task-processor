package listingkit

import "context"

func (s *taskStudioBatchService) ensureDetailRunner() {
	if s == nil || s.detailRunner != nil {
		return
	}
	s.detailRunner = newListingStudioBatchDetailService(s.repo, s.studioSessionRepo, s.batchTaskLinkRepo, s.getTask, s.ensureStudioBatchGenerationGraphForResume)
}

func (s *taskStudioBatchService) ensureServiceRunner() {
	if s == nil || s.serviceRunner != nil {
		return
	}
	s.serviceRunner = newListingStudioBatchServiceRunner(s)
}

func (s *taskStudioBatchService) ensureBatchRunner() {
	if s == nil || s.batchRunner != nil {
		return
	}
	s.batchRunner = newListingStudioBatchGenerationService(s)
}

func (s *taskStudioBatchService) ensureReviewRunner() {
	if s == nil || s.reviewRunner != nil {
		return
	}
	s.reviewRunner = newListingStudioBatchReviewService(s.repo, s.GetStudioBatchDetail, s.currentTime)
}

func (s *taskStudioBatchService) ensureRetryRunner() {
	if s == nil || s.retryRunner != nil {
		return
	}
	s.retryRunner = newListingStudioBatchRetryPrepareService(s.repo, s.batchTaskLinkRepo, s.GetStudioBatchDetail, s.resetStudioBatchRetryItems)
}

func (s *taskStudioBatchService) ensureTaskCreationRunner() {
	if s == nil || s.taskCreationRunner != nil {
		return
	}
	s.taskCreationRunner = newListingStudioBatchTaskCreationService(s)
}

func (s *taskStudioBatchService) ensureTaskExecuteRunner() {
	if s == nil || s.taskExecuteRunner != nil {
		return
	}
	s.taskExecuteRunner = newListingStudioBatchTaskExecuteService(s)
}

func (s *taskStudioBatchService) ensureTaskPrepareRunner() {
	if s == nil || s.taskPrepareRunner != nil {
		return
	}
	s.taskPrepareRunner = newListingStudioBatchTaskPrepareService(
		s.studioBatchSessionUpdater(),
		s.studioBatchUpdater(),
		s.loadStudioBatchTaskPreparationResult,
		s.currentTime,
	)
}

func (s *taskStudioBatchService) ensureTaskResumeRunner() {
	if s == nil || s.taskResumeRunner != nil {
		return
	}
	s.taskResumeRunner = newListingStudioBatchTaskResumeService(
		s.studioBatchSessionUpdater(),
		s.studioBatchUpdater(),
		s.loadStudioBatchTaskPreparationResult,
		s.currentTime,
	)
}

func (s *taskStudioBatchService) studioBatchSessionUpdater() func(context.Context, *SheinStudioSession) error {
	if s == nil {
		return nil
	}
	if sessionUpdater, ok := s.studioSessionRepo.(interface {
		UpdateSession(context.Context, *SheinStudioSession) error
	}); ok {
		return sessionUpdater.UpdateSession
	}
	return nil
}

func (s *taskStudioBatchService) studioBatchUpdater() func(context.Context, *StudioBatchRecord) error {
	if s == nil || s.repo == nil {
		return nil
	}
	return s.repo.UpdateStudioBatch
}
