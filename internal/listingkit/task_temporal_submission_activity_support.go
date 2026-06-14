package listingkit

import (
	"strings"
	"time"

	sdktemporal "go.temporal.io/sdk/temporal"
	sheinpub "task-processor/internal/publishing/shein"
)

func sheinSubmitRequestFromActivity(in SheinPublishAttemptInput) *SubmitTaskRequest {
	return &SubmitTaskRequest{
		Platform:       "shein",
		Action:         in.Action,
		RequestID:      in.RequestID,
		IdempotencyKey: in.RequestID,
		ConfirmedFinal: in.ConfirmedFinal,
	}
}

func sheinRequestedAt(requestedAt time.Time) time.Time {
	if requestedAt.IsZero() {
		return time.Now()
	}
	return requestedAt
}

func newSubmitRemoteActivityError(cause error, supplierCode string, response *sheinpub.SubmissionResponse, snapshot *sheinpub.SubmitSnapshot) error {
	details := SheinSubmitRemoteActivityErrorDetails{
		ErrorMessage: strings.TrimSpace(errorMessage(cause)),
		SupplierCode: supplierCode,
		Response:     response,
		Snapshot:     snapshot,
	}
	if details.ErrorMessage == "" {
		details.ErrorMessage = "shein submit remote failed"
	}
	return sdktemporal.NewNonRetryableApplicationError(
		details.ErrorMessage,
		SheinSubmitRemoteActivityErrorType,
		nil,
		details,
	)
}

func errorMessage(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}
