package listingkit

import (
	"context"
	"errors"
	"time"

	"task-processor/internal/listingkit/submission"
)

const sheinSubmitInFlightTTL = submission.InFlightTTL

var (
	errSheinSubmitReplayExisting = errors.New("shein submit replay existing")
	errSheinSubmitRecoverRemote  = errors.New("shein submit recover remote")
	errSheinSubmitMissingPackage = errors.New("shein submit missing package")
)

func (s *service) SubmitTask(ctx context.Context, taskID string, req *SubmitTaskRequest) (*ListingKitPreview, error) {
	return s.taskSubmissionOrDefault().SubmitTask(ctx, taskID, req)
}

func (s *service) acquireSheinSubmitTask(ctx context.Context, taskID, action, requestID string, startedAt time.Time) (*Task, *ListingKitPreview, error) {
	return s.taskSubmissionRecoveryOrDefault().acquireSheinSubmitTask(ctx, taskID, action, requestID, startedAt)
}

func (s *service) shouldStartSheinPublishWorkflow(platform, action string) bool {
	return s != nil &&
		s.sheinPublishWorkflowEnabled &&
		s.sheinPublishWorkflowClient != nil &&
		platform == "shein" &&
		action == "publish"
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
