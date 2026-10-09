package imageagentapp

import (
	"context"
	record "task-processor/internal/listing/record/target"
	"task-processor/internal/marketplace/shein/goods"
	"task-processor/internal/product/asset"
)

// Original material has no official publishing claim. Its actual artifact is
// still validated by the existing bounded probe before the Asset transaction.
type ImageSetMaterialTargetResolver struct {
	Official asset.ImageSetTargetResolver
	Images   record.TargetImageReader
}

func (r ImageSetMaterialTargetResolver) ResolveImageSetTarget(ctx context.Context, source asset.SourceSelection, target *asset.ImageSetTarget, selected []asset.ApprovedAsset) (asset.ImageSetTargetResolution, error) {
	if source.TargetPlatform != "product" {
		if r.Official == nil {
			return asset.ImageSetTargetResolution{}, asset.ErrRepositoryUnavailable
		}
		return r.Official.ResolveImageSetTarget(ctx, source, target, selected)
	}
	if target != nil || r.Images == nil || len(selected) == 0 || len(selected) > 40 {
		return asset.ImageSetTargetResolution{}, asset.ErrInvalidApproval
	}
	slots := make([]goods.OfficialImageSlot, 0, len(selected))
	for _, item := range selected {
		if item.OfficialPlacement != nil {
			return asset.ImageSetTargetResolution{}, asset.ErrInvalidApproval
		}
		slots = append(slots, goods.OfficialImageSlot{AssetID: item.ID, Type: 1})
	}
	observations, err := record.ProbeTargetImages(ctx, r.Images, goods.OfficialDraftInput{Images: slots}, asset.ApprovedAssetInventory{Assets: selected})
	if err != nil || len(observations) != len(selected) {
		return asset.ImageSetTargetResolution{}, asset.ErrInvalidApproval
	}
	for index, item := range selected {
		observed := observations[index]
		if item.GenerationEvidence != nil && (observed.ContentHash != item.GenerationEvidence.ArtifactHash || observed.Width != item.Width || observed.Height != item.Height) {
			return asset.ImageSetTargetResolution{}, asset.ErrApprovalConflict
		}
		if item.SourceApproval == nil && (observed.Width != item.Width || observed.Height != item.Height) {
			return asset.ImageSetTargetResolution{}, asset.ErrApprovalConflict
		}
		if item.SourceApproval != nil {
			selected[index].Width, selected[index].Height = observed.Width, observed.Height
		}
	}
	return asset.ImageSetTargetResolution{}, nil
}
