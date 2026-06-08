package listingkit

import (
	"task-processor/internal/listingkit/core"
	"task-processor/internal/listingkit/submission"
)

// SubmitInProgressError is aliased from submission package for backward compatibility.
type SubmitInProgressError = submission.SubmitInProgressError

// ErrSubmitInProgress is re-exported from core package for backward compatibility.
var ErrSubmitInProgress = core.ErrSubmitInProgress
