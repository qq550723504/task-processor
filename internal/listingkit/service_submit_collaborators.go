package listingkit

func (s *service) taskSubmissionRefreshOrDefault() *taskSubmissionRefreshService {
	if s.submission.taskSubmissionRefresh != nil {
		return s.submission.taskSubmissionRefresh
	}
	s.submission.taskSubmissionRefresh = newTaskSubmissionRefreshService(buildTaskSubmissionRefreshServiceConfig(s))
	return s.submission.taskSubmissionRefresh
}

func (s *service) taskSubmissionExecutionOrDefault() *taskSubmissionExecutionService {
	if s.submission.taskSubmissionExecution != nil {
		return s.submission.taskSubmissionExecution
	}
	s.submission.taskSubmissionExecution = newTaskSubmissionExecutionService(buildTaskSubmissionExecutionServiceConfig(s))
	return s.submission.taskSubmissionExecution
}

func (s *service) taskSubmissionStateOrDefault() *taskSubmissionStateService {
	if s.submission.taskSubmissionState != nil {
		return s.submission.taskSubmissionState
	}
	s.submission.taskSubmissionState = newTaskSubmissionStateService(taskSubmissionStateServiceConfig{
		repo:                   s.repo,
		rememberSheinSubmitted: s.rememberSheinSubmittedResolution,
	})
	return s.submission.taskSubmissionState
}
