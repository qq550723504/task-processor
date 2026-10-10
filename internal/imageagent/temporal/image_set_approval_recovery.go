package temporal

import (
	"context"
	"errors"
	"reflect"

	"task-processor/internal/imageagent"
	productasset "task-processor/internal/product/asset"
)

// This is internal recovery of a durable action, not authority for a new
// approval. Its only external port is the existing immutable receipt reader.
func (a *Activities) readCommittedImageSetApproval(ctx context.Context, current imageagent.RunProjection) (productasset.ApprovalCommit, bool, error) {
	pending := current.PendingCommand
	if current.Plan.Set == nil || pending == nil || pending.Kind != "approve_results" {
		return productasset.ApprovalCommit{}, false, nil
	}
	if current.Run.Status != imageagent.RunStatusAwaitingFinalApproval && current.Run.Status != imageagent.RunStatusCompleted ||
		current.Run.ActivePlanRevision != current.Plan.Revision || pending.PlanRevision != current.Plan.Revision ||
		!imageSetPublicationPhase(pending.Phase) || imageagent.ValidateImageSetAdmission(current.Run, current.Plan) != nil {
		return productasset.ApprovalCommit{}, false, imageagent.ErrCommandBlocked
	}
	digest, err := imageagent.ImageSetResultDigest(current.Plan, current.Slots, current.RecoverableEffects)
	if err != nil || digest != current.ResultDigest || digest != pending.ResultDigest {
		return productasset.ApprovalCommit{}, false, imageagent.ErrRevisionConflict
	}
	commit, err := a.imageSetApprovals.ReadApprovalCommit(ctx, current.Run.TenantID, pending.ActionID)
	if errors.Is(err, productasset.ErrApprovedAssetsNotReady) {
		// A missing observation is not evidence that an in-flight transaction
		// cannot commit. The original action remains pending and live IAM still
		// governs any subsequent attempt to Select.
		return productasset.ApprovalCommit{}, false, nil
	}
	if err != nil {
		return productasset.ApprovalCommit{}, false, err
	}
	if err := ctx.Err(); err != nil {
		return productasset.ApprovalCommit{}, false, err
	}
	source, target := current.Plan.Set.Source, current.Plan.Set.Target
	expectedSource := productasset.SourceSelectionRequest{ContextKind: string(source.ContextKind), ItemID: source.OperationID, OriginalPublicationID: source.OriginalPublicationID, OriginalSnapshotVersion: source.OriginalVersion, EffectiveCatalogVersion: source.EffectiveVersion, ApplyReceiptID: source.ApplyReceiptID, TargetPlatform: target.Platform}
	var expectedTarget *productasset.ImageSetTarget
	if target.Platform != "product" {
		expectedTarget = &productasset.ImageSetTarget{RecordID: target.RecordID, StoreID: target.StoreID, Site: target.Site, ApplicationID: target.ApplicationID, ApplicationMode: target.ApplicationMode, CategoryID: target.CategoryID, ProductTypeID: target.ProductTypeID, AttributesDigest: target.AttributesDigest, VariantsDigest: target.VariantsDigest}
	}
	if productasset.ValidateApprovalCommit(commit) != nil || commit.ImageSet == nil || commit.TenantID != current.Run.TenantID ||
		commit.ActionID != pending.ActionID || commit.ProductKey != source.ProductID || commit.TargetPlatform != target.Platform ||
		commit.SourceSnapshotVersion != source.EffectiveVersion || commit.ImageSet.Source != expectedSource ||
		!reflect.DeepEqual(commit.ImageSet.Target, expectedTarget) || commit.ImageSet.Digest != pending.SelectionDigest {
		return productasset.ApprovalCommit{}, false, imageagent.ErrRevisionConflict
	}
	return commit, true, nil
}

func imageSetPublicationPhase(phase string) bool {
	return phase == imageagent.ImageSetApprovalPublicationStarted || phase == string(updatePhaseApprovalPersistComplete)
}

func (a *Activities) liveRunProjectionIdentity(ctx context.Context, runID string, identity imageagent.ExecutionIdentity) (context.Context, *imageagent.RunProjection, error) {
	ctx, err := a.restoreExecutionIdentity(ctx, runID, identity)
	return ctx, nil, err
}

func (a *Activities) restoreImageSetCompletionIdentity(ctx context.Context, input *PersistRunStateActivityInput) (context.Context, *imageagent.RunProjection, error) {
	if a.imageSetApprovals == nil || a.executionAuthorizer == nil || input.Projection.Status != imageagent.RunStatusCompleted || input.CurrentNode != "complete" {
		return a.liveRunProjectionIdentity(ctx, input.RunID, input.Identity)
	}
	current, err := a.loadOrganizationExecution(ctx, input.RunID, input.Identity)
	if err != nil {
		return nil, nil, err
	}
	if current.Plan.Set == nil {
		return a.liveRunProjectionIdentity(ctx, input.RunID, input.Identity)
	}
	if current.Run.ActivePlanRevision != input.PlanRevision || input.Projection.Block != nil || input.Projection.PendingCommand != nil ||
		!reflect.DeepEqual(current.Plan, input.Projection.Plan) || !reflect.DeepEqual(current.Slots, input.Projection.Slots) ||
		current.ResultDigest != input.Projection.ResultDigest || !reflect.DeepEqual(current.RecoverableEffects, input.Projection.RecoverableEffects) {
		return nil, nil, imageagent.ErrRevisionConflict
	}
	if current.Run.Status == imageagent.RunStatusCompleted && current.Run.CurrentNode == "complete" && current.Run.Block == nil && current.PendingCommand == nil {
		// The original projection CAS has already completed. Replaying the
		// exact saved facts performs no write and requires no fresh grant.
		input.Projection.CommandIngress = current.CommandIngress
		return withOrganizationActivityIdentity(ctx, current.Run, input.Identity.TraceID), &current, nil
	}
	_, committed, err := a.readCommittedImageSetApproval(ctx, current)
	if err != nil {
		return nil, nil, err
	}
	if !committed {
		return a.liveRunProjectionIdentity(ctx, input.RunID, input.Identity)
	}
	// Only completion of this same plan/result may clear its pending receipt.
	// Keep the already durable command-capacity facts, including on old ACKs.
	input.Projection.CommandIngress = current.CommandIngress
	return withOrganizationActivityIdentity(ctx, current.Run, input.Identity.TraceID), &current, nil
}

func (a *Activities) restoreImageSetPendingRecoveryIdentity(ctx context.Context, input PersistPendingCommandActivityInput) (context.Context, *imageagent.RunProjection, error) {
	if a.imageSetApprovals == nil || a.executionAuthorizer == nil || input.Receipt == nil || input.Receipt.Kind != "approve_results" || !imageSetPublicationPhase(input.Receipt.Phase) {
		return a.liveRunProjectionIdentity(ctx, input.RunID, input.Identity)
	}
	current, err := a.loadOrganizationExecution(ctx, input.RunID, input.Identity)
	if err != nil {
		return nil, nil, err
	}
	before, after := current.PendingCommand, input.Receipt
	if before != nil && before.Kind == "approve_results" && before.Phase == string(updatePhaseApprovalPublish) &&
		after.Phase == imageagent.ImageSetApprovalPublicationStarted && after.ActionID == before.ActionID &&
		after.PlanRevision == before.PlanRevision && after.SelectionDigest == before.SelectionDigest &&
		after.ResultDigest == before.ResultDigest && after.SlotID == before.SlotID && after.Attempt >= before.Attempt &&
		after.Status == "pending" && reflect.DeepEqual(current.CommandIngress, input.CommandIngress) {
		// This is the first durable publication marker. Asset has not been
		// called yet; live IAM, rather than receipt recovery, authorizes it.
		return a.liveRunProjectionIdentity(ctx, input.RunID, input.Identity)
	}
	_, committed, err := a.readCommittedImageSetApproval(ctx, current)
	if err != nil {
		return nil, nil, err
	}
	if !committed {
		return a.liveRunProjectionIdentity(ctx, input.RunID, input.Identity)
	}
	if after.ActionID != before.ActionID || after.PlanRevision != before.PlanRevision || after.SelectionDigest != before.SelectionDigest ||
		after.ResultDigest != before.ResultDigest || after.SlotID != before.SlotID || after.Attempt < before.Attempt ||
		after.Status != "pending" && after.Status != "failed" ||
		before.Phase == string(updatePhaseApprovalPersistComplete) && after.Phase != before.Phase ||
		!reflect.DeepEqual(current.CommandIngress, input.CommandIngress) {
		return nil, nil, imageagent.ErrRevisionConflict
	}
	return withOrganizationActivityIdentity(ctx, current.Run, input.Identity.TraceID), &current, nil
}
