package submission

import (
	"fmt"
	"time"

	"task-processor/internal/listingkit/core"
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
		return core.ErrSubmitInProgress.Error()
	}
	if e.LeaseExpiresAt != nil {
		return fmt.Sprintf("%s: %s %s is in %s until %s", core.ErrSubmitInProgress, e.Platform, e.Action, e.Phase, e.LeaseExpiresAt.Format(time.RFC3339))
	}
	return fmt.Sprintf("%s: %s %s is in %s", core.ErrSubmitInProgress, e.Platform, e.Action, e.Phase)
}

func (e *SubmitInProgressError) Unwrap() error {
	return core.ErrSubmitInProgress
}
