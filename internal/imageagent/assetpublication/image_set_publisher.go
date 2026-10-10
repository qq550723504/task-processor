package assetpublication

import (
	"context"
	"fmt"
	"slices"

	"task-processor/internal/imageagent"
	productasset "task-processor/internal/product/asset"
)

type ImageSetSelector interface {
	Select(context.Context, productasset.ImageSetCommand) (productasset.ApprovalReceipt, error)
}
type ImageSetPublisher struct {
	projections ProjectionSource
	selections  ImageSetSelector
}

func NewImageSetPublisher(projections ProjectionSource, selections ImageSetSelector) (*ImageSetPublisher, error) {
	if nilValue(projections) || nilValue(selections) {
		return nil, imageagent.ErrValidation
	}
	return &ImageSetPublisher{projections: projections, selections: selections}, nil
}
func (p *ImageSetPublisher) PublishApprovedImageSet(ctx context.Context, input imageagent.PublishImageSetInput) (imageagent.PublicationAcknowledgement, error) {
	if p == nil || nilValue(p.projections) || nilValue(p.selections) || ctx == nil || !canonical(input.RunID) || !canonical(input.TenantID) || !canonical(input.UserID) || input.PlanRevision <= 0 {
		return imageagent.PublicationAcknowledgement{}, imageagent.ErrValidation
	}
	if err := ctx.Err(); err != nil {
		return imageagent.PublicationAcknowledgement{}, err
	}
	projection, err := p.projections.GetProjection(ctx, imageagent.RunScope{TenantID: input.TenantID, OwnerUserID: input.UserID, RunID: input.RunID})
	if err != nil {
		return imageagent.PublicationAcknowledgement{}, fmt.Errorf("load selected image set projection: %w", err)
	}
	run := projection.Run
	if run.ID != input.RunID || run.TenantID != input.TenantID || run.UserID != input.UserID || run.Status != imageagent.RunStatusAwaitingFinalApproval || run.ActivePlanRevision != input.PlanRevision || projection.Plan.Revision != input.PlanRevision || imageagent.ValidateImageSetAdmission(run, projection.Plan) != nil {
		return imageagent.PublicationAcknowledgement{}, imageagent.ErrCommandBlocked
	}
	if err := imageagent.ValidateImageSetSelectionIntent(run.ID, projection.Plan, projection.ResultDigest, input.Selection.ActionID, &input.Selection); err != nil {
		return imageagent.PublicationAcknowledgement{}, err
	}
	digest, err := imageagent.ImageSetResultDigest(projection.Plan, projection.Slots, projection.RecoverableEffects)
	if err != nil || digest != input.ResultDigest || digest != projection.ResultDigest {
		return imageagent.PublicationAcknowledgement{}, imageagent.ErrRevisionConflict
	}
	pending := projection.PendingCommand
	if pending == nil || pending.ActionID != input.Selection.ActionID || pending.Kind != "approve_results" || pending.Phase != imageagent.ImageSetApprovalPublicationStarted || pending.PlanRevision != input.PlanRevision || pending.SelectionDigest != input.Selection.SelectionDigest || pending.ResultDigest != digest {
		return imageagent.PublicationAcknowledgement{}, imageagent.ErrCommandBlocked
	}
	// Select owns immutable receipt replay and the inventory-head transaction.
	// An ambiguous acknowledgement must remain on this original command.
	receipt, err := p.selections.Select(ctx, *imageagent.CloneImageSetCommand(&input.Selection))
	if err != nil {
		return imageagent.PublicationAcknowledgement{}, fmt.Errorf("select approved image set: %w", err)
	}
	if receipt.ActionID != input.Selection.ActionID || len(receipt.AssetIDs) == 0 || len(receipt.AssetIDs) != len(input.Selection.Choices) {
		return imageagent.PublicationAcknowledgement{}, imageagent.ErrRevisionConflict
	}
	return imageagent.PublicationAcknowledgement{ProductKey: projection.Plan.Set.Source.ProductID, RunID: run.ID, PlanRevision: input.PlanRevision, ResultDigest: digest, ActionID: receipt.ActionID, AssetIDs: slices.Clone(receipt.AssetIDs)}, nil
}
