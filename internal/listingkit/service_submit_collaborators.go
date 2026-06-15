package listingkit

import listingsubmission "task-processor/internal/listing/submission"

func (s *service) taskRecoveryOrDefault() *taskRecoveryService {
	return s.resolveTaskSubmitTaskRecoveryCollaborators().taskRecovery
}

func (s *service) taskRequeueOrDefault() *taskRequeueService {
	return s.resolveTaskSubmitTaskRecoveryCollaborators().taskRequeue
}

func (s *service) resolveTaskSubmitTaskRecoveryCollaborators() taskSubmitTaskRecoveryCollaborators {
	if s == nil {
		return taskSubmitTaskRecoveryCollaborators{}
	}
	wiring := buildTaskSubmitTaskRecoveryCollaboratorWiring(s)
	s.submission.recoveryGroup = wiring.resolve(s.submission.recoveryGroup)
	return s.submission.recoveryGroup
}

func (s *service) taskSubmissionOrDefault() *taskSubmissionService {
	return s.resolveTaskManagedSubmissionCollaborators().submission
}

func (s *service) taskSubmissionRefreshOrDefault() *taskSubmissionRefreshService {
	return s.resolveTaskManagedSubmissionCollaborators().refresh
}

func (s *service) taskSubmissionExecutionOrDefault() *taskSubmissionExecutionService {
	return s.resolveTaskSubmissionCoreCollaborators().execution
}

func (s *service) taskSubmissionStateOrDefault() *taskSubmissionStateService {
	return s.resolveTaskSubmissionCoreCollaborators().state
}

func (s *service) resolveTaskSubmissionCoreCollaborators() taskSubmissionCoreCollaborators {
	if s == nil {
		return taskSubmissionCoreCollaborators{}
	}
	wiring := buildTaskSubmissionCoreCollaboratorWiring(s)
	s.submission.coreGroup = wiring.resolve(s.submission.coreGroup)
	return s.submission.coreGroup
}

func (s *service) taskSubmissionRecoveryOrDefault() *taskSubmissionRecoveryService {
	return s.resolveTaskManagedSubmissionCollaborators().recovery
}

func (s *service) taskDirectSubmissionOrDefault() *taskDirectSubmissionService {
	return s.resolveTaskManagedSubmissionCollaborators().direct
}

func (s *service) resolveTaskManagedSubmissionCollaborators() taskManagedSubmissionCollaborators {
	if s == nil {
		return taskManagedSubmissionCollaborators{}
	}
	if s.submission.sheinSubmitLocks == nil {
		s.submission.sheinSubmitLocks = listingsubmission.NewSubmitLockManager()
	}
	wiring := buildTaskManagedSubmissionCollaboratorWiring(s)
	s.submission.managedGroup = wiring.resolve(s.submission.managedGroup)
	return s.submission.managedGroup
}

func (s *service) taskTemporalSubmissionPersistenceOrDefault() *taskTemporalSubmissionPersistenceService {
	return s.resolveTaskTemporalSubmissionCollaborators().persistence
}

func (s *service) resolveTaskTemporalSubmissionCollaborators() taskTemporalSubmissionCollaborators {
	if s == nil {
		return taskTemporalSubmissionCollaborators{}
	}
	wiring := buildTaskTemporalSubmissionCollaboratorWiring(s)
	s.submission.temporalGroup = wiring.resolve(s.submission.temporalGroup)
	return s.submission.temporalGroup
}

func (s *service) taskTemporalSubmissionLifecycleOrDefault() *taskTemporalSubmissionLifecycleService {
	return s.resolveTaskTemporalSubmissionCollaborators().lifecycle
}

func (s *service) taskTemporalSubmissionFlowOrDefault() *taskTemporalSubmissionFlowService {
	return s.resolveTaskTemporalSubmissionCollaborators().flow
}

func (s *service) taskTemporalSubmissionRefreshOrDefault() *taskTemporalSubmissionRefreshService {
	return s.resolveTaskTemporalSubmissionCollaborators().refresh
}
