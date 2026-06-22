package listingkit

import (
	"time"

	sheinpub "task-processor/internal/publishing/shein"
)

func beginSheinSubmitAttempt(pkg *SheinPackage, action, requestID, phase string, startedAt time.Time) *sheinpub.SubmissionRecord {
	return sheinpub.BeginSubmitAttempt(pkg, action, requestID, phase, startedAt, sheinSubmitInFlightTTL)
}
