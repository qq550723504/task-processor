package listingkit

import (
	"fmt"
	"time"
)

type SubmitInProgressError struct {
	Platform       string
	Action         string
	Phase          string
	RequestID      string
	LeaseExpiresAt *time.Time
}

func (e *SubmitInProgressError) Error() string {
	if e == nil {
		return ErrSubmitInProgress.Error()
	}
	if e.LeaseExpiresAt != nil {
		return fmt.Sprintf("%s: %s %s is in %s until %s", ErrSubmitInProgress, e.Platform, e.Action, e.Phase, e.LeaseExpiresAt.Format(time.RFC3339))
	}
	return fmt.Sprintf("%s: %s %s is in %s", ErrSubmitInProgress, e.Platform, e.Action, e.Phase)
}

func (e *SubmitInProgressError) Unwrap() error {
	return ErrSubmitInProgress
}
