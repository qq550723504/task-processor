package assetpublication

import (
	"context"
	"reflect"
	"task-processor/internal/imageagent"
	productasset "task-processor/internal/product/asset"
)

type ImageGenerationFactReader interface {
	ReadGenerationFact(context.Context, imageagent.SlotExternalEffectIdentity) (imageagent.GenerationFact, error)
}
type ImageMaterializationReader interface {
	GetSlotExternalEffectV3(context.Context, imageagent.SlotExternalEffectIdentity) (imageagent.SlotEffectV3Attempt, error)
}

type ImageSetCandidateReader struct {
	projections      ProjectionSource
	generations      ImageGenerationFactReader
	materializations ImageMaterializationReader
	publicURLs       imageagent.DurableAssetPublicURLResolver
	trialURLs        *imageagent.IsolatedTrialGeneratedURLPolicy
}

func NewImageSetCandidateReader(projections ProjectionSource, generations ImageGenerationFactReader, materializations ImageMaterializationReader, publicURLs imageagent.DurableAssetPublicURLResolver, trial ...*imageagent.IsolatedTrialGeneratedURLPolicy) (*ImageSetCandidateReader, error) {
	if nilValue(projections) || nilValue(generations) || nilValue(materializations) || nilValue(publicURLs) || len(trial) > 1 {
		return nil, imageagent.ErrValidation
	}
	reader := &ImageSetCandidateReader{projections: projections, generations: generations, materializations: materializations, publicURLs: publicURLs}
	if len(trial) == 1 {
		reader.trialURLs = trial[0]
	}
	return reader, nil
}

func (r *ImageSetCandidateReader) ReadImageSetCandidate(ctx context.Context, source productasset.SourceSelection, choice productasset.ImageSetChoice) (productasset.ImageSetCandidate, error) {
	bad := func() (productasset.ImageSetCandidate, error) {
		return productasset.ImageSetCandidate{}, imageagent.ErrRevisionConflict
	}
	if r == nil || ctx == nil || choice.Kind != "generated" || choice.PlanRevision <= 0 || choice.Attempt <= 0 || !canonical(choice.RunID) || !canonical(choice.SlotID) || !canonical(choice.AssetID) || !canonical(source.TenantID) || !canonical(source.ActorID) || !canonical(source.MemberID) {
		return bad()
	}
	projection, err := r.projections.GetProjection(ctx, imageagent.RunScope{TenantID: source.TenantID, OwnerUserID: source.ActorID, RunID: choice.RunID})
	if err != nil {
		return productasset.ImageSetCandidate{}, err
	}
	run := projection.Run
	if run.ID != choice.RunID || run.TenantID != source.TenantID || run.UserID != source.ActorID || run.MemberID != source.MemberID || run.TargetPlatform != source.TargetPlatform || run.BusinessTaskID != source.ItemID || run.Status != imageagent.RunStatusAwaitingFinalApproval || run.ActivePlanRevision != choice.PlanRevision || projection.Plan.Revision != choice.PlanRevision || imageagent.ValidateImageSetAdmission(run, projection.Plan) != nil {
		return bad()
	}
	bound := projection.Plan.Set.Source
	if string(bound.ContextKind) != source.ContextKind || bound.ProductID != source.ProductKey || bound.OperationID != source.ItemID || bound.OriginalPublicationID != source.OriginalPublicationID || bound.OriginalVersion != source.OriginalSnapshotVersion || bound.EffectiveVersion != source.EffectiveCatalogVersion || bound.ApplyReceiptID != source.ApplyReceiptID || bound.CatalogHash != projection.AssetCatalog.Manifest.Hash || projection.AssetCatalog.ProductContext.ProductID != source.ProductKey {
		return bad()
	}
	digest, err := imageagent.ImageSetResultDigest(projection.Plan, projection.Slots, projection.RecoverableEffects)
	if err != nil || digest != choice.ResultDigest || digest != projection.ResultDigest {
		return bad()
	}
	execution, err := imageagent.ImageSetSlotExecutionInput(projection, choice.SlotID, choice.Attempt)
	if err != nil {
		return bad()
	}
	identity := imageagent.SlotExternalEffectIdentity{RunScope: imageagent.ScopeForRun(run), PlanRevision: choice.PlanRevision, SlotID: choice.SlotID, Attempt: choice.Attempt}
	fact, err := r.generations.ReadGenerationFact(ctx, identity)
	if err != nil {
		return productasset.ImageSetCandidate{}, err
	}
	inputDigest, err := imageagent.ImageSlotGenerationInputDigest(execution)
	if err != nil || fact.Validate() != nil || fact.State != imageagent.GenerationSucceeded || fact.Success.ResultUnavailable != "" || fact.Intent.Identity != identity || fact.Intent.MemberID != source.MemberID || fact.Intent.CatalogHash != bound.CatalogHash || fact.Intent.InputProtocol != imageagent.ImageSetSchema || fact.Intent.InputDigest != inputDigest || fact.Intent.SourceDigest != imageagent.ImageSourceBundleDigest(execution.Slot.Recipe.References) || fact.Intent.PromptVersion != execution.Slot.Recipe.PromptVersion || !execution.Slot.Recipe.Quote.MatchesGenerationIntent(fact.Intent) {
		return bad()
	}
	closure, err := imageagent.ImageGenerationClosure(fact)
	if err != nil {
		return bad()
	}
	effect, err := r.materializations.GetSlotExternalEffectV3(ctx, identity)
	if err != nil {
		return productasset.ImageSetCandidate{}, err
	}
	if effect.Identity != identity || effect.Phase != imageagent.SlotEffectV3PublicationComplete || effect.InputFingerprint != imageagent.SlotExecutionFingerprint(execution) || effect.IdempotencyKey != execution.IdempotencyKey || imageagent.ValidateSlotEffectV3Completion(effect.Published, effect.FinalManifest, effect.ResultFingerprint) != nil || len(effect.Published.Candidates) != 1 || len(effect.FinalManifest.Assets) != 1 {
		return bad()
	}
	for _, slot := range projection.Slots {
		if slot.Slot.ID != choice.SlotID {
			continue
		}
		if slot.Slot.Status != imageagent.SlotStatusAccepted || slot.Attempt != choice.Attempt || len(slot.Candidates) != 1 || !reflect.DeepEqual(slot.Closure, closure) {
			return bad()
		}
		candidate := slot.Candidates[0]
		proof := &imageagent.ImageGenerationProof{IntentID: fact.IntentID, Fingerprint: fact.Fingerprint, SettlementProofDigest: fact.TerminalProofDigest(), Points: fact.Intent.Points}
		if candidate.AssetID != choice.AssetID || candidate.SourceAssetID != execution.Slot.SourceAssetIDs[0] || candidate.Width != 1024 || candidate.Height != 1024 || !reflect.DeepEqual(candidate.Operations, []string{"render_source_edit"}) || !reflect.DeepEqual(candidate.GenerationProof, proof) {
			return bad()
		}
		published, err := imageagent.NewSlotEffectV3PublishedResult(imageagent.SlotExecutionResult{SlotID: choice.SlotID, Attempt: choice.Attempt, Candidates: []imageagent.AssetCandidate{candidate}})
		if err != nil || !reflect.DeepEqual(published, effect.Published) || imageagent.ValidatePublishedAssetRefForSlot(execution, effect.FinalManifest.Assets[0], 0) != nil {
			return bad()
		}
		if effect.FinalManifest.Assets[0].ProviderReceiptID != fact.Success.ResponseID {
			return bad()
		}
		url, err := imageagent.ResolvePublishedAssetURL(execution, candidate.DurableAsset, 0, r.publicURLs, r.trialURLs)
		if err != nil {
			return productasset.ImageSetCandidate{}, err
		}
		role, err := approvedRole(execution.Slot.Role)
		if err != nil {
			return bad()
		}
		asset := productasset.ApprovedAsset{ID: candidate.AssetID, RunID: choice.RunID, PlanRevision: choice.PlanRevision, SlotID: choice.SlotID, Attempt: choice.Attempt, Role: role, URL: url, SourceAssetID: candidate.SourceAssetID, Width: candidate.Width, Height: candidate.Height, Operations: append([]string(nil), candidate.Operations...), GenerationEvidence: &productasset.GenerationEvidence{IntentID: fact.IntentID, Fingerprint: fact.Fingerprint, SettlementProofDigest: fact.TerminalProofDigest(), ArtifactHash: candidate.DurableAsset.SHA256}}
		return productasset.ImageSetCandidate{Asset: asset, Result: productasset.ImageSetResultBinding{RunID: choice.RunID, PlanRevision: choice.PlanRevision, ResultDigest: digest}}, nil
	}
	return bad()
}

var _ productasset.ImageCandidateSelectionReader = (*ImageSetCandidateReader)(nil)
