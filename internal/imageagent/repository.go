package imageagent

import (
	"context"
	"encoding/json"
	"errors"
)

var (
	ErrRunNotFound               = errors.New("image agent run not found")
	ErrRevisionConflict          = errors.New("image agent revision conflict")
	ErrCommandBlocked            = errors.New("image agent command blocked")
	ErrIdentityRequired          = errors.New("verified image agent identity is required")
	ErrValidation                = errors.New("invalid image agent request")
	ErrCatalogSnapshotMissing    = errors.New("image agent catalog snapshot is missing")
	ErrProjectionSnapshotMissing = errors.New("image agent projection snapshot is missing")
	ErrInvalidPersistedPolicy    = errors.New("invalid persisted image agent policy")
)

type RunScope struct {
	TenantID    string
	OwnerUserID string
	RunID       string
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
	OwnerUserID    string
	RunID          string
	SlotID         string
	PlanRevision   int64
	Node           string
	IdempotencyKey string
	Attempt        int
	Outcome        string
	ErrorCategory  string
}

type RunEvent struct {
	TenantID          string
	OwnerUserID       string
	RunID             string
	Type              string
	Cursor            int64
	ProjectionVersion int64
	Payload           json.RawMessage
}

type Repository interface {
	InitializeRun(context.Context, ProjectionInitialization) (RunProjection, error)
	GetProjection(context.Context, RunScope) (RunProjection, error)
	CommitProjection(context.Context, ProjectionCommit) (RunProjection, error)
	ListEvents(context.Context, RunScope, int64, int) ([]RunEvent, error)
	GetAssetCatalog(context.Context, RunScope) (AssetCatalog, error)
}
