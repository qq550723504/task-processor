package submission

import (
	"errors"
	"strings"
)

type SubmitTarget struct {
	Platform string
	Action   string
}

func ResolveSubmitTarget(requestedPlatform, requestedAction, defaultPlatform, defaultAction string) SubmitTarget {
	platform := strings.ToLower(strings.TrimSpace(defaultPlatform))
	if platform == "" {
		platform = "shein"
	}
	action := strings.ToLower(strings.TrimSpace(defaultAction))
	if action == "" {
		action = "publish"
	}
	if value := strings.ToLower(strings.TrimSpace(requestedPlatform)); value != "" {
		platform = value
	}
	if value := strings.ToLower(strings.TrimSpace(requestedAction)); value != "" {
		action = value
	}
	return SubmitTarget{
		Platform: platform,
		Action:   action,
	}
}

func IsReplayOfStartedSubmit(err error, requestID string) bool {
	var inProgress *SubmitInProgressError
	return errors.As(err, &inProgress) &&
		inProgress != nil &&
		strings.TrimSpace(inProgress.RequestID) != "" &&
		inProgress.RequestID == strings.TrimSpace(requestID)
}
