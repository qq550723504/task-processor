package listingkit

import (
	"context"
	"strings"
	"time"

	apperrors "task-processor/internal/core/errors"
	listingsubmission "task-processor/internal/listing/submission"

	"github.com/sirupsen/logrus"
)

type taskSubmissionServiceConfig struct {
	repo                            Repository
	lockSubmit                      func(key string) func()
	resolveDefaultSheinSubmitAction func(context.Context, string) (string, error)
	recovery                        *taskSubmissionRecoveryService
	shouldStartSheinPublishWorkflow func(platform, action string) bool
	submitSheinTaskWithWorkflow     func(context.Context, string, *Task, *SubmitTaskRequest, sheinWorkflowSubmitOptions) (*ListingKitPreview, error)
	submitSheinTaskDirect           func(context.Context, string, *Task, *SubmitTaskRequest, sheinDirectSubmitOptions) (*ListingKitPreview, error)
}

type taskSubmissionService struct {
	repo                            Repository
	lockSubmit                      func(key string) func()
	resolveDefaultSheinSubmitAction func(context.Context, string) (string, error)
	recovery                        *taskSubmissionRecoveryService
	shouldStartSheinPublishWorkflow func(platform, action string) bool
	submitSheinTaskWithWorkflow     func(context.Context, string, *Task, *SubmitTaskRequest, sheinWorkflowSubmitOptions) (*ListingKitPreview, error)
	submitSheinTaskDirect           func(context.Context, string, *Task, *SubmitTaskRequest, sheinDirectSubmitOptions) (*ListingKitPreview, error)
}

type sheinSubmissionAttemptState struct {
	task        *Task
	platform    string
	action      string
	requestID   string
	startedAt   time.Time
	useWorkflow bool
}

func newTaskSubmissionService(config taskSubmissionServiceConfig) *taskSubmissionService {
	return &taskSubmissionService{
		repo:                            config.repo,
		lockSubmit:                      config.lockSubmit,
		resolveDefaultSheinSubmitAction: config.resolveDefaultSheinSubmitAction,
		recovery:                        config.recovery,
		shouldStartSheinPublishWorkflow: config.shouldStartSheinPublishWorkflow,
		submitSheinTaskWithWorkflow:     config.submitSheinTaskWithWorkflow,
		submitSheinTaskDirect:           config.submitSheinTaskDirect,
	}
}

func (s *taskSubmissionService) SubmitTask(ctx context.Context, taskID string, req *SubmitTaskRequest) (*ListingKitPreview, error) {
	attempt, preview, err := s.buildSheinSubmissionAttemptState(ctx, taskID, req)
	if preview != nil || err != nil {
		return preview, err
	}
	if s.lockSubmit != nil {
		unlockSubmit := s.lockSubmit(taskID + ":" + attempt.action)
		defer unlockSubmit()
	}
	return s.executeSheinSubmissionAttempt(ctx, taskID, req, attempt)
}

func (s *taskSubmissionService) buildSheinSubmissionAttemptState(ctx context.Context, taskID string, req *SubmitTaskRequest) (*sheinSubmissionAttemptState, *ListingKitPreview, error) {
	platform, action, err := s.normalizeSubmitTarget(ctx, taskID, req)
	if err != nil {
		return nil, nil, err
	}

	attempt := s.buildSubmissionAttemptState(taskID, platform, action, req)
	logSheinSubmissionRequested(taskID, attempt.platform, attempt.action, attempt.requestID, attempt.useWorkflow, req)

	task, preview, err := s.acquireSubmitTask(ctx, taskID, attempt.action, attempt.requestID, attempt.startedAt)
	if preview != nil || err != nil {
		return nil, preview, err
	}
	attempt.task = task
	return attempt, nil, nil
}

func (s *taskSubmissionService) buildSubmissionAttemptState(taskID, platform, action string, req *SubmitTaskRequest) *sheinSubmissionAttemptState {
	startedAt := time.Now()
	return buildSubmissionAttemptState(taskID, platform, action, req, startedAt, s.shouldStartSheinPublishWorkflow)
}

func buildSubmissionAttemptState(taskID, platform, action string, req *SubmitTaskRequest, startedAt time.Time, shouldStartWorkflow func(string, string) bool) *sheinSubmissionAttemptState {
	idempotencyKey := ""
	explicitRequestID := ""
	if req != nil {
		idempotencyKey = req.IdempotencyKey
		explicitRequestID = req.RequestID
	}
	plan := listingsubmission.BuildSubmitAttemptPlan(taskID, platform, action, idempotencyKey, explicitRequestID, startedAt, shouldStartWorkflow)
	return &sheinSubmissionAttemptState{
		platform:    plan.Platform,
		action:      plan.Action,
		requestID:   plan.RequestID,
		startedAt:   plan.StartedAt,
		useWorkflow: plan.UseWorkflow,
	}
}

func logSheinSubmissionRequested(taskID, platform, action, requestID string, useWorkflow bool, req *SubmitTaskRequest) {
	logrus.WithFields(logrus.Fields{
		"task_id":         strings.TrimSpace(taskID),
		"platform":        platform,
		"action":          action,
		"request_id":      requestID,
		"use_workflow":    useWorkflow,
		"confirmed_final": req != nil && req.ConfirmedFinal,
	}).Info("listingkit shein submit requested")
}

func (s *taskSubmissionService) executeSheinSubmissionAttempt(ctx context.Context, taskID string, req *SubmitTaskRequest, attempt *sheinSubmissionAttemptState) (*ListingKitPreview, error) {
	if attempt == nil {
		return nil, apperrors.New(apperrors.ErrCodeSystem, "submit attempt state is not available")
	}
	if attempt.useWorkflow {
		return s.submitSheinTaskWithWorkflow(ctx, taskID, attempt.task, req, buildWorkflowSubmitOptions(attempt))
	}
	return s.submitSheinTaskDirect(ctx, taskID, attempt.task, req, buildDirectSubmitOptions(attempt))
}

func buildWorkflowSubmitOptions(attempt *sheinSubmissionAttemptState) sheinWorkflowSubmitOptions {
	if attempt == nil {
		return sheinWorkflowSubmitOptions{}
	}
	return sheinWorkflowSubmitOptions{
		platform:  attempt.platform,
		action:    attempt.action,
		requestID: attempt.requestID,
		startedAt: attempt.startedAt,
	}
}

func buildDirectSubmitOptions(attempt *sheinSubmissionAttemptState) sheinDirectSubmitOptions {
	if attempt == nil {
		return sheinDirectSubmitOptions{}
	}
	return sheinDirectSubmitOptions{
		action:    attempt.action,
		requestID: attempt.requestID,
		startedAt: attempt.startedAt,
	}
}

func (s *taskSubmissionService) normalizeSubmitTarget(ctx context.Context, taskID string, req *SubmitTaskRequest) (platform string, action string, err error) {
	defaultAction := ""
	if req == nil || strings.TrimSpace(req.Action) == "" {
		if s.resolveDefaultSheinSubmitAction != nil {
			defaultAction, err = s.resolveDefaultSheinSubmitAction(ctx, taskID)
			if err != nil {
				return "", "", err
			}
		}
	}
	return normalizeSubmitTargetWithDefault(req, defaultAction)
}

func (s *taskSubmissionService) acquireSubmitTask(ctx context.Context, taskID, action, requestID string, startedAt time.Time) (*Task, *ListingKitPreview, error) {
	if s.recovery == nil {
		return nil, nil, apperrors.New(apperrors.ErrCodeSystem, "submit task acquisition is not configured")
	}
	return s.recovery.acquireSheinSubmitTask(ctx, taskID, action, requestID, startedAt)
}
