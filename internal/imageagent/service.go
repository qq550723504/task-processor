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
		return fmt.Errorf("image agent start mode must be manual")
	}
	input.RunID = strings.TrimSpace(input.RunID)
	input.BusinessTaskID = strings.TrimSpace(input.BusinessTaskID)
	input.IdempotencyKey = strings.TrimSpace(input.IdempotencyKey)
	if input.RunID == "" || input.IdempotencyKey == "" {
		return fmt.Errorf("image agent run ID and idempotency key are required")
	}
	input.Plan.CreatedBy = identity.UserID
	if err := ValidatePlan(input.Plan); err != nil {
		return fmt.Errorf("validate image agent plan: %w", err)
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
	identity, err := verifiedExecutionIdentity(ctx)
	if err != nil {
		return err
	}
	if err := s.requireActiveRevision(ctx, identity, runID, planRevision); err != nil {
		return err
	}
	return s.workflows.RetrySlot(ctx, RetrySlotCommand{RunID: strings.TrimSpace(runID), SlotID: strings.TrimSpace(slotID), PlanRevision: planRevision, ActorID: identity.UserID, ActionID: strings.TrimSpace(actionID), Identity: identity})
}

func (s *Service) ApproveResults(ctx context.Context, runID string, planRevision int64, resultDigest, actionID string) error {
	identity, err := verifiedExecutionIdentity(ctx)
	if err != nil {
		return err
	}
	resultDigest = strings.TrimSpace(resultDigest)
	if resultDigest == "" {
		return fmt.Errorf("image agent result digest is required")
	}
	if err := s.requireActiveRevision(ctx, identity, runID, planRevision); err != nil {
		return err
	}
	return s.workflows.ApproveResults(ctx, ApproveResultsCommand{RunID: strings.TrimSpace(runID), PlanRevision: planRevision, ResultDigest: resultDigest, ActorID: identity.UserID, ActionID: strings.TrimSpace(actionID), Identity: identity})
}

func (s *Service) Cancel(ctx context.Context, runID string, planRevision int64, actionID string) error {
	identity, err := verifiedExecutionIdentity(ctx)
	if err != nil {
		return err
	}
	if err := s.requireActiveRevision(ctx, identity, runID, planRevision); err != nil {
		return err
	}
	return s.workflows.Cancel(ctx, CancelRunCommand{RunID: strings.TrimSpace(runID), PlanRevision: planRevision, ActorID: identity.UserID, ActionID: strings.TrimSpace(actionID), Identity: identity})
}

func (s *Service) requireActiveRevision(ctx context.Context, identity ExecutionIdentity, runID string, revision int64) error {
	run, err := s.repository.GetRun(ctx, RunScope{TenantID: identity.TenantID, RunID: strings.TrimSpace(runID)})
	if err != nil {
		return err
	}
	if run.ActivePlanRevision != revision {
		return ErrRevisionConflict
	}
	return nil
}

func verifiedExecutionIdentity(ctx context.Context) (ExecutionIdentity, error) {
	identity, ok := authidentity.AuthenticatedIdentityFromContext(ctx)
	if !ok {
		return ExecutionIdentity{}, fmt.Errorf("verified image agent identity is required")
	}
	return ExecutionIdentity{TenantID: identity.TenantID, UserID: identity.UserID}, nil
}
