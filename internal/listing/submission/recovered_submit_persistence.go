package submission

import (
	"fmt"
	"time"
)

// RecoveredSubmitPersistenceRequest provides callbacks for recovered submit persistence.
type RecoveredSubmitPersistenceRequest struct {
	TaskID               string
	PreviousBlock        *RetryableBlockState
	RecoveredAt          time.Time
	DefaultRecoveryScope string
	Submit               func(string) error
	MarkBlockedRetryable func(*RetryableBlockState, string) error
	PersistFailure       func(string, error) error
	RestoreDurability    func(string, error, error) error
}

// SubmitRecoveredWithRetryablePersistence submits a recovered task and handles retryable persistence.
func SubmitRecoveredWithRetryablePersistence(request RecoveredSubmitPersistenceRequest) error {
	if request.Submit == nil {
		return fmt.Errorf("recovered submit callback is not configured")
	}
	err := request.Submit(request.TaskID)
	if err == nil {
		return nil
	}

	errorMsg := fmt.Sprintf("failed to submit task: %v", err)
	if classified, ok := ClassifyRetryableFailure(err, request.DefaultRecoveryScope); ok {
		updated := BuildReblockedRetryableBlock(request.PreviousBlock, classified, request.RecoveredAt, request.DefaultRecoveryScope)
		if request.MarkBlockedRetryable == nil {
			return fmt.Errorf("mark blocked retryable callback is not configured")
		}
		if markErr := request.MarkBlockedRetryable(updated, errorMsg); markErr != nil {
			return restoreRecoveredSubmitDurability(request, errorMsg, err, fmt.Errorf("mark blocked retryable: %w", markErr))
		}
		return fmt.Errorf("submit recovered task %s: %w", request.TaskID, err)
	}

	if request.PersistFailure != nil {
		if persistErr := request.PersistFailure(errorMsg, err); persistErr != nil {
			return restoreRecoveredSubmitDurability(request, errorMsg, err, persistErr)
		}
	}
	return fmt.Errorf("submit recovered task %s: %w", request.TaskID, err)
}

func restoreRecoveredSubmitDurability(request RecoveredSubmitPersistenceRequest, errorMsg string, submitErr error, persistErr error) error {
	if request.RestoreDurability == nil {
		return fmt.Errorf("submit recovered task %s: %w", request.TaskID, submitErr)
	}
	return request.RestoreDurability(errorMsg, submitErr, persistErr)
}
