package listingkit

import listingsubmission "task-processor/internal/listing/submission"

func (s *service) taskRecoveryOrDefault() *taskRecoveryService {
	s.ensureTaskSubmitTaskRecoveryCollaborators()
	return s.submission.taskRecovery
}

func (s *service) taskRequeueOrDefault() *taskRequeueService {
	s.ensureTaskSubmitTaskRecoveryCollaborators()
	return s.submission.taskRequeue
}

func (s *service) ensureTaskSubmitTaskRecoveryCollaborators() {
	if s == nil {
		return
	}
	wiring := buildTaskSubmitTaskRecoveryCollaboratorWiring(s)
	collaborators := wiring.resolve(s.submission)
	s.submission.taskRecovery = collaborators.taskRecovery
	s.submission.taskRequeue = collaborators.taskRequeue
}

func (s *service) taskSubmissionOrDefault() *taskSubmissionService {
	s.ensureTaskManagedSubmissionCollaborators()
	return s.submission.taskSubmission
}

func (s *service) taskSubmissionRefreshOrDefault() *taskSubmissionRefreshService {
	s.ensureTaskManagedSubmissionCollaborators()
	return s.submission.taskSubmissionRefresh
}

func (s *service) taskSubmissionExecutionOrDefault() *taskSubmissionExecutionService {
	s.ensureTaskSubmissionCoreCollaborators()
	return s.submission.taskSubmissionExecution
}

func (s *service) taskSubmissionStateOrDefault() *taskSubmissionStateService {
	s.ensureTaskSubmissionCoreCollaborators()
	return s.submission.taskSubmissionState
}

func (s *service) ensureTaskSubmissionCoreCollaborators() {
	if s == nil {
		return
	}
	wiring := buildTaskSubmissionCoreCollaboratorWiring(s)
	collaborators := wiring.resolve(s.submission)
	s.submission.taskSubmissionExecution = collaborators.execution
	s.submission.taskSubmissionState = collaborators.state
}

func (s *service) taskSubmissionRecoveryOrDefault() *taskSubmissionRecoveryService {
	s.ensureTaskManagedSubmissionCollaborators()
	return s.submission.taskSubmissionRecovery
}

func (s *service) taskDirectSubmissionOrDefault() *taskDirectSubmissionService {
	s.ensureTaskManagedSubmissionCollaborators()
	return s.submission.taskDirectSubmission
}

func (s *service) ensureTaskManagedSubmissionCollaborators() {
	if s == nil {
		return
	}
	if s.submission.sheinSubmitLocks == nil {
		s.submission.sheinSubmitLocks = listingsubmission.NewSubmitLockManager()
	}
	wiring := buildTaskManagedSubmissionCollaboratorWiring(s)
	collaborators := wiring.resolve(s.submission)
	s.submission.taskSubmissionRecovery = collaborators.recovery
	s.submission.taskDirectSubmission = collaborators.direct
	s.submission.taskSubmissionRefresh = collaborators.refresh
	s.submission.taskSubmission = collaborators.submission
}

func (s *service) taskTemporalSubmissionPersistenceOrDefault() *taskTemporalSubmissionPersistenceService {
	s.ensureTaskTemporalSubmissionCollaborators()
	return s.submission.taskTemporalSubmissionPersistence
}

func (s *service) ensureTaskTemporalSubmissionCollaborators() {
	if s == nil {
		return
	}
	wiring := buildTaskTemporalSubmissionCollaboratorWiring(s)
	collaborators := wiring.resolve(s.submission)
	s.submission.taskTemporalSubmissionPersistence = collaborators.persistence
	s.submission.taskTemporalSubmissionLifecycle = collaborators.lifecycle
	s.submission.taskTemporalSubmissionFlow = collaborators.flow
	s.submission.taskTemporalSubmissionRefresh = collaborators.refresh
	s.submission.taskTemporalSubmission = collaborators.facade
}

func (s *service) taskTemporalSubmissionOrDefault() *taskTemporalSubmissionService {
	s.ensureTaskTemporalSubmissionCollaborators()
	return s.submission.taskTemporalSubmission
}

func (s *service) taskTemporalSubmissionLifecycleOrDefault() *taskTemporalSubmissionLifecycleService {
	s.ensureTaskTemporalSubmissionCollaborators()
	return s.submission.taskTemporalSubmissionLifecycle
}

func (s *service) taskTemporalSubmissionFlowOrDefault() *taskTemporalSubmissionFlowService {
	s.ensureTaskTemporalSubmissionCollaborators()
	return s.submission.taskTemporalSubmissionFlow
}

func (s *service) taskTemporalSubmissionRefreshOrDefault() *taskTemporalSubmissionRefreshService {
	s.ensureTaskTemporalSubmissionCollaborators()
	return s.submission.taskTemporalSubmissionRefresh
}
