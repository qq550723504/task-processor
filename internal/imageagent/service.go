package imageagent

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"strings"

	"task-processor/internal/authidentity"
	"task-processor/internal/shared/aiidentity"
)

type Service struct {
	repository Repository
	workflows  WorkflowClient
	catalogs   AuthorizedAssetCatalog
}

func NewService(repository Repository, workflows WorkflowClient, catalogs AuthorizedAssetCatalog) (*Service, error) {
	if repository == nil {
		return nil, fmt.Errorf("image agent repository is required")
	}
	if workflows == nil {
		return nil, fmt.Errorf("image agent workflow client is required")
	}
	if catalogs == nil {
		return nil, fmt.Errorf("image agent authorized asset catalog is required")
	}
	return &Service{repository: repository, workflows: workflows, catalogs: catalogs}, nil
}

func (s *Service) Start(ctx context.Context, input StartRunInput) error {
	identity, err := verifiedExecutionIdentity(ctx)
	if err != nil {
		return err
	}
	if input.Mode != RunModeManual {
		return fmt.Errorf("%w: image agent start mode must be manual", ErrValidation)
	}
	input.RunID = strings.TrimSpace(input.RunID)
	input.BusinessTaskID = strings.TrimSpace(input.BusinessTaskID)
	input.IdempotencyKey = strings.TrimSpace(input.IdempotencyKey)
	if err := ValidateMaxConcurrentSlots(input.MaxConcurrentSlots); err != nil {
		return err
	}
	input.MaxConcurrentSlots = NormalizeMaxConcurrentSlots(input.MaxConcurrentSlots)
	if input.RunID == "" || input.BusinessTaskID == "" || input.IdempotencyKey == "" {
		return fmt.Errorf("%w: image agent run ID, business task ID, and idempotency key are required", ErrValidation)
	}
	if err := ValidateArtifactKeyIdentifier(input.RunID); err != nil {
		return fmt.Errorf("%w: image agent run ID cannot be used in a durable artifact key", err)
	}
	if _, err := input.Budget.Policy(); err != nil {
		return fmt.Errorf("%w: validate image agent budget: %v", ErrValidation, err)
	}
	identity.BusinessTaskID = input.BusinessTaskID
	input.Plan.CreatedBy = identity.UserID
	if err := ValidatePlan(input.Plan); err != nil {
		return fmt.Errorf("%w: validate image agent plan: %v", ErrValidation, err)
	}
	scope := RunScope{TenantID: identity.TenantID, OwnerUserID: identity.UserID, RunID: input.RunID}
	if existing, getErr := s.repository.GetProjection(ctx, scope); getErr == nil {
		if existing.Run.BusinessTaskID != input.BusinessTaskID || existing.Run.IdempotencyKey != input.IdempotencyKey || existing.Run.Budget != input.Budget || existing.Run.MaxConcurrentSlots != input.MaxConcurrentSlots || !reflect.DeepEqual(existing.Plan, input.Plan) {
			return ErrRevisionConflict
		}
		if existing.Run.Status == RunStatusCompleted {
			return nil
		}
		return s.workflows.StartManual(ctx, WorkflowStart{Run: existing.Run, Plan: existing.Plan, Identity: identity, MaxConcurrentSlots: existing.Run.MaxConcurrentSlots, AssetCatalog: existing.AssetCatalog})
	} else if !errors.Is(getErr, ErrRunNotFound) {
		return getErr
	}
	catalog, err := s.catalogs.Resolve(ctx, AssetCatalogScope{TenantID: identity.TenantID, OwnerUserID: identity.UserID, BusinessTaskID: input.BusinessTaskID, RunID: input.RunID})
	if err != nil {
		return fmt.Errorf("%w: resolve authorized image assets: %v", ErrValidation, err)
	}
	catalog, err = NormalizeAssetCatalog(catalog)
	if err != nil {
		return fmt.Errorf("%w: normalize authorized image assets: %v", ErrValidation, err)
	}
	if err := ValidatePlanAgainstCatalog(input.Plan, catalog); err != nil {
		return fmt.Errorf("%w: validate authorized image assets: %v", ErrValidation, err)
	}
	run := Run{
		ID: input.RunID, BusinessTaskID: input.BusinessTaskID,
		TenantID: identity.TenantID, UserID: identity.UserID,
		Mode: RunModeManual, IdempotencyKey: input.IdempotencyKey,
		Status: RunStatusPlanning, CurrentNode: "plan", Version: 1, ActivePlanRevision: input.Plan.Revision,
		Budget: input.Budget, MaxConcurrentSlots: input.MaxConcurrentSlots,
	}
	projection, err := s.repository.InitializeRun(ctx, ProjectionInitialization{
		Scope: scope, Run: run, Plan: input.Plan, Catalog: catalog,
		Snapshot: RunProjection{Run: run, Plan: input.Plan}, CommitID: "start:" + input.IdempotencyKey,
		EventType: "run.initialized", EventPayload: json.RawMessage(`{}`),
	})
	if err != nil {
		return fmt.Errorf("initialize image agent run: %w", err)
	}
	return s.workflows.StartManual(ctx, WorkflowStart{
		Run: projection.Run, Plan: projection.Plan, Identity: identity,
		MaxConcurrentSlots: projection.Run.MaxConcurrentSlots,
		AssetCatalog:       projection.AssetCatalog,
	})
}

func (s *Service) Get(ctx context.Context, runID string) (RunProjection, error) {
	identity, err := verifiedExecutionIdentity(ctx)
	if err != nil {
		return RunProjection{}, err
	}
	scope := RunScope{TenantID: identity.TenantID, OwnerUserID: identity.UserID, RunID: strings.TrimSpace(runID)}
	return s.repository.GetProjection(ctx, scope)
}

func (s *Service) ReplacePlan(ctx context.Context, runID string, expectedRevision int64, plan Plan, actionID string) error {
	identity, err := s.commandIdentity(ctx, runID, expectedRevision, actionID)
	if err != nil {
		return err
	}
	plan.CreatedBy = identity.UserID
	if plan.ParentRevision != expectedRevision || plan.Revision <= expectedRevision {
		return fmt.Errorf("%w: replacement plan must advance and name its parent revision", ErrValidation)
	}
	if err := ValidatePlan(plan); err != nil {
		return fmt.Errorf("%w: validate replacement plan: %v", ErrValidation, err)
	}
	catalog, err := s.repository.GetAssetCatalog(ctx, RunScope{TenantID: identity.TenantID, OwnerUserID: identity.UserID, RunID: strings.TrimSpace(runID)})
	if err != nil {
		return err
	}
	if err := ValidatePlanAgainstCatalog(plan, catalog); err != nil {
		return fmt.Errorf("%w: validate authorized image assets: %v", ErrValidation, err)
	}
	return s.workflows.ReplacePlan(ctx, ReplacePlanCommand{
		RunID: strings.TrimSpace(runID), ExpectedRevision: expectedRevision, Plan: plan,
		ActorID: identity.UserID, ActionID: strings.TrimSpace(actionID), Identity: identity,
	})
}

func (s *Service) RetrySlot(ctx context.Context, runID, slotID string, planRevision int64, actionID string) error {
	identity, err := s.commandIdentity(ctx, runID, planRevision, actionID)
	if err != nil {
		return err
	}
	slotID = strings.TrimSpace(slotID)
	if slotID == "" {
		return fmt.Errorf("%w: retry slot ID is required", ErrValidation)
	}
	if err := ValidateArtifactKeyIdentifier(slotID); err != nil {
		return fmt.Errorf("%w: retry slot ID cannot be used in a durable artifact key", err)
	}
	return s.workflows.RetrySlot(ctx, RetrySlotCommand{RunID: strings.TrimSpace(runID), SlotID: slotID, PlanRevision: planRevision, ActorID: identity.UserID, ActionID: strings.TrimSpace(actionID), Identity: identity})
}

func (s *Service) ApproveResults(ctx context.Context, runID string, planRevision int64, resultDigest, actionID string) error {
	identity, err := s.commandIdentity(ctx, runID, planRevision, actionID)
	if err != nil {
		return err
	}
	if resultDigest == "" || resultDigest != strings.TrimSpace(resultDigest) {
		return ErrCommandBlocked
	}
	return s.workflows.ApproveResults(ctx, ApproveResultsCommand{RunID: strings.TrimSpace(runID), PlanRevision: planRevision, ResultDigest: resultDigest, ActorID: identity.UserID, ActionID: strings.TrimSpace(actionID), Identity: identity})
}

func (s *Service) Cancel(ctx context.Context, runID string, planRevision int64, actionID string) error {
	identity, err := s.commandIdentity(ctx, runID, planRevision, actionID)
	if err != nil {
		return err
	}
	return s.workflows.Cancel(ctx, CancelRunCommand{RunID: strings.TrimSpace(runID), PlanRevision: planRevision, ActorID: identity.UserID, ActionID: strings.TrimSpace(actionID), Identity: identity})
}

func (s *Service) Resume(ctx context.Context, runID, actionID string) (CommandAcknowledgement, error) {
	identity, err := verifiedExecutionIdentity(ctx)
	if err != nil {
		return CommandAcknowledgement{}, err
	}
	runID, actionID = strings.TrimSpace(runID), strings.TrimSpace(actionID)
	if runID == "" || actionID == "" {
		return CommandAcknowledgement{}, fmt.Errorf("%w: run ID and action ID are required", ErrValidation)
	}
	if _, err := s.repository.GetProjection(ctx, RunScope{TenantID: identity.TenantID, OwnerUserID: identity.UserID, RunID: runID}); err != nil {
		return CommandAcknowledgement{}, err
	}
	return s.workflows.Resume(ctx, ResumeCommand{RunID: runID, ActorID: identity.UserID, ActionID: actionID, Identity: identity})
}

func (s *Service) ListEvents(ctx context.Context, runID string, afterCursor int64, limit int) ([]RunEvent, error) {
	identity, err := verifiedExecutionIdentity(ctx)
	if err != nil {
		return nil, err
	}
	if afterCursor < 0 || limit <= 0 {
		return nil, fmt.Errorf("%w: event cursor and limit are invalid", ErrValidation)
	}
	scope := RunScope{TenantID: identity.TenantID, OwnerUserID: identity.UserID, RunID: strings.TrimSpace(runID)}
	events, err := s.repository.ListEvents(ctx, scope, afterCursor, limit)
	if err != nil {
		return nil, err
	}
	last := afterCursor
	for _, event := range events {
		if event.TenantID != scope.TenantID || event.OwnerUserID != scope.OwnerUserID || event.RunID != scope.RunID || event.Cursor <= last || event.ProjectionVersion != event.Cursor {
			return nil, ErrRevisionConflict
		}
		last = event.Cursor
	}
	return events, nil
}

func (s *Service) commandIdentity(ctx context.Context, runID string, revision int64, actionID string) (ExecutionIdentity, error) {
	identity, err := verifiedExecutionIdentity(ctx)
	if err != nil {
		return ExecutionIdentity{}, err
	}
	runID = strings.TrimSpace(runID)
	if runID == "" || strings.TrimSpace(actionID) == "" {
		return ExecutionIdentity{}, fmt.Errorf("%w: run ID and action ID are required", ErrValidation)
	}
	if revision <= 0 {
		return ExecutionIdentity{}, fmt.Errorf("%w: positive plan revision is required", ErrValidation)
	}
	projection, err := s.repository.GetProjection(ctx, RunScope{TenantID: identity.TenantID, OwnerUserID: identity.UserID, RunID: runID})
	if err != nil {
		return ExecutionIdentity{}, err
	}
	identity.BusinessTaskID = projection.Run.BusinessTaskID
	return identity, nil
}

func verifiedExecutionIdentity(ctx context.Context) (ExecutionIdentity, error) {
	identity, ok := authidentity.AuthenticatedIdentityFromContext(ctx)
	if !ok {
		return ExecutionIdentity{}, ErrIdentityRequired
	}
	ai := aiidentity.FromContext(ctx)
	return ExecutionIdentity{TenantID: identity.TenantID, UserID: identity.UserID, TraceID: ai.TraceID}, nil
}

func cloneBlock(block *Block) *Block {
	if block == nil {
		return nil
	}
	cloned := *block
	return &cloned
}

func clonePendingCommand(receipt *PendingCommandReceipt) *PendingCommandReceipt {
	if receipt == nil {
		return nil
	}
	cloned := *receipt
	return &cloned
}
