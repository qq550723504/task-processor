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
	reads  int
}

func (f *originalSetFactFixture) ReadGenerationFact(context.Context, imageagent.SlotExternalEffectIdentity) (imageagent.GenerationFact, error) {
	f.reads++
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

func candidateApprovalBinding(p imageagent.RunProjection) productasset.ImageSetResultBinding {
	return productasset.ImageSetResultBinding{RunID: p.Run.ID, PlanRevision: p.Plan.Revision, ResultDigest: p.ResultDigest}
}

func rebindLineageAdmission(t *testing.T, p *imageagent.RunProjection) {
	t.Helper()
	receipt := *p.Run.ImageAdmission
	receipt.Command.RunID = p.Run.ID
	receipt.Command.SourceDigest = imageagent.ImageSetSourceDigest(p.Plan.Set.Source, p.Plan)
	var err error
	receipt.Command.PlanDigest, err = imageagent.ImageSetPlanDigest(p.Plan)
	require.NoError(t, err)
	receipt.Digest = ""
	receipt.Digest = fixtureDigest(receipt)
	p.Run.ImageAdmission = &receipt
	require.NoError(t, imageagent.ValidateImageSetAdmission(p.Run, p.Plan))
}

func imageSetApprovalFixture(t *testing.T, parent imageagent.RunProjection) (imageagent.RunProjection, lineageProjectionSource) {
	t.Helper()
	raw, err := json.Marshal(parent)
	require.NoError(t, err)
	var current imageagent.RunProjection
	require.NoError(t, json.Unmarshal(raw, &current))
	current.Run.ID = "3b9ef503-8c6c-4a76-8eb8-047213c565cf"
	if parent.Run.ID == current.Run.ID {
		current.Run.ID = "74e01be8-f4e6-461f-8acb-18fe37329f1c"
	}
	current.Run.Status = imageagent.RunStatusAwaitingFinalApproval
	closed, err := imageagent.ImageSetClosedEffectsDigest(parent.Plan, parent.Slots, parent.RecoverableEffects)
	require.NoError(t, err)
	current.Plan.Set.Regeneration = &imageagent.ImageSetRegeneration{RunID: parent.Run.ID, ClosedEffectsDigest: closed, ResultDigest: parent.ResultDigest}
	current.ResultDigest, err = imageagent.ImageSetResultDigest(current.Plan, current.Slots, nil)
	require.NoError(t, err)
	rebindLineageAdmission(t, &current)
	return current, lineageProjectionSource{parent.Run.ID: parent, current.Run.ID: current}
}

type lineageProjectionSource map[string]imageagent.RunProjection

func (s lineageProjectionSource) GetProjection(_ context.Context, scope imageagent.RunScope) (imageagent.RunProjection, error) {
	p, ok := s[scope.RunID]
	if !ok || imageagent.ScopeForRun(p.Run) != scope {
		return imageagent.RunProjection{}, imageagent.ErrRunNotFound
	}
	return p, nil
}

func TestImageSetCandidateReaderRejectsUnrelatedRunBeforeOriginalFactRead(t *testing.T) {
	parent, facts, source, choice := imageSetCandidateFixture(t)
	current, _, _, _ := imageSetCandidateFixture(t)
	current.Run.ID = "3b9ef503-8c6c-4a76-8eb8-047213c565cf"
	rebindLineageAdmission(t, &current)
	reader, err := NewImageSetCandidateReader(lineageProjectionSource{parent.Run.ID: parent, current.Run.ID: current}, facts, facts, staticPublicURLResolver{})
	require.NoError(t, err)
	_, err = reader.ReadImageSetCandidate(context.Background(), source, candidateApprovalBinding(current), choice)
	require.ErrorIs(t, err, imageagent.ErrRevisionConflict, "same actor/source/platform does not make an unrelated run an ancestor")
	require.Zero(t, facts.reads)
}

func TestImageSetCandidateReaderTraversesOnlyRecordedRegenerationAncestors(t *testing.T) {
	parent, facts, source, choice := imageSetCandidateFixture(t)
	parent.Run.Status = imageagent.RunStatusCompleted
	middle, _ := imageSetApprovalFixture(t, parent)
	current, lineage := imageSetApprovalFixture(t, middle)
	lineage[parent.Run.ID] = parent
	reader, err := NewImageSetCandidateReader(lineage, facts, facts, staticPublicURLResolver{})
	require.NoError(t, err)
	selected, err := reader.ReadImageSetCandidate(context.Background(), source, candidateApprovalBinding(current), choice)
	require.NoError(t, err)
	require.Equal(t, choice.RunID, selected.Asset.RunID)
	require.Equal(t, facts.fact.IntentID, selected.Asset.GenerationEvidence.IntentID)
	require.Equal(t, facts.fact.TerminalProofDigest(), selected.Asset.GenerationEvidence.SettlementProofDigest)

	middle.Plan.Set.Regeneration.ClosedEffectsDigest = strings.Repeat("c", 64)
	rebindLineageAdmission(t, &middle)
	lineage[middle.Run.ID] = middle
	before := facts.reads
	_, err = reader.ReadImageSetCandidate(context.Background(), source, candidateApprovalBinding(current), choice)
	require.ErrorIs(t, err, imageagent.ErrRevisionConflict)
	require.Equal(t, before, facts.reads)
}

func TestImageSetCandidateLineageBindsAllStoreCategoryAndSourceDimensions(t *testing.T) {
	parent, facts, source, choice := imageSetCandidateFixture(t)
	parent.Plan.Set.Target = imageagent.ImageTarget{Platform: "shein", StoreID: "store", Site: "shein-us", RecordID: "record", ApplicationID: "application", ApplicationMode: "self_operated", CategoryID: 1, ProductTypeID: 2, AttributesDigest: strings.Repeat("a", 64), VariantsDigest: strings.Repeat("b", 64), RequirementDigest: strings.Repeat("c", 64), RequirementVersion: "v1"}
	parent.Run.TargetPlatform, source.TargetPlatform = "shein", "shein"
	for i := range parent.Plan.Slots {
		parent.Plan.Slots[i].Recipe.OfficialPlacement = &imageagent.OfficialImagePlacement{Group: "skc", Type: 1, Sort: i + 1, Site: "shein-us"}
	}
	rebindLineageAdmission(t, &parent)
	var err error
	parent.ResultDigest, err = imageagent.ImageSetResultDigest(parent.Plan, parent.Slots, nil)
	require.NoError(t, err)
	current, lineage := imageSetApprovalFixture(t, parent)
	// Refreshing official rules is already permitted by the preparation owner.
	current.Plan.Set.Target.RequirementVersion = "v2"
	current.Plan.Set.Target.RequirementDigest = strings.Repeat("d", 64)
	rebindLineageAdmission(t, &current)
	current.ResultDigest, err = imageagent.ImageSetResultDigest(current.Plan, current.Slots, nil)
	require.NoError(t, err)
	lineage[current.Run.ID] = current
	reader, err := NewImageSetCandidateReader(lineage, facts, facts, staticPublicURLResolver{})
	require.NoError(t, err)
	_, err = reader.readApprovingLineage(context.Background(), source, candidateApprovalBinding(current), choice)
	require.NoError(t, err)
	for _, dimension := range []string{"store", "category", "site", "attributes", "variants", "record", "application"} {
		t.Run(dimension, func(t *testing.T) {
			changed := current
			set := *current.Plan.Set
			changed.Plan.Set = &set
			switch dimension {
			case "store":
				changed.Plan.Set.Target.StoreID = "other-store"
			case "category":
				changed.Plan.Set.Target.CategoryID++
			case "site":
				changed.Plan.Set.Target.Site = "shein-ca"
			case "attributes":
				changed.Plan.Set.Target.AttributesDigest = strings.Repeat("e", 64)
			case "variants":
				changed.Plan.Set.Target.VariantsDigest = strings.Repeat("e", 64)
			case "record":
				changed.Plan.Set.Target.RecordID = "other-record"
			case "application":
				changed.Plan.Set.Target.ApplicationID = "other-application"
			}
			rebindLineageAdmission(t, &changed)
			changed.ResultDigest, err = imageagent.ImageSetResultDigest(changed.Plan, changed.Slots, nil)
			require.NoError(t, err)
			lineage[changed.Run.ID] = changed
			_, err = reader.readApprovingLineage(context.Background(), source, candidateApprovalBinding(changed), choice)
			require.ErrorIs(t, err, imageagent.ErrRevisionConflict)
		})
	}
	require.Zero(t, facts.reads)
}

func imageSetCandidateFixture(t *testing.T) (imageagent.RunProjection, *originalSetFactFixture, productasset.SourceSelection, productasset.ImageSetChoice) {
	t.Helper()
	return imageSetCandidateFixtureWithSource(t, nil)
}
func imageSetCandidateFixtureWithSource(t *testing.T, data []byte) (imageagent.RunProjection, *originalSetFactFixture, productasset.SourceSelection, productasset.ImageSetChoice) {
	t.Helper()
	hash := strings.Repeat("a", 64)
	sourceHash, sourceBytes := hash, int64(100)
	if len(data) > 0 {
		sum := sha256.Sum256(data)
		sourceHash = hex.EncodeToString(sum[:])
		sourceBytes = int64(len(data))
	}
	catalog, err := imageagent.NormalizeAssetCatalog(imageagent.AssetCatalog{ProductContext: imageagent.ProductContextRef{ProductID: "product", Title: "Exact product", SourceSnapshotVersion: 1}, Assets: []imageagent.AuthorizedAsset{{ID: "source", Type: imageagent.AuthorizedAssetSource, URL: "https://source.example.org/original.png", Width: 1024, Height: 1024}}})
	require.NoError(t, err)
	run := imageagent.Run{ID: "392ed0a2-0f01-4c94-9aa4-eb50271fae9c", TenantID: "org", UserID: "actor", MemberID: "member", BusinessTaskID: "operation", ScopeProtocol: imageagent.OrganizationScopeProtocol, TargetPlatform: "product", Mode: imageagent.RunModeManual, ActivePlanRevision: 1, Status: imageagent.RunStatusAwaitingFinalApproval, StartedAt: time.Now().UTC().Truncate(time.Microsecond)}
	recipe := &imageagent.ImageSlotRecipe{Purpose: "product_overview", Background: "white", Language: "zh", Placement: imageagent.ImagePlacement{Group: "detail", Order: 1}, PromptVersion: imageagent.ImageSetSchema, Prompt: "Show the exact product.", References: []imageagent.ImageSourceObservation{{AssetID: "source", SHA256: sourceHash, MediaType: "image/png", Bytes: sourceBytes, Width: 1024, Height: 1024}}, Quote: imageagent.ImageGenerationQuote{Provider: "grsai", Model: "gpt-image-2.5", Protocol: "grsai-json-sync-v1", Resolution: "1024x1024", Quality: "auto", PriceVersion: "price", Points: 12, RouteReference: "route", CredentialReference: "credential", ConfigurationVersion: "config"}}
	slot := imageagent.Slot{ID: "overview", Role: imageagent.SlotRoleDetail, SourceAssetIDs: []string{"source"}, IdempotencyKey: "overview", Status: imageagent.SlotStatusPending, Recipe: recipe}
	failed := slot
	failed.ID, failed.IdempotencyKey = "closeup", "closeup"
	failed.Recipe = imageagent.CloneImageSlotRecipe(recipe)
	failed.Recipe.Purpose = "detail_closeup"
	failed.Recipe.Placement.Order = 2
	plan := imageagent.Plan{Revision: 1, IdempotencyKey: "plan", CreatedBy: run.UserID, SourceAssetIDs: []string{"source"}, Slots: []imageagent.Slot{slot, failed}, Set: &imageagent.ImageSetPlan{Schema: imageagent.ImageSetSchema, Source: imageagent.ImageSourceBinding{ContextKind: imageagent.ImageSourceAcquisition, ProductID: "product", OperationID: "operation", OriginalPublicationID: "publication", OriginalVersion: 1, EffectiveVersion: 1, CatalogHash: catalog.Manifest.Hash}, Target: imageagent.ImageTarget{Platform: "product"}, Configuration: agent.ConfigurationSnapshotRef{Kind: agentconfig.SnapshotKind, ID: "91d39d8d-3819-4a7c-ae7f-04ce91951f9a", Digest: hash}, ConfigurationEpoch: "1", ParametersDigest: hash, InputDigest: hash, MaxPoints: 24}}
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
	fact, err = fact.RecordSuccess(imageagent.GenerationSuccess{ResponseID: "response", RequestID: "request", ResultDigest: hash, ResultURL: "https://output.example.org/result.png"})
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
	effect := imageagent.SlotEffectV3Attempt{Identity: identity, IdempotencyKey: execution.IdempotencyKey, InputFingerprint: imageagent.SlotExecutionFingerprint(execution), Phase: imageagent.SlotEffectV3PublicationComplete, Published: published, ResultFingerprint: resultFingerprint, FinalManifest: imageagent.FinalManifest{Assets: []imageagent.PublishedAssetRef{{ObjectKey: candidate.DurableAsset.ObjectKey, SHA256: hash, SizeBytes: 100, ContentType: "image/png", Width: 1024, Height: 1024, SourceAssetID: "source", Operations: []string{"render_source_edit"}, ProviderReceiptID: "request"}}}}
	source := productasset.SourceSelection{ContextKind: "acquisition", TenantID: run.TenantID, ActorID: run.UserID, MemberID: run.MemberID, ItemID: "operation", ProductKey: "product", OriginalPublicationID: "publication", OriginalSnapshotVersion: 1, EffectiveCatalogVersion: 1, TargetPlatform: "product"}
	choice := productasset.ImageSetChoice{Kind: "generated", RunID: run.ID, AssetID: candidate.AssetID, SlotID: slot.ID, PlanRevision: 1, Attempt: 1, ResultDigest: projection.ResultDigest, Presentation: productasset.ImagePresentation{Group: "detail", Order: 1}}
	return projection, &originalSetFactFixture{fact: fact, effect: effect}, source, choice
}

func TestImageSetCandidateReaderBindsOriginalFactsInKnownPartialRun(t *testing.T) {
	projection, facts, source, choice := imageSetCandidateFixture(t)
	reader, err := NewImageSetCandidateReader(staticProjectionSource{projection: projection}, facts, facts, staticPublicURLResolver{})
	require.NoError(t, err)
	result, err := reader.ReadImageSetCandidate(context.Background(), source, candidateApprovalBinding(projection), choice)
	require.NoError(t, err)
	require.Equal(t, choice.AssetID, result.Asset.ID)
	require.Equal(t, facts.fact.IntentID, result.Asset.GenerationEvidence.IntentID)
	require.Equal(t, facts.fact.Settlement.ProofDigest, result.Asset.GenerationEvidence.SettlementProofDigest)
	require.Equal(t, projection.ResultDigest, result.Result.ResultDigest)
}

func TestImageSetCandidateReaderReusesSettledCandidatesFromClosedParentRuns(t *testing.T) {
	for _, status := range []imageagent.RunStatus{imageagent.RunStatusCompleted, imageagent.RunStatusFailed, imageagent.RunStatusBlocked, imageagent.RunStatusCancelled} {
		for _, storedDigest := range []bool{true, false} {
			t.Run(fmt.Sprintf("%s/stored_digest=%t", status, storedDigest), func(t *testing.T) {
				projection, facts, source, choice := imageSetCandidateFixture(t)
				projection.Run.Status = status
				if !storedDigest {
					projection.ResultDigest = "" // Cancellation can close before final approval.
				}
				current, lineage := imageSetApprovalFixture(t, projection)
				reader, err := NewImageSetCandidateReader(lineage, facts, facts, staticPublicURLResolver{})
				require.NoError(t, err)
				result, err := reader.ReadImageSetCandidate(context.Background(), source, candidateApprovalBinding(current), choice)
				require.NoError(t, err)
				require.Equal(t, choice.RunID, result.Asset.RunID)
				require.Equal(t, choice.AssetID, result.Asset.ID)
				require.Equal(t, choice.ResultDigest, result.Result.ResultDigest)
				require.Equal(t, facts.fact.Settlement.ProofDigest, result.Asset.GenerationEvidence.SettlementProofDigest)
			})
		}
	}
}

func TestImageSetCandidateReaderRejectsParentsStillExecuting(t *testing.T) {
	for _, status := range []imageagent.RunStatus{imageagent.RunStatusAwaitingPlanApproval, imageagent.RunStatusPlanning, imageagent.RunStatusExecuting, imageagent.RunStatusEvaluating, imageagent.RunStatusRepairing} {
		t.Run(string(status), func(t *testing.T) {
			projection, facts, source, choice := imageSetCandidateFixture(t)
			projection.Run.Status = status
			current, lineage := imageSetApprovalFixture(t, projection)
			reader, err := NewImageSetCandidateReader(lineage, facts, facts, staticPublicURLResolver{})
			require.NoError(t, err)
			_, err = reader.ReadImageSetCandidate(context.Background(), source, candidateApprovalBinding(current), choice)
			require.ErrorIs(t, err, imageagent.ErrRevisionConflict)
		})
	}
}

func TestImageSetCandidateReaderRejectsDriftAndUnsettledOriginalFacts(t *testing.T) {
	for _, status := range []imageagent.RunStatus{imageagent.RunStatusAwaitingFinalApproval, imageagent.RunStatusCompleted, imageagent.RunStatusFailed, imageagent.RunStatusBlocked, imageagent.RunStatusCancelled} {
		t.Run(string(status), func(t *testing.T) {
			for _, kind := range []string{"member", "publication", "source_version", "unknown", "unsettled", "wrong_quote", "wrong_artifact", "wrong_native_size", "stored_digest", "choice_digest", "materialization_pending", "missing_admission", "active_revision"} {
				t.Run(kind, func(t *testing.T) {
					projection, facts, source, choice := imageSetCandidateFixture(t)
					projection.Run.Status = status
					current, lineage := imageSetApprovalFixture(t, projection)
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
					case "stored_digest":
						projection.ResultDigest = strings.Repeat("b", 64)
					case "choice_digest":
						choice.ResultDigest = strings.Repeat("b", 64)
					case "materialization_pending":
						facts.effect.Phase = imageagent.SlotEffectV3Phase("staged")
					case "missing_admission":
						projection.Run.ImageAdmission = nil
					case "active_revision":
						projection.Run.ActivePlanRevision++
					}
					lineage[projection.Run.ID] = projection
					reader, err := NewImageSetCandidateReader(lineage, facts, facts, staticPublicURLResolver{})
					require.NoError(t, err)
					_, err = reader.ReadImageSetCandidate(context.Background(), source, candidateApprovalBinding(current), choice)
					require.Error(t, err)
				})
			}
		})
	}
}
