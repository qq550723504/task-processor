package nanobanana

import "strings"

type JobError struct {
	Reason string
	Detail string
}

func (e *JobError) Error() string {
	if e == nil {
		return "nanobanana job failed"
	}
	reason := strings.TrimSpace(e.Reason)
	detail := strings.TrimSpace(e.Detail)
	switch {
	case reason == "" && detail == "":
		return "nanobanana job failed"
	case detail == "":
		return "nanobanana job failed: " + reason
	case reason == "":
		return "nanobanana job failed: " + detail
	default:
		return "nanobanana job failed: " + reason + " (" + detail + ")"
	}
}

func (e *JobError) FailureReason() string {
	if e == nil {
		return ""
	}
	return strings.TrimSpace(e.Reason)
}
