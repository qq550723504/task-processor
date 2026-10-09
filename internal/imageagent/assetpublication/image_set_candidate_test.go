package assetpublication

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"task-processor/internal/agent"
	"task-processor/internal/agentconfig"
	"task-processor/internal/imageagent"
	productasset "task-processor/internal/product/asset"
)

type originalSetFactFixture struct {
	fact   imageagent.GenerationFact
	effect imageagent.SlotEffectV3Attempt
}

func (f *originalSetFactFixture) ReadGenerationFact(context.Context, imageagent.SlotExternalEffectIdentity) (imageagent.GenerationFact, error) {
	return f.fact, nil
}
func (f *originalSetFactFixture) GetSlotExternalEffectV3(context.Context, imageagent.SlotExternalEffectIdentity) (imageagent.SlotEffectV3Attempt, error) {
	return f.effect, nil
}

func fixtureDigest(value any) string {
	raw, _ := json.Marshal(value)
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:])
}

func imageSetCandidateFixture(t *testing.T) (imageagent.RunProjection, *originalSetFactFixture, productasset.SourceSelection, productasset.ImageSetChoice) {
	t.Helper()
	hash := strings.Repeat("a", 64)
	catalog, err := imageagent.NormalizeAssetCatalog(imageagent.AssetCatalog{ProductContext: imageagent.ProductContextRef{ProductID: "product", SourceSnapshotVersion: 1}, Assets: []imageagent.AuthorizedAsset{{ID: "source", Type: imageagent.AuthorizedAssetSource, URL: "https://source.example.org/original.png", Width: 1024, Height: 1024}}})
	require.NoError(t, err)
	run := imageagent.Run{ID: "392ed0a2-0f01-4c94-9aa4-eb50271fae9c", TenantID: "org", UserID: "actor", MemberID: "member", BusinessTaskID: "operation", ScopeProtocol: imageagent.OrganizationScopeProtocol, TargetPlatform: "product", Mode: imageagent.RunModeManual, ActivePlanRevision: 1, Status: imageagent.RunStatusAwaitingFinalApproval, StartedAt: time.Now().UTC().Truncate(time.Microsecond)}
	recipe := &imageagent.ImageSlotRecipe{Purpose: "product_overview", Background: "white", Language: "zh", Placement: imageagent.ImagePlacement{Group: "detail", Order: 1}, PromptVersion: imageagent.ImageSetSchema, Prompt: "Show the exact product.", References: []imageagent.ImageSourceObservation{{AssetID: "source", SHA256: hash, MediaType: "image/png", Bytes: 100, Width: 1024, Height: 1024}}, Quote: imageagent.ImageGenerationQuote{Provider: "grsai", Model: "gpt-image-2.5", Protocol: "grsai-json-sync-v1", Resolution: "1024x1024", Quality: "auto", PriceVersion: "price", Points: 12, RouteReference: "route", CredentialReference: "credential", ConfigurationVersion: "config"}}
	slot := imageagent.Slot{ID: "overview", Role: imageagent.SlotRoleDetail, SourceAssetIDs: []string{"source"}, IdempotencyKey: "overview", Status: imageagent.SlotStatusPending, Recipe: recipe}
	failed := slot
	failed.ID, failed.IdempotencyKey = "closeup", "closeup"
	failed.Recipe = imageagent.CloneImageSlotRecipe(recipe)
	failed.Recipe.Purpose = "detail_closeup"
	failed.Recipe.Placement.Order = 2
	plan := imageagent.Plan{Revision: 1, IdempotencyKey: "plan", CreatedBy: run.UserID, SourceAssetIDs: []string{"source"}, Slots: []imageagent.Slot{slot, failed}, Set: &imageagent.ImageSetPlan{Schema: imageagent.ImageSetSchema, Source: imageagent.ImageSourceBinding{ProductID: "product", OperationID: "operation", OriginalPublicationID: "publication", OriginalVersion: 1, EffectiveVersion: 1, CatalogHash: catalog.Manifest.Hash}, Target: imageagent.ImageTarget{Platform: "product"}, Configuration: agent.ConfigurationSnapshotRef{Kind: agentconfig.SnapshotKind, ID: "91d39d8d-3819-4a7c-ae7f-04ce91951f9a", Digest: hash}, ConfigurationEpoch: "1", ParametersDigest: hash, InputDigest: hash, MaxPoints: 24}}
	plan.Set.QuoteDigest, err = imageagent.ImageSetQuoteDigest(plan)
	require.NoError(t, err)
	planDigest, err := imageagent.ImageSetPlanDigest(plan)
	require.NoError(t, err)
	limits := agentconfig.ImageRunLimits{Images: 2, Points: 24, ElapsedSeconds: 3600}
	run.Budget = imageagent.ImageSetBudget(limits)
	receipt := agentconfig.ImageRunAdmissionReceipt{ID: "74e01be8-f4e6-461f-8acb-18fe37329f1c", AdmittedAt: run.StartedAt, Deadline: run.StartedAt.Add(time.Hour), Command: agentconfig.ImageRunAdmissionCommand{Scope: agent.Scope{OrganizationID: run.TenantID, ActorID: run.UserID}, Snapshot: plan.Set.Configuration, MemberID: run.MemberID, RunID: run.ID, ConfirmActionID: "1b912e40-d50a-48f5-a12b-62c73a42e4b8", SourceDigest: imageagent.ImageSetSourceDigest(plan.Set.Source, plan), InputDigest: plan.Set.InputDigest, PlanDigest: planDigest, QuoteDigest: plan.Set.QuoteDigest, Limits: limits}}
	receipt.Digest = fixtureDigest(receipt)
	run.ImageAdmission = &receipt
	projection := imageagent.RunProjection{Run: run, Plan: plan, AssetCatalog: catalog}
	execution, err := imageagent.ImageSetSlotExecutionInput(projection, slot.ID, 1)
	require.NoError(t, err)
	inputDigest, err := imageagent.ImageSlotGenerationInputDigest(execution)
	require.NoError(t, err)
	identity := imageagent.SlotExternalEffectIdentity{RunScope: imageagent.ScopeForRun(run), PlanRevision: 1, SlotID: slot.ID, Attempt: 1}
	intent := imageagent.GenerationIntent{Identity: identity, MemberID: run.MemberID, CatalogHash: catalog.Manifest.Hash, SourceDigest: imageagent.ImageSourceBundleDigest(recipe.References), PromptVersion: imageagent.ImageSetSchema, InputProtocol: imageagent.ImageSetSchema, InputDigest: inputDigest, Provider: "grsai", Model: "gpt-image-2.5", Protocol: "grsai-json-sync-v1", Resolution: "1024x1024", Quality: "auto", PriceVersion: "price", Points: 12, RouteReference: "route", CredentialReference: "credential", ConfigurationVersion: "config", LimitVersion: 1, MonthStart: time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)}
	fact, err := imageagent.NewGenerationFact(intent)
	require.NoError(t, err)
	fact, err = fact.BindReservation(imageagent.GenerationReservationReceipt{IntentID: fact.IntentID, Fingerprint: fact.Fingerprint, OrganizationID: run.TenantID, MemberID: run.MemberID, OperationID: "image-reserve:" + fact.IntentID, ReservationID: "reservation", ResourceType: "ai_point", Points: 12, PriceVersion: "price", LimitVersion: 1, MonthStart: intent.MonthStart})
	require.NoError(t, err)
	fact, _, err = fact.BeginDispatch()
	require.NoError(t, err)
	fact, err = fact.RecordSuccess(imageagent.GenerationSuccess{ResponseID: "response", ResultDigest: hash, ResultURL: "https://output.example.org/result.png"})
	require.NoError(t, err)
	fact, err = fact.BindSettlement(imageagent.GenerationSettlementReceipt{IntentID: fact.IntentID, Fingerprint: fact.Fingerprint, OperationID: "image-finalize:" + fact.IntentID, ReservationID: "reservation", State: "committed", Points: 12, ProofDigest: fact.TerminalProofDigest()})
	require.NoError(t, err)
	owner, err := imageagent.ArtifactOwnerKey(run.UserID)
	require.NoError(t, err)
	candidate := imageagent.AssetCandidate{AssetID: "generated", SourceAssetID: "source", Width: 1024, Height: 1024, Operations: []string{"render_source_edit"}, DurableAsset: imageagent.DurableAssetIdentity{ObjectKey: fmt.Sprintf("image-agent/public/%s/%s/%s/1/overview/1/0-%s.png", run.TenantID, owner, run.ID, hash), SHA256: hash}, GenerationProof: &imageagent.ImageGenerationProof{IntentID: fact.IntentID, Fingerprint: fact.Fingerprint, SettlementProofDigest: fact.TerminalProofDigest(), Points: 12}}
	accepted := slot
	accepted.Status = imageagent.SlotStatusAccepted
	failed.Status = imageagent.SlotStatusBlocked
	closure, err := imageagent.ImageGenerationClosure(fact)
	require.NoError(t, err)
	projection.Slots = []imageagent.SlotProjection{{Slot: accepted, Attempt: 1, Closure: closure, Candidates: []imageagent.AssetCandidate{candidate}}, {Slot: failed, Closure: &imageagent.ImageSlotClosure{Kind: "not_dispatched"}, ErrorCode: imageagent.BudgetElapsedCode}}
	projection.ResultDigest, err = imageagent.ImageSetResultDigest(plan, projection.Slots, nil)
	require.NoError(t, err)
	published, err := imageagent.NewSlotEffectV3PublishedResult(imageagent.SlotExecutionResult{SlotID: slot.ID, Attempt: 1, Candidates: []imageagent.AssetCandidate{candidate}})
	require.NoError(t, err)
	resultFingerprint, err := imageagent.SlotEffectV3PublishedResultFingerprint(published)
	require.NoError(t, err)
	effect := imageagent.SlotEffectV3Attempt{Identity: identity, IdempotencyKey: execution.IdempotencyKey, InputFingerprint: imageagent.SlotExecutionFingerprint(execution), Phase: imageagent.SlotEffectV3PublicationComplete, Published: published, ResultFingerprint: resultFingerprint, FinalManifest: imageagent.FinalManifest{Assets: []imageagent.PublishedAssetRef{{ObjectKey: candidate.DurableAsset.ObjectKey, SHA256: hash, SizeBytes: 100, ContentType: "image/png", Width: 1024, Height: 1024, SourceAssetID: "source", Operations: []string{"render_source_edit"}, ProviderReceiptID: "response"}}}}
	source := productasset.SourceSelection{TenantID: run.TenantID, ActorID: run.UserID, MemberID: run.MemberID, ItemID: "operation", ProductKey: "product", OriginalPublicationID: "publication", OriginalSnapshotVersion: 1, EffectiveCatalogVersion: 1, TargetPlatform: "product"}
	choice := productasset.ImageSetChoice{Kind: "generated", RunID: run.ID, AssetID: candidate.AssetID, SlotID: slot.ID, PlanRevision: 1, Attempt: 1, ResultDigest: projection.ResultDigest, Presentation: productasset.ImagePresentation{Group: "detail", Order: 1}}
	return projection, &originalSetFactFixture{fact: fact, effect: effect}, source, choice
}

func TestImageSetCandidateReaderBindsOriginalFactsInKnownPartialRun(t *testing.T) {
	projection, facts, source, choice := imageSetCandidateFixture(t)
	reader, err := NewImageSetCandidateReader(staticProjectionSource{projection: projection}, facts, facts, staticPublicURLResolver{})
	require.NoError(t, err)
	result, err := reader.ReadImageSetCandidate(context.Background(), source, choice)
	require.NoError(t, err)
	require.Equal(t, choice.AssetID, result.Asset.ID)
	require.Equal(t, facts.fact.IntentID, result.Asset.GenerationEvidence.IntentID)
	require.Equal(t, facts.fact.Settlement.ProofDigest, result.Asset.GenerationEvidence.SettlementProofDigest)
	require.Equal(t, projection.ResultDigest, result.Result.ResultDigest)
}

func TestImageSetCandidateReaderRejectsDriftAndUnsettledOriginalFacts(t *testing.T) {
	for _, kind := range []string{"member", "publication", "source_version", "unknown", "unsettled", "wrong_quote", "wrong_artifact", "wrong_native_size"} {
		t.Run(kind, func(t *testing.T) {
			projection, facts, source, choice := imageSetCandidateFixture(t)
			switch kind {
			case "member":
				source.MemberID = "other"
			case "publication":
				source.OriginalPublicationID = "other"
			case "source_version":
				source.EffectiveCatalogVersion = 2
			case "unknown":
				projection.RecoverableEffects = []imageagent.RecoverableEffect{{SlotID: "closeup", Attempt: 1, Code: imageagent.SlotProviderOutcomeUnknownCode}}
			case "unsettled":
				facts.fact.Settlement = imageagent.GenerationSettlementReceipt{}
			case "wrong_quote":
				facts.fact.Intent.Points++
			case "wrong_artifact":
				facts.effect.FinalManifest.Assets[0].SHA256 = strings.Repeat("b", 64)
			case "wrong_native_size":
				facts.effect.FinalManifest.Assets[0].Width = 1200
			}
			reader, err := NewImageSetCandidateReader(staticProjectionSource{projection: projection}, facts, facts, staticPublicURLResolver{})
			require.NoError(t, err)
			_, err = reader.ReadImageSetCandidate(context.Background(), source, choice)
			require.Error(t, err)
		})
	}
}
