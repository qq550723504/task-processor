package submission

import (
	"errors"
	"fmt"
	"strings"
	"time"
)

const (
	SubmitActionSaveDraft = "save_draft"
	SubmitActionPublish   = "publish"

	SubmitStatusPending    = "pending"
	SubmitStatusRunning    = "running"
	SubmitStatusSucceeded  = "succeeded"
	SubmitStatusFailed     = "failed"
	SubmitStatusRecovering = "recovering"

	SubmitPhaseValidate       = "validate"
	SubmitPhasePrepareProduct = "prepare_product"
	SubmitPhaseUploadImages   = "upload_images"
	SubmitPhasePreValidate    = "pre_validate"
	SubmitPhaseSubmitRemote   = "submit_remote"
	SubmitPhasePersistResult  = "persist_result"
)

type SubmitAttempt struct {
	AttemptID      string
	TaskID         string
	TenantID       string
	TargetPlatform string
	Action         string
	Status         string
	Phase          string
	IdempotencyKey string
	RemoteRecord   SubmitRemoteRecord
	Error          *SubmitErrorDetail
	CreatedAt      time.Time
	UpdatedAt      time.Time
	FinishedAt     *time.Time
}

type SubmitRemoteRecord struct {
	ProductID string
	DraftID   string
	PublishID string
}

type SubmitErrorDetail struct {
	Code        string
	Message     string
	Recoverable bool
}

type SubmitAttemptValidationError struct {
	Field string
}

func (e *SubmitAttemptValidationError) Error() string {
	if e == nil {
		return "invalid submit attempt"
	}
	return fmt.Sprintf("invalid submit attempt: %s is required or unsupported", e.Field)
}

func AsSubmitAttemptValidationError(err error, target **SubmitAttemptValidationError) bool {
	return errors.As(err, target)
}

func NormalizeSubmitAttempt(attempt SubmitAttempt) SubmitAttempt {
	attempt.AttemptID = strings.TrimSpace(attempt.AttemptID)
	attempt.TaskID = strings.TrimSpace(attempt.TaskID)
	attempt.TenantID = strings.TrimSpace(attempt.TenantID)
	attempt.TargetPlatform = strings.ToLower(strings.TrimSpace(attempt.TargetPlatform))
	attempt.Action = NormalizeSubmitAction(attempt.Action, "")
	attempt.Status = strings.ToLower(strings.TrimSpace(attempt.Status))
	attempt.Phase = strings.ToLower(strings.TrimSpace(attempt.Phase))
	attempt.IdempotencyKey = strings.TrimSpace(attempt.IdempotencyKey)
	attempt.RemoteRecord = SubmitRemoteRecord{
		ProductID: strings.TrimSpace(attempt.RemoteRecord.ProductID),
		DraftID:   strings.TrimSpace(attempt.RemoteRecord.DraftID),
		PublishID: strings.TrimSpace(attempt.RemoteRecord.PublishID),
	}
	if attempt.Error != nil {
		attempt.Error = &SubmitErrorDetail{
			Code:        strings.TrimSpace(attempt.Error.Code),
			Message:     strings.TrimSpace(attempt.Error.Message),
			Recoverable: attempt.Error.Recoverable,
		}
	}
	return attempt
}

func ValidateSubmitAttempt(attempt SubmitAttempt) error {
	attempt = NormalizeSubmitAttempt(attempt)
	switch {
	case attempt.TaskID == "":
		return &SubmitAttemptValidationError{Field: "task_id"}
	case attempt.TargetPlatform == "":
		return &SubmitAttemptValidationError{Field: "target_platform"}
	case !IsSupportedSubmitAction(attempt.Action):
		return &SubmitAttemptValidationError{Field: "action"}
	case !isSupportedSubmitStatus(attempt.Status):
		return &SubmitAttemptValidationError{Field: "status"}
	case !isSupportedSubmitPhase(attempt.Phase):
		return &SubmitAttemptValidationError{Field: "phase"}
	case attempt.IdempotencyKey == "":
		return &SubmitAttemptValidationError{Field: "idempotency_key"}
	default:
		return nil
	}
}

func isSupportedSubmitStatus(status string) bool {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case SubmitStatusPending, SubmitStatusRunning, SubmitStatusSucceeded, SubmitStatusFailed, SubmitStatusRecovering:
		return true
	default:
		return false
	}
}

func isSupportedSubmitPhase(phase string) bool {
	switch strings.ToLower(strings.TrimSpace(phase)) {
	case SubmitPhaseValidate, SubmitPhasePrepareProduct, SubmitPhaseUploadImages, SubmitPhasePreValidate, SubmitPhaseSubmitRemote, SubmitPhasePersistResult:
		return true
	default:
		return false
	}
}
