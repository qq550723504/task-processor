package listingkit

import (
	"fmt"
	"time"

	listingsubmission "task-processor/internal/listing/submission"
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
	requestedPlatform := ""
	requestedAction := ""
	if req != nil {
		requestedPlatform = req.Platform
		requestedAction = req.Action
	}
	target := listingsubmission.ResolveSubmitTarget(requestedPlatform, requestedAction, "shein", defaultAction)
	platform = target.Platform
	action = target.Action
	if platform != "shein" {
		return "", "", fmt.Errorf("%w: %s", ErrUnsupportedSubmitPlatform, platform)
	}
	if !listingsubmission.IsSupportedSubmitAction(action) {
		return "", "", listingsubmission.UnsupportedSubmitActionError(action)
	}
	return platform, action, nil
}
