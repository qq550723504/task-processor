package grsai

import "strings"

type JobError struct {
	Reason string
	Detail string
}

func (e *JobError) Error() string {
	if e == nil {
		return "grsai job failed"
	}
	reason := strings.TrimSpace(e.Reason)
	detail := strings.TrimSpace(e.Detail)
	switch {
	case reason == "" && detail == "":
		return "grsai job failed"
	case detail == "":
		return "grsai job failed: " + reason
	case reason == "":
		return "grsai job failed: " + detail
	default:
		return "grsai job failed: " + reason + " (" + detail + ")"
	}
}

func (e *JobError) FailureReason() string {
	if e == nil {
		return ""
	}
	return strings.TrimSpace(e.Reason)
}
