package imageagent_test

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"task-processor/internal/agent"
	"task-processor/internal/agentconfig"
	"task-processor/internal/authidentity"
	"task-processor/internal/imageagent"
	"task-processor/internal/imageagent/store"
)

type setConfigurationFixture struct {
	template   agentconfig.SetTemplate
	snapshot   agentconfig.ImageConfigurationSnapshot
	receipt    *agentconfig.ImageRunAdmissionReceipt
	enabled    bool
	admitACK   error
	admissions int
}

func (c *setConfigurationFixture) PrepareImageConfiguration(_ context.Context, input agentconfig.ImageStartCommand) (agentconfig.ImageConfigurationSnapshot, error) {
	if !c.enabled {
		return agentconfig.ImageConfigurationSnapshot{}, agentconfig.ErrNotEnabled
	}
	c.snapshot = agentconfig.ImageConfigurationSnapshot{ID: "91d39d8d-3819-4a7c-ae7f-04ce91951f9a", Digest: strings.Repeat("a", 64), Scope: input.Scope, MemberID: input.MemberID, RequestKey: input.RequestKey, ContextID: input.ContextID, RunID: input.RunID, TargetPlatform: input.TargetPlatform, SourceDigest: input.SourceDigest, InputDigest: input.InputDigest, Parameters: c.template, ParametersDigest: strings.Repeat("b", 64), Epoch: "1", AgentVersion: agentconfig.ImageAgentVersion, HardLimits: input.HardLimits}
	return c.snapshot, nil
}
func (c *setConfigurationFixture) LoadImageConfiguration(context.Context, agent.Scope, agent.ConfigurationSnapshotRef) (agentconfig.ImageConfigurationSnapshot, error) {
	return c.snapshot, nil
}
func (c *setConfigurationFixture) ReadImageRunAdmission(context.Context, agent.Scope, agent.ConfigurationSnapshotRef) (agentconfig.ImageRunAdmissionReceipt, error) {
	if c.receipt == nil {
		return agentconfig.ImageRunAdmissionReceipt{}, agentconfig.ErrNotFound
	}
	return *c.receipt, nil
}
func (c *setConfigurationFixture) AdmitImageRun(_ context.Context, command agentconfig.ImageRunAdmissionCommand, _ agentconfig.ImageRunLimits) (agentconfig.ImageRunAdmissionReceipt, error) {
	if !c.enabled {
		return agentconfig.ImageRunAdmissionReceipt{}, agentconfig.ErrNotEnabled
	}
	c.admissions++
	now := time.Now().UTC().Truncate(time.Microsecond)
	receipt := agentconfig.ImageRunAdmissionReceipt{ID: "6be5e768-31c1-434e-a6c0-22a508fb3a35", Command: command, AdmittedAt: now, Deadline: now.Add(time.Duration(command.Limits.ElapsedSeconds) * time.Second)}
	raw, _ := json.Marshal(receipt)
	sum := sha256.Sum256(raw)
	receipt.Digest = hex.EncodeToString(sum[:])
	c.receipt = &receipt
	return receipt, c.admitACK
}

type setContextFixture struct {
	preparation     imageagent.ImageSetPreparation
	revalidationErr error
}

func (c *setContextFixture) ResolveImageSet(context.Context, imageagent.ExecutionIdentity, imageagent.PrepareImageSetInput) (imageagent.ImageSetPreparation, error) {
	return c.preparation, nil
}
func (c *setContextFixture) RevalidateImageSet(context.Context, imageagent.ExecutionIdentity, imageagent.RunProjection) error {
	return c.revalidationErr
}

type setQuoteFixture struct {
	quote imageagent.ImageGenerationQuote
}

func (q *setQuoteFixture) ReadImageGenerationQuote(context.Context, imageagent.ExecutionIdentity) (imageagent.ImageGenerationQuote, error) {
	return q.quote, nil
}

func TestPrepareImageSetRejectsAnotherSourceOwner(t *testing.T) {
	s, _, _, _, contexts, _, ctx, input := imageSetServiceFixture(t)
	input.ContextKind = imageagent.ImageSourceAcquisition
	contexts.preparation.Source.ContextKind = imageagent.ImageSourceSupply
	_, err := s.PrepareImageSet(ctx, input)
	require.ErrorIs(t, err, imageagent.ErrRevisionConflict)
}

func TestPrepareImageSetQuotesOnlyExplicitlySelectedTasks(t *testing.T) {
	s, _, workflows, _, _, _, ctx, input := imageSetServiceFixture(t)
	require.NoError(t, json.Unmarshal([]byte(`{"SelectedTaskIDs":["overview"]}`), &input))
	prepared, err := s.PrepareImageSet(ctx, input)
	require.NoError(t, err)
	require.Equal(t, 1, prepared.Images, "unselected tasks must not acquire a new generation identity or quote")
	require.Equal(t, "overview", prepared.Projection.Plan.Slots[0].ID)
	require.EqualValues(t, 12, prepared.Points)
	require.Zero(t, workflows.starts)
}

func TestRegenerationRequiresTheOriginalRunToHaveKnownClosedEffects(t *testing.T) {
	s, _, _, _, _, _, ctx, input := imageSetServiceFixture(t)
	first, err := s.PrepareImageSet(ctx, input)
	require.NoError(t, err)
	input.RequestID = "3b9ef503-8c6c-4a76-8eb8-047213c565cf"
	raw, err := json.Marshal(map[string]any{"RegenerateFromRunID": first.Projection.Run.ID, "SelectedTaskIDs": []string{"overview"}})
	require.NoError(t, err)
	require.NoError(t, json.Unmarshal(raw, &input))
	_, err = s.PrepareImageSet(ctx, input)
	require.ErrorIs(t, err, imageagent.ErrCommandBlocked, "an unfinished or UNKNOWN original request cannot be replaced by another dispatch")
}

func TestKnownClosedSubsetRegenerationCreatesANewAwaitingConfirmationRun(t *testing.T) {
	s, repo, workflows, config, _, _, ctx, input := imageSetServiceFixture(t)
	prepared, err := s.PrepareImageSet(ctx, input)
	require.NoError(t, err)
	parent, err := s.ConfirmImagePlan(ctx, confirmSetInput(prepared))
	require.NoError(t, err)
	parent.Run.Status = imageagent.RunStatusBlocked
	parent.Run.CurrentNode = "approve-results"
	parent.Run.Version++
	mutations := []imageagent.SlotProjectionMutation{}
	for i := range parent.Slots {
		parent.Slots[i].Slot.Status = imageagent.SlotStatusBlocked
		parent.Slots[i].ErrorCode = "budget_exceeded"
		parent.Slots[i].Closure = &imageagent.ImageSlotClosure{Kind: "not_dispatched"}
		slot := parent.Slots[i]
		mutations = append(mutations, imageagent.SlotProjectionMutation{PlanRevision: parent.Plan.Revision, Result: imageagent.SlotResult{SlotID: slot.Slot.ID, Status: slot.Slot.Status, ErrorCode: slot.ErrorCode, Closure: slot.Closure}, Projection: slot, Attempt: imageagent.StepAttempt{TenantID: parent.Run.TenantID, OwnerUserID: parent.Run.UserID, RunID: parent.Run.ID, PlanRevision: parent.Plan.Revision, SlotID: slot.Slot.ID, Node: "closed", IdempotencyKey: "closed:" + slot.Slot.ID, Outcome: "blocked", ErrorCategory: slot.ErrorCode}})
	}
	closedDigest, err := imageagent.ImageSetClosedEffectsDigest(parent.Plan, parent.Slots, nil)
	require.NoError(t, err)
	_, err = repo.CommitProjection(ctx, imageagent.ProjectionCommit{Scope: imageagent.ScopeForRun(parent.Run), CommitID: "controlled-closed-parent", ExpectedProjectionVersion: parent.ProjectionVersion, ExpectedRunVersion: parent.Run.Version - 1, Snapshot: parent, SlotMutations: mutations, RunMutation: &imageagent.RunMutation{Status: parent.Run.Status, CurrentNode: parent.Run.CurrentNode, ActivePlanRevision: parent.Plan.Revision}, EventType: "run.blocked", EventPayload: json.RawMessage(`{}`)})
	require.NoError(t, err)
	input.RequestID = "3b9ef503-8c6c-4a76-8eb8-047213c565cf"
	input.RegenerateFromRunID = parent.Run.ID
	input.SelectedTaskIDs = []string{"overview"}
	newRun, err := s.PrepareImageSet(ctx, input)
	require.NoError(t, err)
	require.NotEqual(t, parent.Run.ID, newRun.Projection.Run.ID)
	require.Equal(t, imageagent.RunStatusAwaitingPlanApproval, newRun.Projection.Run.Status)
	require.Len(t, newRun.Projection.Plan.Slots, 1)
	require.EqualValues(t, 12, newRun.Points)
	require.Equal(t, &imageagent.ImageSetRegeneration{RunID: parent.Run.ID, ClosedEffectsDigest: closedDigest}, newRun.Projection.Plan.Set.Regeneration)
	require.Len(t, workflows.starts, 1, "the new run cannot start before its own point confirmation")
	require.Equal(t, 1, config.admissions)
	stored, err := repo.GetProjection(ctx, imageagent.ScopeForRun(parent.Run))
	require.NoError(t, err)
	require.Equal(t, parent.ResultDigest, stored.ResultDigest, "original effects and receipts remain untouched")
}

func imageSetServiceFixture(t *testing.T) (*imageagent.Service, imageagent.Repository, *recordingWorkflowClient, *setConfigurationFixture, *setContextFixture, *setQuoteFixture, context.Context, imageagent.PrepareImageSetInput) {
	return imageSetServiceFixtureWithRepository(t, nil)
}
func imageSetServiceFixtureWithRepository(t *testing.T, wrap func(imageagent.Repository) imageagent.Repository) (*imageagent.Service, imageagent.Repository, *recordingWorkflowClient, *setConfigurationFixture, *setContextFixture, *setQuoteFixture, context.Context, imageagent.PrepareImageSetInput) {
	t.Helper()
	config := &setConfigurationFixture{enabled: true, template: agentconfig.SetTemplate{Schema: agentconfig.ImageConfigurationSchema, Mode: "standard", ShareOriginals: true, Background: "white", Language: "zh", Carousel: []agentconfig.ContentTask{{ID: "identity", Purpose: "product_identity"}}, Detail: []agentconfig.ContentTask{{ID: "overview", Purpose: "product_overview"}}}}
	catalog, err := imageagent.NormalizeAssetCatalog(imageagent.AssetCatalog{ProductContext: imageagent.ProductContextRef{ProductID: "product", Title: "Test product", SourceSnapshotVersion: 1}, Assets: []imageagent.AuthorizedAsset{{ID: "source-1", Type: imageagent.AuthorizedAssetSource, URL: "https://images.example.org/source.png", Width: 1024, Height: 1024}}})
	require.NoError(t, err)
	contexts := &setContextFixture{preparation: imageagent.ImageSetPreparation{Source: imageagent.ImageSourceBinding{ContextKind: imageagent.ImageSourceAcquisition, ProductID: "product", OperationID: "operation", OriginalPublicationID: "publication", OriginalVersion: 1, EffectiveVersion: 1, CatalogHash: catalog.Manifest.Hash}, Target: imageagent.ImageTarget{Platform: "product"}, Catalog: catalog, Observations: []imageagent.ImageSourceObservation{{AssetID: "source-1", SHA256: strings.Repeat("c", 64), Bytes: 10, Width: 1024, Height: 1024, MediaType: "image/png"}}}}
	quotes := &setQuoteFixture{quote: imageagent.ImageGenerationQuote{Provider: "grsai", Model: "gpt-image-2.5", Protocol: "grsai-json-sync-v1", Resolution: "1024x1024", Quality: "auto", PriceVersion: "price", Points: 12, RouteReference: "route", CredentialReference: "credential", ConfigurationVersion: "config"}}
	repository := store.NewMemoryRepository()
	if wrap != nil {
		repository = wrap(repository)
	}
	workflows := &recordingWorkflowClient{}
	service, err := imageagent.NewService(repository, workflows, staticCatalogResolver{catalog: catalog}, imageagent.WithOrganizationScope(), imageagent.WithImageSetDependencies(imageagent.ImageSetDependencies{Configuration: config, Contexts: contexts, Quotes: quotes, HardLimits: agentconfig.ImageRunLimits{Images: 32, Points: 10000, ElapsedSeconds: 3600}}))
	require.NoError(t, err)
	ctx := authidentity.WithAuthenticatedIdentity(context.Background(), authidentity.AuthenticatedIdentity{TenantID: "org", EffectiveOrganizationID: "org", UserID: "actor", EffectiveMemberID: "member"})
	input := imageagent.PrepareImageSetInput{ContextKind: imageagent.ImageSourceAcquisition, RequestID: "c5fd7c43-138c-43dc-8dfb-6f9be74a5e9e", ContextID: "operation", Target: imageagent.ImageTargetSelection{Platform: "product"}, SharedOriginalIDs: []string{"source-1"}}
	return service, repository, workflows, config, contexts, quotes, ctx, input
}

func confirmSetInput(prepared imageagent.PreparedImageSet) imageagent.ConfirmImagePlanInput {
	return imageagent.ConfirmImagePlanInput{RunID: prepared.Projection.Run.ID, ActionID: "b912e7d4-df80-44a5-8510-3cdf91d5b8dd", ExpectedRevision: 1, PlanDigest: prepared.PlanDigest, QuoteDigest: prepared.QuoteDigest}
}

func TestConfirmImageSetRechecksTenantAdmissionButRecoversOriginalReceipts(t *testing.T) {
	for _, stage := range []string{"new", "run_receipt", "configuration_receipt"} {
		t.Run(stage, func(t *testing.T) {
			var wrap func(imageagent.Repository) imageagent.Repository
			if stage == "configuration_receipt" {
				wrap = func(r imageagent.Repository) imageagent.Repository {
					return &confirmCommitFailure{Repository: r, fail: true}
				}
			}
			service, repo, workflows, config, contexts, quotes, ctx, input := imageSetServiceFixtureWithRepository(t, wrap)
			prepared, err := service.PrepareImageSet(ctx, input)
			require.NoError(t, err)
			command := confirmSetInput(prepared)
			if stage != "new" {
				_, err = service.ConfirmImagePlan(ctx, command)
				if stage == "configuration_receipt" {
					require.ErrorContains(t, err, "confirm persistence unavailable")
				} else {
					require.NoError(t, err)
				}
			}
			starts := len(workflows.starts)
			restarted, err := imageagent.NewService(repo, workflows, staticCatalogResolver{catalog: contexts.preparation.Catalog},
				imageagent.WithOrganizationScope(),
				imageagent.WithTenantStartGate(imageagent.TenantAllowlistStartGate{Enabled: true, AllowedTenantIDs: []string{"another-org"}}),
				imageagent.WithImageSetDependencies(imageagent.ImageSetDependencies{Configuration: config, Contexts: contexts, Quotes: quotes, HardLimits: agentconfig.ImageRunLimits{Images: 32, Points: 10000, ElapsedSeconds: 3600}}))
			require.NoError(t, err)
			resumed, err := restarted.ConfirmImagePlan(ctx, command)
			if stage == "new" {
				require.ErrorIs(t, err, imageagent.ErrCommandBlocked)
				require.Zero(t, config.admissions)
				require.Empty(t, workflows.starts)
				stored, readErr := repo.GetProjection(ctx, imageagent.ScopeForRun(prepared.Projection.Run))
				require.NoError(t, readErr)
				require.Equal(t, imageagent.RunStatusAwaitingPlanApproval, stored.Run.Status)
				require.Nil(t, stored.Run.ImageAdmission)
				return
			}
			require.NoError(t, err)
			require.Equal(t, *config.receipt, *resumed.Run.ImageAdmission)
			require.Equal(t, 1, config.admissions)
			require.Len(t, workflows.starts, starts+1)
			require.Equal(t, command.ActionID, workflows.starts[starts].Run.ImageAdmission.Command.ConfirmActionID)
		})
	}
}

func TestPrepareFullImageSetSharesOnlyOriginalsAndRequiresConfirmation(t *testing.T) {
	s, _, workflows, config, _, _, ctx, input := imageSetServiceFixture(t)
	prepared, err := s.PrepareImageSet(ctx, input)
	require.NoError(t, err)
	require.Equal(t, 2, prepared.Images)
	require.EqualValues(t, 24, prepared.Points)
	require.Equal(t, imageagent.RunStatusAwaitingPlanApproval, prepared.Projection.Run.Status)
	require.Empty(t, workflows.starts)
	require.Zero(t, config.admissions)
	require.Equal(t, prepared.Projection.Plan.Slots[0].Recipe.References, prepared.Projection.Plan.Slots[1].Recipe.References)
	require.NotEqual(t, prepared.Projection.Plan.Slots[0].ID, prepared.Projection.Plan.Slots[1].ID)
	replay, err := s.PrepareImageSet(ctx, input)
	require.NoError(t, err)
	require.Equal(t, prepared, replay)
	input.SharedOriginalIDs = []string{"source-2"}
	_, err = s.PrepareImageSet(ctx, input)
	require.ErrorIs(t, err, imageagent.ErrRevisionConflict)
}

func TestConfirmImageSetUsesOriginalReceiptAfterAdmissionOrStartACKLoss(t *testing.T) {
	s, repo, workflows, config, _, _, ctx, input := imageSetServiceFixture(t)
	prepared, err := s.PrepareImageSet(ctx, input)
	require.NoError(t, err)
	config.admitACK = errors.New("admission ACK lost")
	workflows.startErr = errors.New("Temporal start ACK lost")
	confirmed, err := s.ConfirmImagePlan(ctx, confirmSetInput(prepared))
	require.ErrorContains(t, err, "start ACK lost")
	require.Equal(t, imageagent.RunStatusExecuting, confirmed.Run.Status)
	require.Equal(t, 1, config.admissions)
	original := *confirmed.Run.ImageAdmission
	config.enabled = false
	workflows.startErr = nil
	replayed, err := s.ConfirmImagePlan(ctx, confirmSetInput(prepared))
	require.NoError(t, err)
	require.Equal(t, original, *replayed.Run.ImageAdmission)
	require.Equal(t, 1, config.admissions)
	require.Len(t, workflows.starts, 2)
	require.Equal(t, workflows.starts[0], workflows.starts[1])
	stored, err := repo.GetProjection(ctx, imageagent.ScopeForRun(confirmed.Run))
	require.NoError(t, err)
	require.Equal(t, &original, stored.Run.ImageAdmission)
}

func TestConfirmImageSetRejectsChangedPriceSourceOrDisabledEnterprise(t *testing.T) {
	for _, kind := range []string{"price", "source", "disabled", "member"} {
		t.Run(kind, func(t *testing.T) {
			s, _, workflows, config, contexts, quotes, ctx, input := imageSetServiceFixture(t)
			prepared, err := s.PrepareImageSet(ctx, input)
			require.NoError(t, err)
			switch kind {
			case "price":
				quotes.quote.Points++
			case "source":
				contexts.revalidationErr = imageagent.ErrRevisionConflict
			case "disabled":
				config.enabled = false
			case "member":
				ctx = authidentity.WithAuthenticatedIdentity(ctx, authidentity.AuthenticatedIdentity{TenantID: "org", EffectiveOrganizationID: "org", UserID: "actor", EffectiveMemberID: "replacement"})
			}
			_, err = s.ConfirmImagePlan(ctx, confirmSetInput(prepared))
			require.Error(t, err)
			require.Empty(t, workflows.starts)
			require.Zero(t, config.admissions)
		})
	}
}

func TestConfirmImageSetRestoresExpiredOriginalStartWithoutExtendingBudget(t *testing.T) {
	for _, kind := range []string{"run_receipt", "config_receipt"} {
		t.Run(kind, func(t *testing.T) {
			service, repo, workflows, config, contexts, quotes, ctx, input := imageSetServiceFixtureWithRepository(t, func(r imageagent.Repository) imageagent.Repository {
				return &confirmCommitFailure{Repository: r, fail: kind == "config_receipt"}
			})
			prepared, err := service.PrepareImageSet(ctx, input)
			require.NoError(t, err)
			command := confirmSetInput(prepared)
			workflows.startErr = errors.New("Temporal unavailable")
			_, err = service.ConfirmImagePlan(ctx, command)
			require.Error(t, err)
			original := *imageagent.CloneImageAdmission(config.receipt)
			config.enabled = false
			quotes.quote.Points++
			workflows.startErr = nil
			restarted, err := imageagent.NewService(repo, workflows, staticCatalogResolver{catalog: contexts.preparation.Catalog}, imageagent.WithOrganizationScope(), imageagent.WithTenantStartGate(imageagent.TenantAllowlistStartGate{Enabled: true, AllowedTenantIDs: []string{"another-org"}}), imageagent.WithImageSetDependencies(imageagent.ImageSetDependencies{Configuration: config, Contexts: contexts, Quotes: quotes, HardLimits: agentconfig.ImageRunLimits{Images: 32, Points: 10000, ElapsedSeconds: 3600}, Now: func() time.Time { return original.Deadline.Add(time.Second) }}))
			require.NoError(t, err)
			resumed, err := restarted.ConfirmImagePlan(ctx, command)
			require.NoError(t, err, "the original owner must get the expired immutable input to close it without dispatch")
			require.Equal(t, original, *resumed.Run.ImageAdmission)
			require.Equal(t, original.AdmittedAt, resumed.Run.StartedAt)
			require.Equal(t, original.Deadline, resumed.Run.StartedAt.Add(resumed.Run.Budget.MaxElapsed))
			require.Equal(t, 1, config.admissions)
			last := workflows.starts[len(workflows.starts)-1]
			require.Equal(t, original, *last.Run.ImageAdmission)
			require.Equal(t, prepared.Projection.Plan, last.Plan)
			if kind == "run_receipt" {
				require.Len(t, workflows.starts, 2)
				require.Equal(t, workflows.starts[0], workflows.starts[1])
			} else {
				require.Len(t, workflows.starts, 1)
			}
		})
	}
}

func TestExpiredImageSetConfirmationDoesNotReviveClosedWorkflowProgress(t *testing.T) {
	for _, status := range []imageagent.RunStatus{imageagent.RunStatusBlocked, imageagent.RunStatusFailed} {
		t.Run(string(status), func(t *testing.T) {
			service, repo, workflows, config, contexts, quotes, ctx, input := imageSetServiceFixture(t)
			prepared, err := service.PrepareImageSet(ctx, input)
			require.NoError(t, err)
			command := confirmSetInput(prepared)
			current, err := service.ConfirmImagePlan(ctx, command)
			require.NoError(t, err)
			next := current
			next.Run.Status, next.Run.Version = status, current.Run.Version+1
			scope := imageagent.ScopeForRun(current.Run)
			original, err := repo.CommitProjection(ctx, imageagent.ProjectionCommit{Scope: scope, CommitID: "workflow-progress", ExpectedProjectionVersion: current.ProjectionVersion, ExpectedRunVersion: current.Run.Version, Snapshot: next, RunMutation: &imageagent.RunMutation{Status: status, CurrentNode: next.Run.CurrentNode, ActivePlanRevision: next.Plan.Revision}, EventType: "run.updated", EventPayload: []byte(`{}`)})
			require.NoError(t, err)
			restarted, err := imageagent.NewService(repo, workflows, staticCatalogResolver{catalog: contexts.preparation.Catalog}, imageagent.WithOrganizationScope(), imageagent.WithImageSetDependencies(imageagent.ImageSetDependencies{Configuration: config, Contexts: contexts, Quotes: quotes, HardLimits: agentconfig.ImageRunLimits{Images: 32, Points: 10000, ElapsedSeconds: 3600}, Now: func() time.Time { return config.receipt.Deadline.Add(time.Second) }}))
			require.NoError(t, err)
			resumed, err := restarted.ConfirmImagePlan(ctx, command)
			require.NoError(t, err)
			require.Equal(t, original, resumed)
			require.Len(t, workflows.starts, 1, "confirmation must not start another execution after workflow progress is already closed or blocked")
		})
	}
}

func TestRestartFailedImageSetRestoresOriginalAdmissionAfterSourceRevocation(t *testing.T) {
	for _, revoked := range []bool{false, true} {
		t.Run(map[bool]string{false: "original_after_deadline", true: "source_revoked"}[revoked], func(t *testing.T) {
			service, repo, workflows, config, contexts, quotes, ctx, input := imageSetServiceFixture(t)
			prepared, err := service.PrepareImageSet(ctx, input)
			require.NoError(t, err)
			current, err := service.ConfirmImagePlan(ctx, confirmSetInput(prepared))
			require.NoError(t, err)
			next := current
			next.Run.Status, next.Run.CurrentNode, next.Run.Version = imageagent.RunStatusFailed, "workflow_failed", current.Run.Version+1
			original, err := repo.CommitProjection(ctx, imageagent.ProjectionCommit{Scope: imageagent.ScopeForRun(current.Run), CommitID: "workflow-failed", ExpectedProjectionVersion: current.ProjectionVersion, ExpectedRunVersion: current.Run.Version, Snapshot: next, RunMutation: &imageagent.RunMutation{Status: next.Run.Status, CurrentNode: next.Run.CurrentNode, ActivePlanRevision: next.Plan.Revision}, EventType: "run.failed", EventPayload: []byte(`{}`)})
			require.NoError(t, err)
			config.enabled = false
			quotes.quote.PriceVersion = "current-price-changed"
			if revoked {
				contexts.revalidationErr = imageagent.ErrRevisionConflict
			}
			restarted, err := imageagent.NewService(repo, workflows, staticCatalogResolver{catalog: contexts.preparation.Catalog}, imageagent.WithOrganizationScope(), imageagent.WithTenantStartGate(imageagent.TenantAllowlistStartGate{Enabled: true, AllowedTenantIDs: []string{"another-org"}}), imageagent.WithImageSetDependencies(imageagent.ImageSetDependencies{Configuration: config, Contexts: contexts, Quotes: quotes, HardLimits: agentconfig.ImageRunLimits{Images: 32, Points: 10000, ElapsedSeconds: 3600}, Now: func() time.Time { return config.receipt.Deadline.Add(time.Second) }}))
			require.NoError(t, err)
			err = restarted.RestartFailed(ctx, original.Run.ID)
			require.NoError(t, err, "the original workflow must close undispatched slots after its live source check")
			require.Len(t, workflows.starts, 2)
			require.Equal(t, original.Run, workflows.starts[1].Run)
			require.Equal(t, original.Plan, workflows.starts[1].Plan)
			require.Equal(t, original.AssetCatalog, workflows.starts[1].AssetCatalog)
			require.Equal(t, workflows.starts[0].Identity, workflows.starts[1].Identity)
			require.Equal(t, 1, config.admissions)
			stored, err := repo.GetProjection(ctx, imageagent.ScopeForRun(original.Run))
			require.NoError(t, err)
			require.Equal(t, original, stored, "restart only restores the original workflow; its owner controls projection transitions")
		})
	}
}

func TestConfirmAdmittedImageSetRestoresOriginalStartAfterSourceChange(t *testing.T) {
	for _, kind := range []string{"run_receipt", "config_receipt"} {
		t.Run(kind, func(t *testing.T) {
			service, repo, workflows, config, contexts, _, ctx, input := imageSetServiceFixtureWithRepository(t, func(r imageagent.Repository) imageagent.Repository {
				return &confirmCommitFailure{Repository: r, fail: kind == "config_receipt"}
			})
			prepared, err := service.PrepareImageSet(ctx, input)
			require.NoError(t, err)
			command := confirmSetInput(prepared)
			workflows.startErr = errors.New("Temporal start unavailable")
			_, err = service.ConfirmImagePlan(ctx, command)
			require.Error(t, err)
			original := *imageagent.CloneImageAdmission(config.receipt)
			contexts.revalidationErr = imageagent.ErrRevisionConflict
			config.enabled = false
			workflows.startErr = nil
			resumed, err := service.ConfirmImagePlan(ctx, command)
			require.NoError(t, err, "source changes cannot prevent recovery of an already admitted immutable input")
			require.Equal(t, &original, resumed.Run.ImageAdmission)
			require.Equal(t, 1, config.admissions)
			last := workflows.starts[len(workflows.starts)-1]
			require.Equal(t, prepared.Projection.Plan, last.Plan)
			require.Equal(t, prepared.Projection.AssetCatalog, last.AssetCatalog)
			require.Equal(t, original, *last.Run.ImageAdmission)
			require.Equal(t, original.Deadline, last.Run.StartedAt.Add(last.Run.Budget.MaxElapsed))
			stored, err := repo.GetProjection(ctx, imageagent.ScopeForRun(resumed.Run))
			require.NoError(t, err)
			require.Equal(t, &original, stored.Run.ImageAdmission)
		})
	}
}

func TestPrepareImageSetMissingSpecificationsBlocksBeforeGeneration(t *testing.T) {
	s, _, workflows, config, _, _, ctx, input := imageSetServiceFixture(t)
	config.template.Detail = []agentconfig.ContentTask{{ID: "dimensions", Purpose: "specification_dimensions"}}
	_, err := s.PrepareImageSet(ctx, input)
	require.ErrorIs(t, err, imageagent.ErrValidation)
	require.ErrorContains(t, err, "specifications evidence")
	require.Empty(t, workflows.starts)
}

func TestPrepareImageSetRejectsAResolvedDifferentStore(t *testing.T) {
	s, _, workflows, config, contexts, _, ctx, input := imageSetServiceFixture(t)
	input.Target = imageagent.ImageTargetSelection{Platform: "shein", RecordID: "record", StoreID: "chosen-store", Site: "shein-us", CategoryID: 1}
	contexts.preparation.Target = imageagent.ImageTarget{Platform: "shein", RecordID: "record", StoreID: "different-store", Site: "shein-us", CategoryID: 1, ProductTypeID: 2, ApplicationID: "application", ApplicationMode: "fully_managed", AttributesDigest: strings.Repeat("d", 64), VariantsDigest: strings.Repeat("e", 64), RequirementDigest: strings.Repeat("f", 64), RequirementVersion: "current"}
	contexts.preparation.OfficialPlacements = map[string]imageagent.OfficialImagePlacement{"identity": {Group: "skc", Type: 1, Sort: 1, Site: "shein-us"}, "overview": {Group: "detail", Type: 7, Sort: 1, Site: "shein-us"}}
	_, err := s.PrepareImageSet(ctx, input)
	require.ErrorIs(t, err, imageagent.ErrRevisionConflict)
	require.Empty(t, workflows.starts)
	require.Empty(t, config.snapshot.ID)
}

func TestPrepareImageSetBindsExplicitOfficialPositionsToEachConfiguredTask(t *testing.T) {
	for _, mode := range []string{"exact", "changed", "missing", "extra", "generic_claim"} {
		t.Run(mode, func(t *testing.T) {
			s, _, _, configuration, contexts, _, ctx, input := imageSetServiceFixture(t)
			configuration.template.Detail = nil
			position := imageagent.OfficialImagePlacement{Group: "skc", Type: 1, Sort: 1, Site: "shein-us"}
			input.Target = imageagent.ImageTargetSelection{Platform: "shein", RecordID: "record", StoreID: "store", Site: "shein-us", CategoryID: 1}
			input.OfficialPlacements = map[string]imageagent.OfficialImagePlacement{"identity": position}
			contexts.preparation.Target = imageagent.ImageTarget{Platform: "shein", RecordID: "record", StoreID: "store", Site: "shein-us", ApplicationID: "application", ApplicationMode: "self_operated", CategoryID: 1, ProductTypeID: 2, AttributesDigest: strings.Repeat("a", 64), VariantsDigest: strings.Repeat("b", 64), RequirementDigest: strings.Repeat("c", 64), RequirementVersion: "shein-images-v1"}
			contexts.preparation.OfficialPlacements = map[string]imageagent.OfficialImagePlacement{"identity": position}
			switch mode {
			case "changed":
				changed := position
				changed.Type = 2
				input.OfficialPlacements["identity"] = changed
			case "missing":
				input.OfficialPlacements = nil
			case "extra":
				input.OfficialPlacements["unused"] = position
			case "generic_claim":
				input.Target = imageagent.ImageTargetSelection{Platform: "product"}
				contexts.preparation.Target = imageagent.ImageTarget{Platform: "product"}
			}
			prepared, err := s.PrepareImageSet(ctx, input)
			if mode == "exact" {
				require.NoError(t, err)
				require.Equal(t, &position, prepared.Projection.Plan.Slots[0].Recipe.OfficialPlacement)
			} else {
				require.Error(t, err)
			}
		})
	}
}

func TestPreparedSetCannotBypassConfirmationThroughOriginalStart(t *testing.T) {
	s, _, workflows, _, _, _, ctx, input := imageSetServiceFixture(t)
	prepared, err := s.PrepareImageSet(ctx, input)
	require.NoError(t, err)
	err = s.Start(ctx, imageagent.StartRunInput{RunID: "bypass", BusinessTaskID: "operation", TargetPlatform: "product", ImagePolicyContext: imageagent.ImagePolicyContext{Country: "zz", Family: "default", SceneCategory: "general"}, Mode: imageagent.RunModeManual, IdempotencyKey: "bypass", Plan: prepared.Projection.Plan, Budget: prepared.Projection.Run.Budget})
	require.ErrorIs(t, err, imageagent.ErrCommandBlocked)
	require.Empty(t, workflows.starts)
}

func TestCancelPreparedSetDoesNotNeedTemporalOrReviveOnConfirmation(t *testing.T) {
	s, repo, workflows, config, _, _, ctx, input := imageSetServiceFixture(t)
	prepared, err := s.PrepareImageSet(ctx, input)
	require.NoError(t, err)
	require.NoError(t, s.Cancel(ctx, prepared.Projection.Run.ID, 1, "cancel-prepared"))
	stored, err := repo.GetProjection(ctx, imageagent.ScopeForRun(prepared.Projection.Run))
	require.NoError(t, err)
	require.Equal(t, imageagent.RunStatusCancelled, stored.Run.Status)
	require.Empty(t, workflows.cancellations)
	_, err = s.ConfirmImagePlan(ctx, confirmSetInput(prepared))
	require.ErrorIs(t, err, imageagent.ErrCommandBlocked)
	require.Zero(t, config.admissions)
}

type confirmCommitFailure struct {
	imageagent.Repository
	fail bool
}

func (r *confirmCommitFailure) CommitProjection(ctx context.Context, commit imageagent.ProjectionCommit) (imageagent.RunProjection, error) {
	if r.fail && strings.HasPrefix(commit.CommitID, "confirm:") {
		r.fail = false
		return imageagent.RunProjection{}, errors.New("confirm persistence unavailable")
	}
	return r.Repository.CommitProjection(ctx, commit)
}
func TestConfirmConsumesTheOriginalAdmissionAfterImageCommitFailureAndCurrentPriceChange(t *testing.T) {
	service, repo, workflows, config, _, quotes, ctx, input := imageSetServiceFixtureWithRepository(t, func(r imageagent.Repository) imageagent.Repository {
		return &confirmCommitFailure{Repository: r, fail: true}
	})
	prepared, err := service.PrepareImageSet(ctx, input)
	require.NoError(t, err)
	command := confirmSetInput(prepared)
	_, err = service.ConfirmImagePlan(ctx, command)
	require.ErrorContains(t, err, "confirm persistence unavailable")
	require.Equal(t, 1, config.admissions)
	require.Empty(t, workflows.starts)
	original := *config.receipt
	projection, err := repo.GetProjection(ctx, imageagent.ScopeForRun(prepared.Projection.Run))
	require.NoError(t, err)
	require.Nil(t, projection.Run.ImageAdmission)
	config.enabled = false
	quotes.quote.Points++
	resumed, err := service.ConfirmImagePlan(ctx, command)
	require.NoError(t, err)
	require.Equal(t, original, *resumed.Run.ImageAdmission)
	require.Equal(t, 1, config.admissions)
	require.Len(t, workflows.starts, 1)
	changed := command
	changed.ActionID = "731f0fcf-39ac-4f7b-9057-16fef9b20cf4"
	_, err = service.ConfirmImagePlan(ctx, changed)
	require.Error(t, err)
	require.Len(t, workflows.starts, 1)
}
