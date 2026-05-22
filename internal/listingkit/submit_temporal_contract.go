package listingkit

import (
	"context"
	"fmt"
	"time"

	sheinpub "task-processor/internal/publishing/shein"
	sheinproduct "task-processor/internal/shein/api/product"
)

// SheinPublishWorkflowClient is the listingkit-facing seam for the Temporal
// SHEIN publish workflow. Implementations live outside the listingkit package.
type SheinPublishWorkflowClient interface {
	StartSheinPublish(ctx context.Context, in SheinPublishWorkflowStartInput) error
	QuerySheinPublishState(ctx context.Context, taskID string) (*SheinPublishWorkflowState, error)
}

type SheinPublishWorkflowStartInput struct {
	TaskID          string    `json:"task_id"`
	Platform        string    `json:"platform"`
	Action          string    `json:"action"`
	RequestID       string    `json:"request_id"`
	ConfirmedFinal  bool      `json:"confirmed_final"`
	RequestedAt     time.Time `json:"requested_at"`
	TriggeredByUser string    `json:"triggered_by_user,omitempty"`
}

type SheinPublishWorkflowState struct {
	TaskID          string     `json:"task_id"`
	Action          string     `json:"action"`
	RequestID       string     `json:"request_id,omitempty"`
	CurrentPhase    string     `json:"current_phase,omitempty"`
	LastError       string     `json:"last_error,omitempty"`
	StartedAt       *time.Time `json:"started_at,omitempty"`
	FinishedAt      *time.Time `json:"finished_at,omitempty"`
	WorkflowRunning bool       `json:"workflow_running"`
}

// SheinPublishActivityHost owns the service-facing contract that Temporal
// activities use to drive the SHEIN publish PoC without importing the temporal
// package back into listingkit.
type SheinPublishActivityHost interface {
	BeginSheinPublishAttempt(ctx context.Context, in SheinPublishAttemptInput) error
	ValidateSheinPublishReadiness(ctx context.Context, in SheinPublishAttemptInput) error
	PrepareSheinPublishPayload(ctx context.Context, in SheinPublishAttemptInput) (*SheinPreparedSubmitPayload, error)
	UploadSheinPublishImages(ctx context.Context, in *SheinPreparedSubmitPayload) (*SheinPreparedSubmitPayload, error)
	PreValidateSheinPublish(ctx context.Context, in *SheinPreparedSubmitPayload) error
	SubmitSheinPublishRemote(ctx context.Context, in *SheinPreparedSubmitPayload) (*SheinRemoteSubmitResult, error)
	PersistSheinPublishSuccess(ctx context.Context, in SheinPersistSubmitSuccessInput) error
	PersistSheinPublishFailure(ctx context.Context, in SheinPersistSubmitFailureInput) error
	RefreshSheinPublishRemoteStatus(ctx context.Context, in SheinRefreshRemoteStatusInput) (*SheinRefreshRemoteStatusResult, error)
	BuildSheinTaskPreview(ctx context.Context, taskID string) (*ListingKitPreview, error)
}

type SheinPublishAttemptInput struct {
	TaskID         string    `json:"task_id"`
	Action         string    `json:"action"`
	RequestID      string    `json:"request_id"`
	ConfirmedFinal bool      `json:"confirmed_final"`
	RequestedAt    time.Time `json:"requested_at"`
}

type SheinPreparedSubmitPayload struct {
	TaskID           string                   `json:"task_id"`
	Action           string                   `json:"action"`
	RequestID        string                   `json:"request_id"`
	Product          *sheinproduct.Product    `json:"product,omitempty"`
	NeedsImageUpload bool                     `json:"needs_image_upload"`
	Snapshot         *sheinpub.SubmitSnapshot `json:"snapshot,omitempty"`
}

type SheinRemoteSubmitResult struct {
	TaskID       string                       `json:"task_id"`
	Action       string                       `json:"action"`
	RequestID    string                       `json:"request_id"`
	SupplierCode string                       `json:"supplier_code,omitempty"`
	Response     *sheinpub.SubmissionResponse `json:"response,omitempty"`
	Snapshot     *sheinpub.SubmitSnapshot     `json:"snapshot,omitempty"`
}

type SheinPersistSubmitSuccessInput struct {
	TaskID       string                       `json:"task_id"`
	Action       string                       `json:"action"`
	RequestID    string                       `json:"request_id"`
	SupplierCode string                       `json:"supplier_code,omitempty"`
	Response     *sheinpub.SubmissionResponse `json:"response,omitempty"`
	Snapshot     *sheinpub.SubmitSnapshot     `json:"snapshot,omitempty"`
}

type SheinPersistSubmitFailureInput struct {
	TaskID       string                       `json:"task_id"`
	Action       string                       `json:"action"`
	RequestID    string                       `json:"request_id"`
	Phase        string                       `json:"phase"`
	ErrorMessage string                       `json:"error_message"`
	SupplierCode string                       `json:"supplier_code,omitempty"`
	Response     *sheinpub.SubmissionResponse `json:"response,omitempty"`
	Snapshot     *sheinpub.SubmitSnapshot     `json:"snapshot,omitempty"`
}

type SheinRefreshRemoteStatusInput struct {
	TaskID       string `json:"task_id"`
	Action       string `json:"action"`
	RequestID    string `json:"request_id"`
	SupplierCode string `json:"supplier_code,omitempty"`
}

type SheinRefreshRemoteStatusResult struct {
	TaskID       string `json:"task_id"`
	Action       string `json:"action"`
	RequestID    string `json:"request_id"`
	RemoteStatus string `json:"remote_status,omitempty"`
}

const SheinSubmitRemoteActivityErrorType = "listingkit.shein_publish.submit_remote"

type SheinSubmitRemoteActivityErrorDetails struct {
	ErrorMessage string                       `json:"error_message"`
	SupplierCode string                       `json:"supplier_code,omitempty"`
	Response     *sheinpub.SubmissionResponse `json:"response,omitempty"`
	Snapshot     *sheinpub.SubmitSnapshot     `json:"snapshot,omitempty"`
}

type SheinPublishActivityHostSource interface {
	SheinPublishActivityHost
}

func NewSheinPublishActivityHost(svc any) (SheinPublishActivityHost, error) {
	if svc == nil {
		return nil, fmt.Errorf("listingkit service is nil")
	}
	host, ok := svc.(SheinPublishActivityHost)
	if !ok {
		return nil, fmt.Errorf("listingkit service does not implement SheinPublishActivityHost")
	}
	return host, nil
}
