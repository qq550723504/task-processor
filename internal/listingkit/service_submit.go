package listingkit

import (
	"context"
	"errors"
	"fmt"
	"strings"
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

type sheinWorkflowSubmitOptions struct {
	platform  string
	action    string
	requestID string
	startedAt time.Time
}

type sheinDirectSubmitOptions struct {
	action    string
	requestID string
	startedAt time.Time
}

func normalizeSubmitTarget(req *SubmitTaskRequest) (platform string, action string, err error) {
	return normalizeSubmitTargetWithDefault(req, "")
}

func normalizeSubmitTargetWithDefault(req *SubmitTaskRequest, defaultAction string) (platform string, action string, err error) {
	platform = "shein"
	action = "publish"
	if value := strings.ToLower(strings.TrimSpace(defaultAction)); value != "" {
		action = value
	}
	if req != nil {
		if value := strings.ToLower(strings.TrimSpace(req.Platform)); value != "" {
			platform = value
		}
		if value := strings.ToLower(strings.TrimSpace(req.Action)); value != "" {
			action = value
		}
	}
	if platform != "shein" {
		return "", "", fmt.Errorf("%w: %s", ErrUnsupportedSubmitPlatform, platform)
	}
	if !isSupportedSubmitAction(action) {
		return "", "", unsupportedSubmitActionError(action)
	}
	return platform, action, nil
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

func shouldReplayStartedTemporalSubmit(err error, requestID string) bool {
	var inProgress *submission.SubmitInProgressError
	return errors.As(err, &inProgress) &&
		inProgress != nil &&
		strings.TrimSpace(inProgress.RequestID) != "" &&
		inProgress.RequestID == strings.TrimSpace(requestID)
}
