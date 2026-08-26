package imageagent

import (
	"context"
	"encoding/json"
	"errors"
)

var (
	ErrRunNotFound      = errors.New("image agent run not found")
	ErrRevisionConflict = errors.New("image agent revision conflict")
	ErrCommandBlocked   = errors.New("image agent command blocked")
	ErrIdentityRequired = errors.New("verified image agent identity is required")
	ErrValidation       = errors.New("invalid image agent request")
)

type RunScope struct {
	TenantID string
	RunID    string
}

type RunMutation struct {
	Status             RunStatus
	CurrentNode        string
	ActivePlanRevision int64
	Block              *Block
}

type SlotResult struct {
	SlotID            string
	Attempt           int
	Status            SlotStatus
	CandidateAssetIDs []string
	ErrorCode         string
}

type StepAttempt struct {
	TenantID       string
	RunID          string
	SlotID         string
	Node           string
	IdempotencyKey string
	Attempt        int
	Outcome        string
	ErrorCategory  string
}

type RunEvent struct {
	TenantID          string
	RunID             string
	Type              string
	Cursor            int64
	ProjectionVersion int64
	Payload           json.RawMessage
}

type Repository interface {
	CreateRun(context.Context, *Run) error
	GetRun(context.Context, RunScope) (*Run, error)
	UpdateRun(context.Context, RunScope, int64, RunMutation) error
	AppendPlan(context.Context, RunScope, int64, Plan) error
	SaveSlotResult(context.Context, RunScope, int64, SlotResult) error
	AppendAttempt(context.Context, StepAttempt) error
	AppendEvent(context.Context, RunEvent) error
	AppendProjectionEvent(context.Context, RunEvent) (RunEvent, error)
	ListEvents(context.Context, RunScope, int64, int) ([]RunEvent, error)
	SaveAssetCatalog(context.Context, RunScope, AssetCatalog) error
	GetAssetCatalog(context.Context, RunScope) (AssetCatalog, error)
}
