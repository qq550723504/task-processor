package submission

import (
	"fmt"
	"strings"
)

type ResponseOutcome struct {
	Success         bool
	Code            string
	Message         string
	ValidationNotes []string
}

func SaveDraftSucceeded(action string, outcome *ResponseOutcome) bool {
	if action != "save_draft" || outcome == nil {
		return false
	}
	return outcome.Success || outcome.Code == "0"
}

func BuildResponseError(platform, action string, outcome *ResponseOutcome) error {
	if outcome == nil || outcome.Success || SaveDraftSucceeded(action, outcome) {
		return nil
	}
	if action != "publish" {
		return nil
	}

	platform = strings.TrimSpace(platform)
	if platform == "" {
		platform = "submission"
	}
	if len(outcome.ValidationNotes) > 0 {
		return fmt.Errorf("%s publish pre-validation failed: %s", platform, strings.Join(outcome.ValidationNotes, "; "))
	}
	message := strings.TrimSpace(outcome.Message)
	if message == "" {
		message = strings.TrimSpace(outcome.Code)
	}
	if message == "" {
		return fmt.Errorf("%s publish did not complete", platform)
	}
	return fmt.Errorf("%s publish did not complete: %s", platform, message)
}
