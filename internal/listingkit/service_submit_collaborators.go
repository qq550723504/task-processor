package listingkit

import "task-processor/internal/listingkit/submission"

func (s *service) taskRecoveryOrDefault() *taskRecoveryService {
	if s == nil {
		return nil
	}
	if s.submission.taskRecovery != nil {
		return s.submission.taskRecovery
	}
	s.submission.taskRecovery = newTaskRecoveryService(buildTaskRecoveryServiceConfig(s))
	return s.submission.taskRecovery
}

func (s *service) taskRequeueOrDefault() *taskRequeueService {
	if s == nil {
		return nil
	}
	if s.submission.taskRequeue != nil {
		return s.submission.taskRequeue
	}
	s.submission.taskRequeue = newTaskRequeueService(buildTaskRequeueServiceConfig(s))
	return s.submission.taskRequeue
}

func (s *service) taskSubmissionOrDefault() *taskSubmissionService {
	if s.submission.taskSubmission != nil {
		return s.submission.taskSubmission
	}
	if s.submission.sheinSubmitLocks == nil {
		s.submission.sheinSubmitLocks = submission.NewSubmitLockManager()
	}
	s.submission.taskSubmission = newTaskSubmissionService(buildTaskSubmissionServiceConfig(s))
	return s.submission.taskSubmission
}

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
	s.submission.taskSubmissionState = newTaskSubmissionStateService(buildTaskSubmissionStateServiceConfig(s))
	return s.submission.taskSubmissionState
}

func (s *service) taskSubmissionRecoveryOrDefault() *taskSubmissionRecoveryService {
	if s.submission.taskSubmissionRecovery != nil {
		return s.submission.taskSubmissionRecovery
	}
	s.submission.taskSubmissionRecovery = newTaskSubmissionRecoveryService(buildTaskSubmissionRecoveryServiceConfig(s))
	return s.submission.taskSubmissionRecovery
}

func (s *service) taskDirectSubmissionOrDefault() *taskDirectSubmissionService {
	if s.submission.taskDirectSubmission != nil {
		return s.submission.taskDirectSubmission
	}
	s.submission.taskDirectSubmission = newTaskDirectSubmissionService(buildTaskDirectSubmissionServiceConfig(s))
	return s.submission.taskDirectSubmission
}

func (s *service) taskTemporalSubmissionAdapterOrDefault() *taskTemporalSubmissionAdapter {
	if s.submission.taskTemporalSubmissionAdapter != nil {
		return s.submission.taskTemporalSubmissionAdapter
	}
	s.submission.taskTemporalSubmissionAdapter = newTaskTemporalSubmissionAdapter(buildTaskTemporalSubmissionAdapterConfig(s))
	return s.submission.taskTemporalSubmissionAdapter
}
