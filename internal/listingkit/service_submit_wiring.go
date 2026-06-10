package listingkit

import (
	"context"
	"fmt"

	sheinpub "task-processor/internal/publishing/shein"
)

func buildTaskRequeueServiceConfig(s *service) taskRequeueServiceConfig {
	return taskRequeueServiceConfig{
		repo: s.repo,
		taskSubmitter: func() TaskSubmitter {
			return s.taskSubmitter
		},
	}
}

func buildTaskRecoveryServiceConfig(s *service) taskRecoveryServiceConfig {
	return taskRecoveryServiceConfig{
		repo: s.repo,
		taskSubmitter: func() TaskSubmitter {
			return s.taskSubmitter
		},
	}
}

func buildTaskSubmissionRecoveryServiceConfig(s *service) taskSubmissionRecoveryServiceConfig {
	state := s.taskSubmissionStateOrDefault()
	execution := s.taskSubmissionExecutionOrDefault()
	return taskSubmissionRecoveryServiceConfig{
		repo:                        s.repo,
		buildTaskPreview:            s.buildTaskPreview,
		buildSheinSubmitProductAPI:  execution.buildSheinSubmitProductAPI,
		buildSheinSubmitOtherAPI:    s.buildSheinSubmitOtherAPI,
		rememberSheinSubmitted:      s.rememberSheinSubmittedResolution,
		persistSuccessfulSubmission: state.persistSuccessfulSheinSubmission,
		recordSubmissionFailure:     state.recordSheinSubmissionFailureForState,
		resolveRemoteStatusCallback: s.resolveSheinSubmitRemoteStatus,
	}
}

func buildTaskSubmissionServiceConfig(s *service) taskSubmissionServiceConfig {
	execution := s.taskSubmissionExecutionOrDefault()
	direct := s.taskDirectSubmissionOrDefault()
	recovery := s.taskSubmissionRecoveryOrDefault()
	return taskSubmissionServiceConfig{
		repo: s.repo,
		lockSubmit: func(key string) func() {
			return s.submission.sheinSubmitLocks.Lock(key)
		},
		resolveDefaultSheinSubmitAction: s.resolveDefaultSheinSubmitAction,
		acquireSheinSubmitTask:          s.acquireSheinSubmitTask,
		shouldStartSheinPublishWorkflow: s.shouldStartSheinPublishWorkflow,
		submitSheinTaskWithWorkflow:     s.submitSheinTaskWithWorkflow,
		submitSheinTaskDirect:           direct.submitSheinTaskDirect,
		buildTaskPreview:                s.buildTaskPreview,
		buildSheinSubmitProductAPI:      execution.buildSheinSubmitProductAPI,
		buildSheinSubmitOtherAPI:        s.buildSheinSubmitOtherAPI,
		mutateTaskResult:                recovery.mutateTaskResult,
		resolveRemoteStatus:             recovery.resolveSheinSubmitRemoteStatus,
	}
}

func buildTaskDirectSubmissionServiceConfig(s *service) taskDirectSubmissionServiceConfig {
	state := s.taskSubmissionStateOrDefault()
	execution := s.taskSubmissionExecutionOrDefault()
	return taskDirectSubmissionServiceConfig{
		normalizeSheinSubmitPackage:     execution.normalizeSheinSubmitPackage,
		validateSheinPublishFreshness:   s.validateSheinPublishFreshness,
		failSheinDirectSubmit:           state.failSheinDirectSubmit,
		buildSheinSubmitProductAPI:      execution.buildSheinSubmitProductAPI,
		persistSheinDirectSubmitPhase:   state.persistSheinDirectSubmitPhase,
		prepareSheinDirectSubmitProduct: s.prepareSheinDirectSubmitProduct,
		completeSheinDirectRemoteSubmit: s.completeSheinDirectRemoteSubmit,
		buildTaskPreview:                s.buildTaskPreview,
	}
}

func buildTaskSubmissionExecutionServiceConfig(s *service) taskSubmissionExecutionServiceConfig {
	resolver := buildSubmitRuntimeContextResolver(s)
	return taskSubmissionExecutionServiceConfig{
		sheinProductAPIBuilder:   s.sheinProductAPIBuilder,
		sheinImageAPIBuilder:     s.sheinImageAPIBuilder,
		sheinTranslateAPIBuilder: s.sheinTranslateAPIBuilder,
		sheinContentOptimizer:    s.sheinContentOptimizer,
		currentSheinPricingRule:  s.currentSheinPricingRule,
		resolveSheinStoreID:      resolver.resolveStoreID,
		resolveSubmitSettings:    resolver.resolveSubmitSettings,
	}
}

func buildTaskTemporalSubmissionAdapterConfig(s *service) taskTemporalSubmissionAdapterConfig {
	resolver := buildSubmitRuntimeContextResolver(s)
	state := s.taskSubmissionStateOrDefault()
	execution := s.taskSubmissionExecutionOrDefault()
	recovery := s.taskSubmissionRecoveryOrDefault()
	return taskTemporalSubmissionAdapterConfig{
		startSheinPublishWorkflow: func(ctx context.Context, in SheinPublishWorkflowStartInput) error {
			return s.sheinPublishWorkflowClient.StartSheinPublish(ctx, in)
		},
		beginSheinSubmitLease: recovery.beginSheinSubmitLease,
		loadSheinPublishTask: func(ctx context.Context, taskID string) (*Task, *SheinPackage, error) {
			task, err := s.repo.GetTask(ctx, taskID)
			if err != nil {
				return nil, nil, err
			}
			if task.Result == nil {
				return nil, nil, ErrTaskResultUnavailable
			}
			pkg := sheinpub.NormalizePackageSemanticFields(task.Result.Shein)
			if pkg == nil || pkg.PreviewPayload == nil {
				return nil, nil, fmt.Errorf("%w: shein preview payload is not available", ErrSubmitBlocked)
			}
			return task, pkg, nil
		},
		normalizeSheinSubmitPackage:          execution.normalizeSheinSubmitPackage,
		validateSheinPublishFreshness:        s.validateSheinPublishFreshness,
		saveTaskResult:                       s.repo.SaveTaskResult,
		persistSheinSubmitPhase:              state.persistSheinSubmitPhase,
		prepareSheinSubmitProduct:            execution.prepareSheinSubmitProduct,
		uploadSheinSubmitImages:              execution.uploadSheinSubmitImages,
		resolveSubmitSettings:                resolver.resolveSubmitSettings,
		buildSheinSubmitProductAPI:           execution.buildSheinSubmitProductAPI,
		preValidateSheinSubmitProduct:        execution.preValidateSheinSubmitProduct,
		executeSheinSubmitRemote:             execution.executeSheinSubmitRemote,
		retrySheinSensitiveWordSubmit:        s.retrySheinSensitiveWordSubmit,
		persistSuccessfulSheinSubmission:     state.persistSuccessfulSheinSubmission,
		recordSheinSubmissionFailureForState: state.recordSheinSubmissionFailureForState,
		refreshSheinSubmitRemoteStatus:       recovery.refreshSheinSubmitRemoteStatus,
		handleWorkflowStartFailure:           recovery.handleSheinWorkflowStartFailure,
		rememberSheinSubmitted:               s.rememberSheinSubmittedResolution,
		getTaskPreview:                       s.GetTaskPreview,
	}
}
