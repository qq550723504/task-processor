package temporal

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"image"
	"image/png"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	"task-processor/internal/agent"
	"task-processor/internal/agentconfig"
	"task-processor/internal/imageagent"
	"task-processor/internal/imageagent/store"
)

func imageSetPersistenceFixture(t *testing.T) (*Activities, imageagent.Repository, ExecuteSlotV3ActivityInput) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "image.db")), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	require.NoError(t, err)
	require.NoError(t, store.AutoMigrateOrganizationScope(db))
	pool, err := db.DB()
	require.NoError(t, err)
	t.Cleanup(func() { _ = pool.Close() })
	repo := store.NewOrganizationRepository(db)
	hash := strings.Repeat("a", 64)
	run := imageagent.Run{ID: "392ed0a2-0f01-4c94-9aa4-eb50271fae9c", TenantID: "org", UserID: "actor", MemberID: "member", BusinessTaskID: "source-operation", ScopeProtocol: imageagent.OrganizationScopeProtocol, TargetPlatform: "product", Mode: imageagent.RunModeManual, IdempotencyKey: "prepare", Status: imageagent.RunStatusPlanning, CurrentNode: "plan", Version: 1, ActivePlanRevision: 1, MaxConcurrentSlots: 1}
	catalog, err := imageagent.NormalizeAssetCatalog(imageagent.AssetCatalog{ProductContext: imageagent.ProductContextRef{ProductID: "product"}, Assets: []imageagent.AuthorizedAsset{{ID: "source-1", Type: imageagent.AuthorizedAssetSource, URL: "https://images.example.org/source.png"}}})
	require.NoError(t, err)
	slot := imageagent.Slot{ID: "overview", Role: imageagent.SlotRoleDetail, SourceAssetIDs: []string{"source-1"}, IdempotencyKey: "overview", Status: imageagent.SlotStatusPending, Recipe: &imageagent.ImageSlotRecipe{Purpose: "product_overview", Background: "white", Language: "zh", Placement: imageagent.ImagePlacement{Group: "detail", Order: 1}, PromptVersion: imageagent.ImageSetSchema, Prompt: "Show the exact original product.", References: []imageagent.ImageSourceObservation{{AssetID: "source-1", SHA256: hash, Bytes: 100, Width: 1024, Height: 1024, MediaType: "image/png"}}, Quote: imageagent.ImageGenerationQuote{Provider: "grsai", Model: "gpt-image-2.5", Protocol: "grsai-json-sync-v1", Resolution: "1024x1024", Quality: "auto", PriceVersion: "price", Points: 12, RouteReference: "route", CredentialReference: "credential", ConfigurationVersion: "config"}}}
	plan := imageagent.Plan{Revision: 1, IdempotencyKey: "plan", SourceAssetIDs: []string{"source-1"}, CreatedBy: "actor", Slots: []imageagent.Slot{slot}, Set: &imageagent.ImageSetPlan{Schema: imageagent.ImageSetSchema, Source: imageagent.ImageSourceBinding{ContextKind: imageagent.ImageSourceAcquisition, ProductID: "product", OperationID: run.BusinessTaskID, OriginalPublicationID: "publication", OriginalVersion: 1, EffectiveVersion: 1, CatalogHash: catalog.Manifest.Hash}, Target: imageagent.ImageTarget{Platform: "product"}, Configuration: agent.ConfigurationSnapshotRef{Kind: agentconfig.SnapshotKind, ID: "9e7afaa9-a9f9-48ba-a11a-b5bb377f08e9", Digest: hash}, ConfigurationEpoch: "1", ParametersDigest: hash, InputDigest: hash, MaxPoints: 12}}
	plan.Set.QuoteDigest, err = imageagent.ImageSetQuoteDigest(plan)
	require.NoError(t, err)
	limits := agentconfig.ImageRunLimits{Images: 1, Points: 12, ElapsedSeconds: 3600}
	run.Budget = imageagent.ImageSetBudget(limits)
	run.StartedAt = time.Now().UTC().Truncate(time.Microsecond)
	planDigest, err := imageagent.ImageSetPlanDigest(plan)
	require.NoError(t, err)
	receipt := agentconfig.ImageRunAdmissionReceipt{ID: "74e01be8-f4e6-461f-8acb-18fe37329f1c", AdmittedAt: run.StartedAt, Deadline: run.StartedAt.Add(time.Hour), Command: agentconfig.ImageRunAdmissionCommand{Scope: agent.Scope{OrganizationID: run.TenantID, ActorID: run.UserID}, Snapshot: plan.Set.Configuration, MemberID: run.MemberID, RunID: run.ID, ConfirmActionID: "1b912e40-d50a-48f5-a12b-62c73a42e4b8", SourceDigest: imageagent.ImageSetSourceDigest(plan.Set.Source, plan), InputDigest: plan.Set.InputDigest, PlanDigest: planDigest, QuoteDigest: plan.Set.QuoteDigest, Limits: limits}}
	receiptBytes, err := json.Marshal(receipt)
	require.NoError(t, err)
	receiptSum := sha256.Sum256(receiptBytes)
	receipt.Digest = hex.EncodeToString(receiptSum[:])
	run.ImageAdmission = &receipt
	projection, err := repo.InitializeRun(context.Background(), imageagent.ProjectionInitialization{Scope: imageagent.ScopeForRun(run), Run: run, Plan: plan, Catalog: catalog, Snapshot: imageagent.RunProjection{Run: run, Plan: plan}, CommitID: "prepare", EventType: "run.initialized", EventPayload: []byte(`{}`)})
	require.NoError(t, err)
	input := ExecuteSlotV3ActivityInput{RunID: run.ID, Identity: imageagent.ExecutionIdentity{RunID: run.ID, ScopeProtocol: run.ScopeProtocol, TenantID: run.TenantID, UserID: run.UserID, MemberID: run.MemberID, BusinessTaskID: run.BusinessTaskID}, TargetPlatform: "product", PlanRevision: 1, Slot: slot, Attempt: 1, ImageSet: plan.Set, AssetCatalog: projection.AssetCatalog, IdempotencyKey: slotAttemptKey(1, slot, 1)}
	activities := newV3Activities(t, repo, repo.(imageagent.SlotExternalEffectV3Repository), &recordingStagedExecutor{}, &recordingArtifactStore{})
	activities.executionAuthorizer = allowGenerationExecution{}
	return activities, repo, input
}

type settledSetExecutor struct {
	*recordingStagedExecutor
	onGenerate func(context.Context, imageagent.SlotExecutionInput)
}

func (e *settledSetExecutor) GenerateSlot(ctx context.Context, input imageagent.SlotExecutionInput) (imageagent.SlotGeneratedOutput, error) {
	e.onGenerate(ctx, input)
	return e.recordingStagedExecutor.GenerateSlot(ctx, input)
}

func TestPersistImageSetAcceptedOutputKeepsOriginalGenerationProof(t *testing.T) {
	a, repo, input := imageSetPersistenceFixture(t)
	var pngBytes bytes.Buffer
	require.NoError(t, png.Encode(&pngBytes, image.NewNRGBA(image.Rect(0, 0, 1024, 1024))))
	recorder := &recordingStagedExecutor{generated: imageagent.SlotGeneratedOutput{SlotID: input.Slot.ID, Attempt: 1, SourceAssetID: "source-1", Assets: []imageagent.GeneratedAsset{{Bytes: pngBytes.Bytes(), ContentType: "image/png", Width: 1024, Height: 1024, Operations: []string{"render_source_edit"}, ProviderReceiptID: "response"}}}}
	recorder.mutateResult = func(result *imageagent.SlotExecutionResult) {
		for index := range result.Candidates {
			result.Candidates[index].Width, result.Candidates[index].Height = 1024, 1024
			result.Candidates[index].Operations = []string{"render_source_edit"}
		}
	}
	var originalFact imageagent.GenerationFact
	a.stagedSlotExecutor = &settledSetExecutor{recordingStagedExecutor: recorder, onGenerate: func(ctx context.Context, execution imageagent.SlotExecutionInput) {
		quote := execution.Slot.Recipe.Quote
		digest, err := imageagent.ImageSlotGenerationInputDigest(execution)
		require.NoError(t, err)
		intent := imageagent.GenerationIntent{Identity: slotEffectReservationV3(execution).Identity, MemberID: input.Identity.MemberID, CatalogHash: execution.AssetCatalog.Manifest.Hash, SourceDigest: imageagent.ImageSourceBundleDigest(execution.Slot.Recipe.References), PromptVersion: execution.Slot.Recipe.PromptVersion, InputProtocol: imageagent.ImageSetSchema, InputDigest: digest, RouteReference: quote.RouteReference, CredentialReference: quote.CredentialReference, ConfigurationVersion: quote.ConfigurationVersion, Provider: quote.Provider, Model: quote.Model, Protocol: quote.Protocol, Resolution: quote.Resolution, Quality: quote.Quality, PriceVersion: quote.PriceVersion, Points: quote.Points, LimitVersion: 1, MonthStart: time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)}
		facts := repo.(imageagent.GenerationFactRepository)
		fact, err := facts.PrepareGenerationIntent(ctx, intent)
		require.NoError(t, err)
		fact, err = facts.BindGenerationReservation(ctx, intent, imageagent.GenerationReservationReceipt{IntentID: fact.IntentID, Fingerprint: fact.Fingerprint, OrganizationID: input.Identity.TenantID, MemberID: input.Identity.MemberID, OperationID: "image-reserve:" + fact.IntentID, ReservationID: "reservation", ResourceType: "ai_point", Points: intent.Points, PriceVersion: intent.PriceVersion, LimitVersion: intent.LimitVersion, MonthStart: intent.MonthStart})
		require.NoError(t, err)
		_, won, err := facts.BeginGenerationDispatch(ctx, intent)
		require.NoError(t, err)
		require.True(t, won)
		fact, err = facts.RecordGenerationSuccess(ctx, intent, imageagent.GenerationSuccess{ResponseID: "response", ResultDigest: strings.Repeat("b", 64), ResultURL: "https://images.example.org/output.png"})
		require.NoError(t, err)
		originalFact, err = facts.BindGenerationSettlement(ctx, intent, imageagent.GenerationSettlementReceipt{IntentID: fact.IntentID, Fingerprint: fact.Fingerprint, OperationID: "image-finalize:" + fact.IntentID, ReservationID: "reservation", State: "committed", Points: intent.Points, ProofDigest: fact.TerminalProofDigest()})
		require.NoError(t, err)
	}}
	published, err := a.ExecuteSlotV3(context.Background(), input)
	require.NoError(t, err)
	projection, err := a.PersistImageSetSlotResult(context.Background(), PersistSlotResultV3ActivityInput{RunID: input.RunID, Identity: input.Identity, PlanRevision: 1, AttemptKey: input.IdempotencyKey, Result: SlotWorkflowV3Result{Published: published, Status: imageagent.SlotStatusAccepted, EffectPhase: imageagent.SlotEffectV3PublicationComplete}})
	require.NoError(t, err)
	require.Len(t, projection.Candidates, 1)
	require.Equal(t, &imageagent.ImageGenerationProof{IntentID: originalFact.IntentID, Fingerprint: originalFact.Fingerprint, SettlementProofDigest: originalFact.TerminalProofDigest(), Points: 12}, projection.Candidates[0].GenerationProof)
	replay, err := a.PersistImageSetSlotResult(context.Background(), PersistSlotResultV3ActivityInput{RunID: input.RunID, Identity: input.Identity, PlanRevision: 1, AttemptKey: input.IdempotencyKey, Result: SlotWorkflowV3Result{Published: published, Status: imageagent.SlotStatusAccepted, EffectPhase: imageagent.SlotEffectV3PublicationComplete}})
	require.NoError(t, err)
	require.Equal(t, projection, replay)
	expired, err := a.PersistImageSetSlotResult(context.Background(), PersistSlotResultV3ActivityInput{RunID: input.RunID, Identity: input.Identity, PlanRevision: 1, AttemptKey: input.IdempotencyKey, Result: SlotWorkflowV3Result{Published: imageagent.SlotEffectV3PublishedResult{SlotID: input.Slot.ID, Attempt: 1}, Status: imageagent.SlotStatusBlocked, ErrorCode: imageagent.BudgetElapsedCode, EffectPhase: imageagent.SlotEffectV3ProviderNotDispatched}})
	require.NoError(t, err)
	require.Equal(t, projection, expired, "expiration must not overwrite original settled materialized success")
	require.Equal(t, 1, recorder.GenerateCalls())
	current, err := repo.GetProjection(context.Background(), originalFact.Intent.Identity.RunScope)
	require.NoError(t, err)
	_, err = imageagent.ImageSetResultDigest(current.Plan, current.Slots, nil)
	require.NoError(t, err, "the retained set must bind the materialized output and original settlement")
}

func TestPersistImageSetSlotResultDistinguishesKnownUnstartedFromUnknown(t *testing.T) {
	for _, mode := range []string{"known_unstarted", "unknown", "expired_unknown"} {
		t.Run(mode, func(t *testing.T) {
			activities, repo, input := imageSetPersistenceFixture(t)
			code, phase := imageagent.BudgetElapsedCode, imageagent.SlotEffectV3ProviderNotDispatched
			if mode != "known_unstarted" {
				_, won, err := repo.(imageagent.SlotExternalEffectV3Repository).ReserveSlotProviderV3(context.Background(), slotEffectReservationV3(slotExecutionInputV3(input)))
				require.NoError(t, err)
				require.True(t, won)
				code, phase = imageagent.SlotProviderOutcomeUnknownCode, imageagent.SlotEffectV3ProviderUnknown
				if mode == "expired_unknown" {
					code, phase = imageagent.BudgetElapsedCode, imageagent.SlotEffectV3ProviderNotDispatched
				}
			}
			result, err := activities.PersistImageSetSlotResult(context.Background(), PersistSlotResultV3ActivityInput{RunID: input.RunID, Identity: input.Identity, PlanRevision: 1, AttemptKey: input.IdempotencyKey, Result: SlotWorkflowV3Result{Published: imageagent.SlotEffectV3PublishedResult{SlotID: input.Slot.ID, Attempt: 1}, Status: imageagent.SlotStatusBlocked, ErrorCode: code, EffectPhase: phase}})
			require.NoError(t, err)
			if mode == "known_unstarted" {
				require.Zero(t, result.Attempt)
				require.Equal(t, &imageagent.ImageSlotClosure{Kind: "not_dispatched"}, result.Closure)
			} else {
				require.Equal(t, 1, result.Attempt)
				require.Nil(t, result.Closure)
				require.Equal(t, imageagent.SlotProviderOutcomeUnknownCode, result.ErrorCode, "a later budget denial cannot replace the original UNKNOWN owner")
			}
		})
	}
}

func TestExpiredImageSetProjectionPreservesPersistedBlockedEffectPolicy(t *testing.T) {
	for _, policy := range []struct {
		phase imageagent.SlotEffectV3Phase
		code  string
	}{
		{imageagent.SlotEffectV3ProviderUnknown, imageagent.SlotProviderOutcomeUnknownCode},
		{imageagent.SlotEffectV3StagingUnknown, imageagent.SlotStagingOutcomeUnknownCode},
		{imageagent.SlotEffectV3PublicationUnknown, imageagent.SlotPublicationOutcomeUnknownCode},
		{imageagent.SlotEffectV3ReviewRequired, imageagent.SlotReviewRequiredCode},
		{imageagent.SlotEffectV3ReviewTransportRequired, imageagent.SlotReviewTransportRequiredCode},
		{imageagent.SlotEffectV3RecoveryBlocked, imageagent.SlotRecoveryBlockedCode},
	} {
		t.Run(string(policy.phase), func(t *testing.T) {
			a, repo, input := imageSetPersistenceFixture(t)
			ctx := context.Background()
			effects := repo.(imageagent.SlotExternalEffectV3Repository)
			execution := slotExecutionInputV3(input)
			reservation := slotEffectReservationV3(execution)
			_, won, err := effects.ReserveSlotProviderV3(ctx, reservation)
			require.NoError(t, err)
			require.True(t, won)
			transition := imageagent.SlotEffectV3BlockTransition{Reservation: reservation, Phase: policy.phase, Code: policy.code}
			if policy.phase == imageagent.SlotEffectV3StagingUnknown || policy.phase == imageagent.SlotEffectV3PublicationUnknown {
				manifest := v3StagingManifest(input, tinyPNGBytes(t))
				_, err = effects.PrepareSlotStagingV3(ctx, reservation, manifest)
				require.NoError(t, err)
				if policy.phase == imageagent.SlotEffectV3PublicationUnknown {
					fingerprint, err := imageagent.StagingManifestFingerprint(manifest)
					require.NoError(t, err)
					_, err = effects.CommitSlotStagedV3(ctx, reservation, fingerprint)
					require.NoError(t, err)
					final, err := expectedFinalManifestV3(execution, manifest)
					require.NoError(t, err)
					fingerprint, err = imageagent.FinalManifestFingerprint(final)
					require.NoError(t, err)
					_, claim, won, err := effects.ClaimSlotPublicationV3(ctx, imageagent.PublicationClaimRequest{Reservation: reservation, Owner: "original-owner", LeaseDuration: time.Minute, PublicationFingerprint: fingerprint, FinalManifest: final})
					require.NoError(t, err)
					require.True(t, won)
					transition.Owner, transition.Fence = claim.Owner, claim.Fence
				}
			}
			original, err := effects.BlockSlotEffectV3(ctx, transition)
			require.NoError(t, err)
			result, err := a.PersistImageSetSlotResult(ctx, PersistSlotResultV3ActivityInput{RunID: input.RunID, Identity: input.Identity, PlanRevision: 1, AttemptKey: input.IdempotencyKey, Result: SlotWorkflowV3Result{Published: imageagent.SlotEffectV3PublishedResult{SlotID: input.Slot.ID, Attempt: 1}, Status: imageagent.SlotStatusBlocked, ErrorCode: imageagent.BudgetElapsedCode, EffectPhase: imageagent.SlotEffectV3ProviderNotDispatched}})
			require.NoError(t, err)
			require.Equal(t, policy.code, result.ErrorCode)
			require.Equal(t, 1, result.Attempt)
			require.Nil(t, result.Closure)
			wantPolicy, err := imageagent.SlotEffectV3BlockedPolicyFor(original.Phase, original.BlockedCode)
			require.NoError(t, err)
			gotPolicy, ok := imageagent.SlotEffectV3BlockedPolicyForCode(result.ErrorCode)
			require.True(t, ok)
			require.Equal(t, wantPolicy, gotPolicy, "expiration must retain the original permitted actions")
			retained, err := effects.GetSlotExternalEffectV3(ctx, reservation.Identity)
			require.NoError(t, err)
			require.Equal(t, original, retained)
			require.Zero(t, a.stagedSlotExecutor.(*recordingStagedExecutor).GenerateCalls())
		})
	}
}

func TestSetSlotClosureBindsTheOriginalSettledEconomics(t *testing.T) {
	for _, mode := range []string{"invalid_locator", "invalid_bytes", "invalid_bytes_execute", "transient", "unsettled_invalid_bytes", "price_drift"} {
		t.Run(mode, func(t *testing.T) {
			drift := mode == "price_drift"
			a, repo, input := imageSetPersistenceFixture(t)
			execution := slotExecutionInputV3(input)
			reservation := slotEffectReservationV3(execution)
			_, won, err := repo.(imageagent.SlotExternalEffectV3Repository).ReserveSlotProviderV3(context.Background(), reservation)
			require.NoError(t, err)
			require.True(t, won)
			digest, err := imageagent.ImageSlotGenerationInputDigest(execution)
			require.NoError(t, err)
			quote := input.Slot.Recipe.Quote
			intent := imageagent.GenerationIntent{Identity: reservation.Identity, MemberID: input.Identity.MemberID, CatalogHash: input.AssetCatalog.Manifest.Hash, SourceDigest: imageagent.ImageSourceBundleDigest(input.Slot.Recipe.References), PromptVersion: input.Slot.Recipe.PromptVersion, InputProtocol: imageagent.ImageSetSchema, InputDigest: digest, RouteReference: quote.RouteReference, CredentialReference: quote.CredentialReference, ConfigurationVersion: quote.ConfigurationVersion, Provider: quote.Provider, Model: quote.Model, Protocol: quote.Protocol, Resolution: quote.Resolution, Quality: quote.Quality, PriceVersion: quote.PriceVersion, Points: quote.Points, LimitVersion: 1, MonthStart: time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)}
			if drift {
				intent.PriceVersion = "different-price"
			}
			facts := repo.(imageagent.GenerationFactRepository)
			fact, err := facts.PrepareGenerationIntent(context.Background(), intent)
			require.NoError(t, err)
			fact, err = facts.BindGenerationReservation(context.Background(), intent, imageagent.GenerationReservationReceipt{IntentID: fact.IntentID, Fingerprint: fact.Fingerprint, OrganizationID: input.Identity.TenantID, MemberID: input.Identity.MemberID, OperationID: "image-reserve:" + fact.IntentID, ReservationID: "reservation", ResourceType: "ai_point", Points: intent.Points, PriceVersion: intent.PriceVersion, LimitVersion: intent.LimitVersion, MonthStart: intent.MonthStart})
			require.NoError(t, err)
			_, won, err = facts.BeginGenerationDispatch(context.Background(), intent)
			require.NoError(t, err)
			require.True(t, won)
			if !drift {
				_, err = repo.(imageagent.SlotExternalEffectV3Repository).BlockSlotEffectV3(context.Background(), imageagent.SlotEffectV3BlockTransition{Reservation: reservation, Phase: imageagent.SlotEffectV3ProviderUnknown, Code: imageagent.SlotProviderOutcomeUnknownCode})
				require.NoError(t, err)
				_, err = a.PersistImageSetSlotResult(context.Background(), PersistSlotResultV3ActivityInput{RunID: input.RunID, Identity: input.Identity, PlanRevision: 1, AttemptKey: input.IdempotencyKey, Result: SlotWorkflowV3Result{Published: imageagent.SlotEffectV3PublishedResult{SlotID: input.Slot.ID, Attempt: 1}, Status: imageagent.SlotStatusBlocked, ErrorCode: imageagent.SlotProviderOutcomeUnknownCode, EffectPhase: imageagent.SlotEffectV3ProviderUnknown}})
				require.NoError(t, err)
				current, err := repo.GetProjection(context.Background(), reservation.Identity.RunScope)
				require.NoError(t, err)
				block := &imageagent.Block{Code: imageagent.SlotProviderOutcomeUnknownCode, Message: imageagent.SlotProviderOutcomeUnknownCode, SlotID: input.Slot.ID}
				err = a.PersistRunState(context.Background(), PersistRunStateActivityInput{RunID: input.RunID, Identity: input.Identity, PlanRevision: 1, CommitID: "blocked", CurrentNode: "retry_slot", Projection: WorkflowResult{Status: imageagent.RunStatusBlocked, Block: block, Plan: current.Plan, Slots: current.Slots, RecoverableEffects: []imageagent.RecoverableEffect{{SlotID: input.Slot.ID, Attempt: 1, Code: imageagent.SlotProviderOutcomeUnknownCode}}}})
				require.NoError(t, err)
			}
			proof := imageagent.GenerationSuccess{ResponseID: "response", ResultDigest: strings.Repeat("b", 64), ResultUnavailable: "invalid_result"}
			if mode == "invalid_bytes" || mode == "invalid_bytes_execute" || mode == "transient" || mode == "unsettled_invalid_bytes" {
				proof = proof.WithResultLocator("https://output.example/image.png", "")
			}
			fact, err = facts.RecordGenerationSuccess(context.Background(), intent, proof)
			require.NoError(t, err)
			if mode != "unsettled_invalid_bytes" {
				fact, err = facts.BindGenerationSettlement(context.Background(), intent, imageagent.GenerationSettlementReceipt{IntentID: fact.IntentID, Fingerprint: fact.Fingerprint, OperationID: "image-finalize:" + fact.IntentID, ReservationID: "reservation", State: "committed", Points: intent.Points, ProofDigest: fact.TerminalProofDigest()})
				require.NoError(t, err)
			}
			if drift {
				_, err := a.PersistImageSetSlotResult(context.Background(), PersistSlotResultV3ActivityInput{RunID: input.RunID, Identity: input.Identity, PlanRevision: 1, AttemptKey: input.IdempotencyKey, Result: SlotWorkflowV3Result{Published: imageagent.SlotEffectV3PublishedResult{SlotID: input.Slot.ID, Attempt: 1}, Status: imageagent.SlotStatusBlocked, ErrorCode: imageagent.SlotProviderOutcomeUnknownCode, EffectPhase: imageagent.SlotEffectV3ProviderUnknown}})
				require.ErrorIs(t, err, imageagent.ErrRevisionConflict)
				return
			}
			recovery := EffectRecoveryWorkflowInput{RunID: input.RunID, Identity: input.Identity, PlanRevision: 1, Slot: input.Slot, Attempt: 1, ImageSet: input.ImageSet, TargetPlatform: input.TargetPlatform, AssetCatalog: input.AssetCatalog}
			gets := 0
			if proof.ResultURL != "" {
				a.generationOutputRecovery = func(_ context.Context, original imageagent.SlotExecutionInput, persisted imageagent.GenerationFact) (imageagent.SlotGeneratedOutput, error) {
					gets++
					require.Equal(t, input.Slot.ID, original.Slot.ID)
					require.Equal(t, fact, persisted)
					if mode == "transient" {
						return imageagent.SlotGeneratedOutput{}, errors.New("download timed out")
					}
					return imageagent.SlotGeneratedOutput{}, imageagent.ErrInvalidGeneratedOutput
				}
				if mode == "invalid_bytes_execute" {
					_, err = a.ExecuteSlotV3(context.Background(), input)
					require.ErrorContains(t, err, imageagent.SlotProviderOutcomeUnknownCode)
				} else if mode == "transient" {
					_, err = a.RecoverEffectV3(context.Background(), recovery)
					require.Error(t, err)
				} else {
					_, err = a.RecoverEffectV3(context.Background(), recovery)
					require.NoError(t, err, "known invalid output must reach original durable reconciliation")
				}
			}
			result, err := a.ReconcileEffectRecoveryV3(context.Background(), recovery)
			require.NoError(t, err)
			if mode == "transient" || mode == "unsettled_invalid_bytes" {
				require.Nil(t, result.Closure)
				current, err := repo.GetProjection(context.Background(), reservation.Identity.RunScope)
				require.NoError(t, err)
				require.Len(t, current.RecoverableEffects, 1)
				return
			}
			require.Equal(t, &imageagent.ImageSlotClosure{Kind: "settled", IntentID: fact.IntentID, Fingerprint: fact.Fingerprint, SettlementProofDigest: fact.TerminalProofDigest(), Points: intent.Points}, result.Closure)
			require.Equal(t, imageagent.InvalidGeneratedOutputCode, result.BlockedCode)
			current, err := repo.GetProjection(context.Background(), reservation.Identity.RunScope)
			require.NoError(t, err)
			require.Equal(t, result.Closure, current.Slots[0].Closure)
			require.Empty(t, current.RecoverableEffects)
			replay, err := a.ReconcileEffectRecoveryV3(context.Background(), recovery)
			require.NoError(t, err, "lost acknowledgement must read the original reconciled result")
			require.Equal(t, result, replay)
			if proof.ResultURL != "" {
				closedGets := gets
				_, err = a.RecoverEffectV3(context.Background(), recovery)
				require.NoError(t, err)
				_, err = a.ReconcileEffectRecoveryV3(context.Background(), recovery)
				require.NoError(t, err)
				require.Equal(t, closedGets, gets, "durable invalid closure must not repeatedly download or generate")
			}
			storedFact, err := facts.ReadGenerationFact(context.Background(), reservation.Identity)
			require.NoError(t, err)
			require.Equal(t, fact, storedFact, "invalid output cannot rewrite the original economic success or settlement")
			require.Zero(t, a.stagedSlotExecutor.(*recordingStagedExecutor).GenerateCalls(), "known bad output never redispatches the provider")
			_, err = imageagent.ImageSetClosedEffectsDigest(current.Plan, current.Slots, current.RecoverableEffects)
			require.NoError(t, err, "known charged failure must permit review/subset preparation")
		})
	}
}
