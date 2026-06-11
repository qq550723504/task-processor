package listingkit

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"task-processor/internal/listingkit/submission"
)

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

func shouldReplayStartedTemporalSubmit(err error, requestID string) bool {
	var inProgress *submission.SubmitInProgressError
	return errors.As(err, &inProgress) &&
		inProgress != nil &&
		strings.TrimSpace(inProgress.RequestID) != "" &&
		inProgress.RequestID == strings.TrimSpace(requestID)
}
