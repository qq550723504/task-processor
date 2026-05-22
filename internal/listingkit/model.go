package listingkit

import "errors"

var ErrTaskNotFound = errors.New("task not found")
var ErrTaskNotPending = errors.New("task is not pending")
var ErrGenerationTaskNotFound = errors.New("generation task not found")
var ErrGenerationTaskNotRetryable = errors.New("generation task is not retryable")
var ErrGenerationActionNotFound = errors.New("generation action not found")
var ErrChildTaskRetryInvalidRequest = errors.New("child task retry invalid request")
var ErrChildTaskNotFound = errors.New("child task not found")
var ErrChildTaskNotRetryable = errors.New("child task is not retryable")
var ErrChildTaskRetryConflict = errors.New("child task retry conflict")
var ErrUnsupportedSubmitPlatform = errors.New("unsupported submit platform")
var ErrSubmitBlocked = errors.New("submit blocked by readiness")
var ErrSubmitInProgress = errors.New("submit already in progress")
var ErrInvalidSheinResolutionCacheKind = errors.New("invalid shein resolution cache kind")
var ErrInvalidSheinCategorySearchQuery = errors.New("invalid shein category search query")

type TaskStatus string

const (
	TaskStatusPending     TaskStatus = "pending"
	TaskStatusProcessing  TaskStatus = "processing"
	TaskStatusCompleted   TaskStatus = "completed"
	TaskStatusNeedsReview TaskStatus = "needs_review"
	TaskStatusFailed      TaskStatus = "failed"
)
