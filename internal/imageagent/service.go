package imageagent

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"task-processor/internal/authidentity"
)

type Service struct {
	repository Repository
	workflows  WorkflowClient
}

func NewService(repository Repository, workflows WorkflowClient) (*Service, error) {
	if repository == nil {
		return nil, fmt.Errorf("image agent repository is required")
	}
	if workflows == nil {
		return nil, fmt.Errorf("image agent workflow client is required")
	}
	return &Service{repository: repository, workflows: workflows}, nil
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
	if input.RunID == "" || input.IdempotencyKey == "" {
		return fmt.Errorf("%w: image agent run ID and idempotency key are required", ErrValidation)
	}
	input.Plan.CreatedBy = identity.UserID
	if err := ValidatePlan(input.Plan); err != nil {
		return fmt.Errorf("%w: validate image agent plan: %v", ErrValidation, err)
	}
	run := Run{
		ID: input.RunID, BusinessTaskID: input.BusinessTaskID,
		TenantID: identity.TenantID, UserID: identity.UserID,
		Mode: RunModeManual, IdempotencyKey: input.IdempotencyKey,
		Status: RunStatusPlanning, CurrentNode: "plan", Version: 1,
		Budget: input.Budget,
	}
	if err := s.ensureRun(ctx, &run); err != nil {
		return fmt.Errorf("create image agent run: %w", err)
	}
	scope := RunScope{TenantID: identity.TenantID, RunID: run.ID}
	if err := s.repository.AppendPlan(ctx, scope, 0, input.Plan); err != nil {
		return fmt.Errorf("append image agent plan: %w", err)
	}
	run.ActivePlanRevision = input.Plan.Revision
	return s.workflows.StartManual(ctx, WorkflowStart{
		Run: run, Plan: input.Plan, Identity: identity,
		MaxConcurrentSlots: input.MaxConcurrentSlots,
	})
}

func (s *Service) Get(ctx context.Context, runID string) (RunProjection, error) {
	identity, err := verifiedExecutionIdentity(ctx)
	if err != nil {
		return RunProjection{}, err
	}
	scope := RunScope{TenantID: identity.TenantID, RunID: strings.TrimSpace(runID)}
	run, err := s.repository.GetRun(ctx, scope)
	if err != nil {
		return RunProjection{}, err
	}
	workflowProjection, err := s.workflows.GetProjection(ctx, scope, identity)
	if err != nil {
		return RunProjection{}, fmt.Errorf("query image agent workflow projection: %w", err)
	}
	if workflowProjection.Plan.Revision != run.ActivePlanRevision {
		return RunProjection{}, ErrRevisionConflict
	}
	lastEventID, err := s.latestEventCursor(ctx, scope)
	if err != nil {
		return RunProjection{}, err
	}
	projectedRun := *run
	projectedRun.Status = workflowProjection.Status
	projectedRun.Block = cloneBlock(workflowProjection.Block)
	return RunProjection{
		Run: projectedRun, Plan: workflowProjection.Plan,
		Slots:        append([]SlotProjection(nil), workflowProjection.Slots...),
		ResultDigest: workflowProjection.ResultDigest,
		Actions:      AllowedActions(projectedRun),
		LastEventID:  lastEventID,
	}, nil
}

func (s *Service) ReplacePlan(ctx context.Context, runID string, expectedRevision int64, plan Plan, actionID string) error {
	identity, run, err := s.commandRun(ctx, runID, expectedRevision, actionID)
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
	if run.Status != RunStatusBlocked {
		return ErrCommandBlocked
	}
	return s.workflows.ReplacePlan(ctx, ReplacePlanCommand{
		RunID: strings.TrimSpace(runID), ExpectedRevision: expectedRevision, Plan: plan,
		ActorID: identity.UserID, ActionID: strings.TrimSpace(actionID), Identity: identity,
	})
}

func (s *Service) ensureRun(ctx context.Context, desired *Run) error {
	err := s.repository.CreateRun(ctx, desired)
	if err == nil || !errors.Is(err, ErrRevisionConflict) {
		return err
	}
	existing, getErr := s.repository.GetRun(ctx, RunScope{TenantID: desired.TenantID, RunID: desired.ID})
	if getErr != nil {
		return err
	}
	if existing.ID != desired.ID || existing.BusinessTaskID != desired.BusinessTaskID ||
		existing.TenantID != desired.TenantID || existing.UserID != desired.UserID ||
		existing.Mode != desired.Mode || existing.IdempotencyKey != desired.IdempotencyKey ||
		existing.Budget != desired.Budget {
		return err
	}
	return nil
}

func (s *Service) RetrySlot(ctx context.Context, runID, slotID string, planRevision int64, actionID string) error {
	identity, run, err := s.commandRun(ctx, runID, planRevision, actionID)
	if err != nil {
		return err
	}
	slotID = strings.TrimSpace(slotID)
	if slotID == "" {
		return fmt.Errorf("%w: retry slot ID is required", ErrValidation)
	}
	if run.Status != RunStatusBlocked || run.Block == nil || strings.TrimSpace(run.Block.SlotID) != slotID {
		return ErrCommandBlocked
	}
	return s.workflows.RetrySlot(ctx, RetrySlotCommand{RunID: strings.TrimSpace(runID), SlotID: slotID, PlanRevision: planRevision, ActorID: identity.UserID, ActionID: strings.TrimSpace(actionID), Identity: identity})
}

func (s *Service) ApproveResults(ctx context.Context, runID string, planRevision int64, resultDigest, actionID string) error {
	identity, run, err := s.commandRun(ctx, runID, planRevision, actionID)
	if err != nil {
		return err
	}
	resultDigest = strings.TrimSpace(resultDigest)
	if resultDigest == "" || run.Status != RunStatusAwaitingFinalApproval {
		return ErrCommandBlocked
	}
	projection, err := s.workflows.GetProjection(ctx, RunScope{TenantID: identity.TenantID, RunID: strings.TrimSpace(runID)}, identity)
	if err != nil {
		return fmt.Errorf("query image agent workflow projection for approval: %w", err)
	}
	if projection.Status != RunStatusAwaitingFinalApproval || projection.Plan.Revision != planRevision || strings.TrimSpace(projection.ResultDigest) == "" || strings.TrimSpace(projection.ResultDigest) != resultDigest {
		return ErrCommandBlocked
	}
	return s.workflows.ApproveResults(ctx, ApproveResultsCommand{RunID: strings.TrimSpace(runID), PlanRevision: planRevision, ResultDigest: resultDigest, ActorID: identity.UserID, ActionID: strings.TrimSpace(actionID), Identity: identity})
}

func (s *Service) Cancel(ctx context.Context, runID string, planRevision int64, actionID string) error {
	identity, run, err := s.commandRun(ctx, runID, planRevision, actionID)
	if err != nil {
		return err
	}
	switch run.Status {
	case RunStatusCompleted, RunStatusFailed, RunStatusCancelled:
		return ErrCommandBlocked
	}
	return s.workflows.Cancel(ctx, CancelRunCommand{RunID: strings.TrimSpace(runID), PlanRevision: planRevision, ActorID: identity.UserID, ActionID: strings.TrimSpace(actionID), Identity: identity})
}

func (s *Service) ListEvents(ctx context.Context, runID string, afterCursor int64, limit int) ([]RunEvent, error) {
	identity, err := verifiedExecutionIdentity(ctx)
	if err != nil {
		return nil, err
	}
	if afterCursor < 0 || limit <= 0 {
		return nil, fmt.Errorf("%w: event cursor and limit are invalid", ErrValidation)
	}
	scope := RunScope{TenantID: identity.TenantID, RunID: strings.TrimSpace(runID)}
	events, err := s.repository.ListEvents(ctx, scope, afterCursor, limit)
	if err != nil {
		return nil, err
	}
	last := afterCursor
	for _, event := range events {
		if event.TenantID != scope.TenantID || event.RunID != scope.RunID || event.Cursor <= last {
			return nil, ErrRevisionConflict
		}
		last = event.Cursor
	}
	return events, nil
}

func (s *Service) commandRun(ctx context.Context, runID string, revision int64, actionID string) (ExecutionIdentity, *Run, error) {
	identity, err := verifiedExecutionIdentity(ctx)
	if err != nil {
		return ExecutionIdentity{}, nil, err
	}
	if strings.TrimSpace(actionID) == "" {
		return ExecutionIdentity{}, nil, fmt.Errorf("%w: action ID is required", ErrValidation)
	}
	run, err := s.repository.GetRun(ctx, RunScope{TenantID: identity.TenantID, RunID: strings.TrimSpace(runID)})
	if err != nil {
		return ExecutionIdentity{}, nil, err
	}
	if run.ActivePlanRevision != revision {
		return ExecutionIdentity{}, nil, ErrRevisionConflict
	}
	if revision <= 0 {
		return ExecutionIdentity{}, nil, fmt.Errorf("%w: positive plan revision is required", ErrValidation)
	}
	return identity, run, nil
}

func (s *Service) latestEventCursor(ctx context.Context, scope RunScope) (int64, error) {
	const pageSize = 100
	var cursor int64
	for {
		events, err := s.repository.ListEvents(ctx, scope, cursor, pageSize)
		if err != nil {
			return 0, err
		}
		for _, event := range events {
			if event.TenantID != scope.TenantID || event.RunID != scope.RunID || event.Cursor <= cursor {
				return 0, ErrRevisionConflict
			}
			cursor = event.Cursor
		}
		if len(events) < pageSize {
			return cursor, nil
		}
	}
}

func verifiedExecutionIdentity(ctx context.Context) (ExecutionIdentity, error) {
	identity, ok := authidentity.AuthenticatedIdentityFromContext(ctx)
	if !ok {
		return ExecutionIdentity{}, ErrIdentityRequired
	}
	return ExecutionIdentity{TenantID: identity.TenantID, UserID: identity.UserID}, nil
}

func cloneBlock(block *Block) *Block {
	if block == nil {
		return nil
	}
	cloned := *block
	return &cloned
}
