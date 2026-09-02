package listingkit

func (s *taskStudioBatchService) ensureDetailRunner() {
	if s == nil || s.detailRunner != nil {
		return
	}
	s.detailRunner = newListingStudioBatchDetailService(s.repo, s.studioSessionRepo, s.ensureStudioBatchGenerationGraphForResume)
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
	s.retryRunner = newListingStudioBatchRetryPrepareService(s.repo, s.GetStudioBatchDetail, s.resetStudioBatchRetryItems)
}
