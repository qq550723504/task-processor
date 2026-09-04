package imageagent

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"strings"

	"github.com/google/uuid"

	"task-processor/internal/authidentity"
	"task-processor/internal/shared/aiidentity"
)

type Service struct {
	repository Repository
	workflows  WorkflowClient
	catalogs   AuthorizedAssetCatalog
	startGate  TenantStartGate
}

// TenantStartGate admits a run before any durable run state or workflow is
// created. It keeps provider governance at the service boundary rather than
// allowing an ineligible tenant to create a run that must fail later.
type TenantStartGate interface {
	AllowTenantStart(context.Context, string) bool
}

// TenantAllowlistStartGate is the provider-neutral admission policy for the
// ProductImage capability. Enabled and the allowlist must both be true for a
// tenant to start an image-agent run.
type TenantAllowlistStartGate struct {
	Enabled          bool
	AllowedTenantIDs []string
}

func (g TenantAllowlistStartGate) AllowTenantStart(_ context.Context, tenantID string) bool {
	if !g.Enabled {
		return false
	}
	tenantID = strings.TrimSpace(tenantID)
	if tenantID == "" {
		return false
	}
	for _, allowed := range g.AllowedTenantIDs {
		if strings.TrimSpace(allowed) == tenantID {
			return true
		}
	}
	return false
}

type ServiceOption func(*Service) error

func WithTenantStartGate(gate TenantStartGate) ServiceOption {
	return func(service *Service) error {
		if gate == nil {
			return fmt.Errorf("image agent tenant start gate is required")
		}
		service.startGate = gate
		return nil
	}
}

func NewService(repository Repository, workflows WorkflowClient, catalogs AuthorizedAssetCatalog, options ...ServiceOption) (*Service, error) {
	if repository == nil {
		return nil, fmt.Errorf("image agent repository is required")
	}
	if workflows == nil {
		return nil, fmt.Errorf("image agent workflow client is required")
	}
	if catalogs == nil {
		return nil, fmt.Errorf("image agent authorized asset catalog is required")
	}
	service := &Service{repository: repository, workflows: workflows, catalogs: catalogs}
	for _, option := range options {
		if option == nil {
			return nil, fmt.Errorf("image agent service option is required")
		}
		if err := option(service); err != nil {
			return nil, err
		}
	}
	return service, nil
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
	if err := ValidateImagePolicyContext(input.TargetPlatform, input.ImagePolicyContext); err != nil {
		return err
	}
	if err := ValidateMaxConcurrentSlots(input.MaxConcurrentSlots); err != nil {
		return err
	}
	input.MaxConcurrentSlots = NormalizeMaxConcurrentSlots(input.MaxConcurrentSlots)
	if input.RunID == "" || input.BusinessTaskID == "" || input.IdempotencyKey == "" {
		return fmt.Errorf("%w: image agent run ID, business task ID, and idempotency key are required", ErrValidation)
	}
	if err := validatePersistedIdempotencyKey("run", input.IdempotencyKey); err != nil {
		return fmt.Errorf("%w: %v", ErrValidation, err)
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
		if existing.Run.BusinessTaskID != input.BusinessTaskID || existing.Run.TargetPlatform != input.TargetPlatform || existing.Run.ImagePolicyContext != input.ImagePolicyContext || existing.Run.IdempotencyKey != input.IdempotencyKey || existing.Run.Budget != input.Budget || existing.Run.MaxConcurrentSlots != input.MaxConcurrentSlots || !reflect.DeepEqual(existing.Plan, input.Plan) {
			return ErrRevisionConflict
		}
		if existing.Run.Status == RunStatusCompleted || existing.Run.Status == RunStatusCancelled {
			return nil
		}
		return s.startExistingProjection(ctx, existing, identity)
	} else if !errors.Is(getErr, ErrRunNotFound) {
		return getErr
	}
	if !s.tenantStartAllowed(ctx, identity.TenantID) {
		return fmt.Errorf("%w: image agent provider is unavailable for tenant", ErrCommandBlocked)
	}
	if err := ValidateInitialSubmittedPlan(input.Plan); err != nil {
		return fmt.Errorf("%w: validate image agent plan: %v", ErrValidation, err)
	}
	primarySourceAssetID := ""
	if len(input.Plan.SourceAssetIDs) == 1 {
		primarySourceAssetID = strings.TrimSpace(input.Plan.SourceAssetIDs[0])
	}
	catalog, err := s.catalogs.Resolve(ctx, AssetCatalogScope{TenantID: identity.TenantID, OwnerUserID: identity.UserID, BusinessTaskID: input.BusinessTaskID, RunID: input.RunID, TargetPlatform: input.TargetPlatform, PrimarySourceAssetID: primarySourceAssetID, StyleReferenceIDs: append([]string(nil), input.Plan.StyleReferenceIDs...)})
	if err != nil {
		return fmt.Errorf("%w: resolve authorized image assets: %v", ErrValidation, err)
	}
	catalog, err = NormalizeAssetCatalog(catalog)
	if err != nil {
		return fmt.Errorf("%w: normalize authorized image assets: %v", ErrValidation, err)
	}
	if err := ValidateSubmittedPlanAgainstCatalog(input.Plan, catalog); err != nil {
		return fmt.Errorf("%w: validate authorized image assets: %v", ErrValidation, err)
	}
	run := Run{
		ID: input.RunID, BusinessTaskID: input.BusinessTaskID, TargetPlatform: input.TargetPlatform,
		ImagePolicyContext: input.ImagePolicyContext,
		TenantID:           identity.TenantID, UserID: identity.UserID,
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

// LaunchTaskRun starts a manual single-main-slot run for a verified business
// task without trusting a browser-built plan. It resolves the task-scoped
// authorized asset catalog, selects the primary (or caller-requested) source
// asset, and delegates to Start so admission, idempotent replay, and workflow
// ingress reuse the generic run-creation semantics unchanged.
func (s *Service) LaunchTaskRun(ctx context.Context, input TaskRunLaunchInput) (TaskRunLaunchResult, error) {
	identity, err := verifiedExecutionIdentity(ctx)
	if err != nil {
		return TaskRunLaunchResult{}, err
	}
	input.RequestID = strings.TrimSpace(input.RequestID)
	input.BusinessTaskID = strings.TrimSpace(input.BusinessTaskID)
	input.TargetPlatform = strings.ToLower(strings.TrimSpace(input.TargetPlatform))
	input.SourceAssetID = strings.TrimSpace(input.SourceAssetID)
	if input.RequestID == "" || input.BusinessTaskID == "" {
		return TaskRunLaunchResult{}, fmt.Errorf("%w: request ID and business task ID are required", ErrValidation)
	}
	if err := validatePersistedIdempotencyKey("launch request", input.RequestID); err != nil {
		return TaskRunLaunchResult{}, fmt.Errorf("%w: %v", ErrValidation, err)
	}
	if err := ValidateImagePolicyContext(input.TargetPlatform, input.ImagePolicyContext); err != nil {
		return TaskRunLaunchResult{}, err
	}
	styleIDs, err := normalizeTaskLaunchStyleIDs(input.StyleAssetIDs)
	if err != nil {
		return TaskRunLaunchResult{}, fmt.Errorf("%w: %v", ErrValidation, err)
	}
	// Admission is checked before the task catalog is resolved: the production
	// resolver can probe up to 32 remote images on a 30s budget, and ineligible
	// tenants must fail closed without consuming that capacity.
	if !s.tenantStartAllowed(ctx, identity.TenantID) {
		return TaskRunLaunchResult{}, fmt.Errorf("%w: image agent provider is unavailable for tenant", ErrCommandBlocked)
	}
	if input.SourceAssetID == "" {
		return TaskRunLaunchResult{}, fmt.Errorf("%w: source_asset_id must select one task-owned source asset", ErrValidation)
	}
	runID, runIdempotencyKey := taskLaunchRunIdentity(identity, input)
	catalog, err := s.catalogs.Resolve(ctx, AssetCatalogScope{
		TenantID: identity.TenantID, OwnerUserID: identity.UserID, BusinessTaskID: input.BusinessTaskID,
		RunID: runID, TargetPlatform: input.TargetPlatform,
		PrimarySourceAssetID: input.SourceAssetID, StyleReferenceIDs: styleIDs,
	})
	if err != nil {
		return TaskRunLaunchResult{}, fmt.Errorf("%w: resolve authorized image assets: %v", ErrValidation, err)
	}
	catalog, err = NormalizeAssetCatalog(catalog)
	if err != nil {
		return TaskRunLaunchResult{}, fmt.Errorf("%w: normalize authorized image assets: %v", ErrValidation, err)
	}
	primarySource, err := primaryTaskLaunchSource(catalog, input.SourceAssetID)
	if err != nil {
		return TaskRunLaunchResult{}, fmt.Errorf("%w: %v", ErrValidation, err)
	}
	if err := validateTaskLaunchStyleIDs(catalog, styleIDs); err != nil {
		return TaskRunLaunchResult{}, fmt.Errorf("%w: %v", ErrValidation, err)
	}
	plan := Plan{
		Revision: 1, IdempotencyKey: runID + "-plan",
		SourceAssetIDs: []string{primarySource}, StyleReferenceIDs: styleIDs,
		Slots: []Slot{{
			ID: "main", Role: SlotRoleMain, SourceAssetIDs: []string{primarySource}, StyleReferenceIDs: styleIDs,
			IdempotencyKey: runID + "-slot-main", Status: SlotStatusPending,
		}},
	}
	if err := s.Start(ctx, StartRunInput{
		RunID: runID, BusinessTaskID: input.BusinessTaskID, TargetPlatform: input.TargetPlatform,
		ImagePolicyContext: input.ImagePolicyContext, Mode: RunModeManual,
		IdempotencyKey: runIdempotencyKey, Plan: plan,
		Budget: Budget{MaxImages: 1, EnabledLimits: BudgetLimitImages}, MaxConcurrentSlots: 1,
	}); err != nil {
		return TaskRunLaunchResult{}, err
	}
	return TaskRunLaunchResult{RunID: runID}, nil
}

// PreflightTaskRunAssets resolves the task-scoped asset catalog without
// creating a run. The workspace launcher uses it to force an explicit
// source selection; crawler ordering never establishes user intent.
func (s *Service) PreflightTaskRunAssets(ctx context.Context, input TaskRunAssetsInput) (TaskRunAssetPreflight, error) {
	identity, err := verifiedExecutionIdentity(ctx)
	if err != nil {
		return TaskRunAssetPreflight{}, err
	}
	input.BusinessTaskID = strings.TrimSpace(input.BusinessTaskID)
	input.TargetPlatform = strings.ToLower(strings.TrimSpace(input.TargetPlatform))
	if input.BusinessTaskID == "" {
		return TaskRunAssetPreflight{}, fmt.Errorf("%w: business task ID is required", ErrValidation)
	}
	if input.TargetPlatform == "" {
		return TaskRunAssetPreflight{}, fmt.Errorf("%w: target platform is required", ErrValidation)
	}
	// Same admission ordering as LaunchTaskRun: the production resolver can
	// probe up to 32 remote images on a 30s budget, so ineligible tenants
	// must fail closed before any catalog work.
	if !s.tenantStartAllowed(ctx, identity.TenantID) {
		return TaskRunAssetPreflight{}, fmt.Errorf("%w: image agent provider is unavailable for tenant", ErrCommandBlocked)
	}
	catalog, err := s.catalogs.Resolve(ctx, AssetCatalogScope{
		TenantID: identity.TenantID, OwnerUserID: identity.UserID,
		BusinessTaskID: input.BusinessTaskID, TargetPlatform: input.TargetPlatform,
	})
	if err != nil {
		return TaskRunAssetPreflight{}, fmt.Errorf("%w: resolve authorized image assets: %v", ErrValidation, err)
	}
	catalog, err = NormalizeAssetCatalog(catalog)
	if err != nil {
		return TaskRunAssetPreflight{}, fmt.Errorf("%w: normalize authorized image assets: %v", ErrValidation, err)
	}
	preflight := TaskRunAssetPreflight{
		BusinessTaskID: input.BusinessTaskID, TargetPlatform: input.TargetPlatform,
		Sources: []AuthorizedAsset{}, Styles: []AuthorizedAsset{},
	}
	for _, asset := range catalog.Assets {
		switch asset.Type {
		case AuthorizedAssetSource:
			preflight.Sources = append(preflight.Sources, asset)
		case AuthorizedAssetStyle:
			preflight.Styles = append(preflight.Styles, asset)
		}
	}
	return preflight, nil
}

// taskLaunchRunIdentity derives the durable run identity from a caller-stable
// request identity. Retries of one launch map to the same run, while a later
// intentional regeneration supplies a new request ID even when its generation
// payload is identical. Start still rejects reuse of one ID with a new payload.
func taskLaunchRunIdentity(identity ExecutionIdentity, input TaskRunLaunchInput) (runID, idempotencyKey string) {
	payload := strings.Join([]string{
		identity.TenantID, identity.UserID, input.BusinessTaskID, input.RequestID,
	}, "\x00")
	launchKey := uuid.NewSHA1(uuid.NameSpaceURL, []byte("task-processor:imageagent:task-launch:"+payload)).String()
	return "image-agent-" + launchKey, "image-agent-run-" + launchKey
}

func normalizeTaskLaunchStyleIDs(raw []string) ([]string, error) {
	seen := make(map[string]struct{}, len(raw))
	out := make([]string, 0, len(raw))
	for _, value := range raw {
		id := strings.TrimSpace(value)
		if id == "" {
			return nil, fmt.Errorf("style asset IDs cannot contain an empty value")
		}
		if _, duplicate := seen[id]; duplicate {
			continue
		}
		seen[id] = struct{}{}
		out = append(out, id)
	}
	return out, nil
}

// primaryTaskLaunchSource requires the caller to select one authorized source
// asset explicitly. Catalog ordering is crawler-provided and never establishes
// user intent, so no implicit fallback exists: a missing selection fails
// closed before any generation budget is spent.
func primaryTaskLaunchSource(catalog AssetCatalog, requested string) (string, error) {
	for _, asset := range catalog.Assets {
		if asset.Type == AuthorizedAssetSource && asset.ID == requested {
			return requested, nil
		}
	}
	return "", fmt.Errorf("source asset %q is not authorized for this business task", requested)
}

func validateTaskLaunchStyleIDs(catalog AssetCatalog, styleIDs []string) error {
	if len(styleIDs) == 0 {
		return nil
	}
	known := make(map[string]struct{}, len(catalog.Assets))
	for _, asset := range catalog.Assets {
		if asset.Type == AuthorizedAssetStyle {
			known[asset.ID] = struct{}{}
		}
	}
	for _, id := range styleIDs {
		if _, ok := known[id]; !ok {
			return fmt.Errorf("style asset %q is not authorized for this business task", id)
		}
	}
	return nil
}

// RestartFailed exposes the same-request recovery path using only immutable,
// owner-scoped inputs already persisted with the run.
func (s *Service) RestartFailed(ctx context.Context, runID string) error {
	identity, err := verifiedExecutionIdentity(ctx)
	if err != nil {
		return err
	}
	scope := RunScope{TenantID: identity.TenantID, OwnerUserID: identity.UserID, RunID: strings.TrimSpace(runID)}
	projection, err := s.repository.GetProjection(ctx, scope)
	if err != nil {
		return err
	}
	if projection.Run.Status != RunStatusFailed {
		return fmt.Errorf("%w: only a failed image agent run can be restarted", ErrCommandBlocked)
	}
	identity.BusinessTaskID = projection.Run.BusinessTaskID
	return s.startExistingProjection(ctx, projection, identity)
}

func (s *Service) startExistingProjection(ctx context.Context, projection RunProjection, identity ExecutionIdentity) error {
	// The workflow is already durable (Temporal USE_EXISTING conflict policy
	// keyed by run ID); StartManual on an existing projection is a re-dispatch
	// of the same workflow, not a new paid execution.
	if !s.tenantStartAllowed(ctx, identity.TenantID) {
		return fmt.Errorf("%w: image agent provider is unavailable for tenant", ErrCommandBlocked)
	}
	return s.workflows.StartManual(ctx, WorkflowStart{
		Run: projection.Run, Plan: projection.Plan, Identity: identity,
		MaxConcurrentSlots: projection.Run.MaxConcurrentSlots, AssetCatalog: projection.AssetCatalog,
	})
}

func (s *Service) tenantStartAllowed(ctx context.Context, tenantID string) bool {
	return s == nil || s.startGate == nil || s.startGate.AllowTenantStart(ctx, tenantID)
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
	if err := ValidateReplacementSubmittedPlan(expectedRevision, plan); err != nil {
		return fmt.Errorf("%w: validate replacement plan: %v", ErrValidation, err)
	}
	projection, err := s.repository.GetProjection(ctx, RunScope{TenantID: identity.TenantID, OwnerUserID: identity.UserID, RunID: strings.TrimSpace(runID)})
	if err != nil {
		return err
	}
	if projection.Run.Status == RunStatusBlocked && !BlockAllowsAction(projection.Run.Block, ActionEditPlan) {
		return fmt.Errorf("%w: the current block permits cancellation only", ErrCommandBlocked)
	}
	catalog, err := s.repository.GetAssetCatalog(ctx, RunScope{TenantID: identity.TenantID, OwnerUserID: identity.UserID, RunID: strings.TrimSpace(runID)})
	if err != nil {
		return err
	}
	if err := ValidateSubmittedPlanAgainstCatalog(plan, catalog); err != nil {
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

func (s *Service) RecoverEffect(ctx context.Context, runID, slotID string, attempt int, planRevision int64, actionID string) error {
	identity, err := s.commandIdentity(ctx, runID, planRevision, actionID)
	if err != nil {
		return err
	}
	slotID = strings.TrimSpace(slotID)
	if slotID == "" || attempt <= 0 {
		return fmt.Errorf("%w: recover slot ID and attempt are required", ErrValidation)
	}
	if err := ValidateArtifactKeyIdentifier(slotID); err != nil {
		return fmt.Errorf("%w: recover slot ID cannot be used in a durable artifact key", err)
	}
	runID = strings.TrimSpace(runID)
	projection, err := s.repository.GetProjection(ctx, RunScope{TenantID: identity.TenantID, OwnerUserID: identity.UserID, RunID: runID})
	if err != nil {
		return err
	}
	if projection.Run.Status != RunStatusBlocked {
		return fmt.Errorf("%w: only the current blocked effect can be recovered", ErrCommandBlocked)
	}
	if projection.Plan.Revision != planRevision {
		return fmt.Errorf("%w: recover effect plan revision is stale", ErrRevisionConflict)
	}
	if _, _, ok := FindRecoverableEffect(projection, slotID, attempt); !ok {
		return fmt.Errorf("%w: only external-effect recovery blocks support re-drive", ErrCommandBlocked)
	}
	return s.workflows.RecoverEffect(ctx, RecoverEffectCommand{
		RunID: runID, PlanRevision: planRevision, SlotID: slotID, Attempt: attempt,
		ActionID: strings.TrimSpace(actionID), Identity: identity, Projection: projection,
	})
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
	runID = strings.TrimSpace(runID)
	if runID == "" || ValidateActionID(actionID) != nil {
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
	if runID == "" || ValidateActionID(actionID) != nil {
		return ExecutionIdentity{}, fmt.Errorf("%w: run ID and action ID are required", ErrValidation)
	}
	if revision <= 0 || revision > MaxJSONSafePlanRevision {
		return ExecutionIdentity{}, fmt.Errorf("%w: positive JSON-safe plan revision is required", ErrValidation)
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
