package supplychainapp

import (
	"context"
	"reflect"
	"task-processor/internal/imageagent"
	record "task-processor/internal/listing/record/target"
	"task-processor/internal/marketplace/shein/goods"
	"task-processor/internal/product/asset"
)

type ImageSetAssetTargetResolver struct {
	Rules  ImageSetTargetRules
	Images record.TargetImageReader
}

func (r ImageSetAssetTargetResolver) ResolveImageSetTarget(ctx context.Context, source asset.SourceSelection, target *asset.ImageSetTarget, assets []asset.ApprovedAsset) (asset.ImageSetTargetResolution, error) {
	var empty asset.ImageSetTargetResolution
	if target == nil || !target.Valid() || r.Images == nil || len(assets) == 0 || len(assets) > 40 {
		return empty, asset.ErrInvalidApproval
	}
	identity := imageagent.ExecutionIdentity{ScopeProtocol: imageagent.OrganizationScopeProtocol, TenantID: source.TenantID, UserID: source.ActorID, MemberID: source.MemberID, BusinessTaskID: source.ItemID}
	input := imageagent.ImageTargetSelection{Platform: source.TargetPlatform, RecordID: target.RecordID, StoreID: target.StoreID, Site: target.Site, CategoryID: target.CategoryID}
	binding := imageagent.ImageSourceBinding{ProductID: source.ProductKey, OperationID: source.ItemID, OriginalPublicationID: source.OriginalPublicationID, OriginalVersion: source.OriginalSnapshotVersion, EffectiveVersion: source.EffectiveCatalogVersion, ApplyReceiptID: source.ApplyReceiptID}
	slots := make([]goods.OfficialImageSlot, 0, len(assets))
	positions := make([]*asset.ImageOfficialPlacement, len(assets))
	for index, value := range assets {
		if value.OfficialPlacement == nil || value.OfficialPlacement.Site != target.Site {
			return empty, asset.ErrInvalidApproval
		}
		position := *value.OfficialPlacement
		positions[index] = &position
		slots = append(slots, goods.OfficialImageSlot{Group: position.Group, SKC: position.SKC, SKU: position.SKU, Type: position.Type, Sort: position.Sort, AssetID: value.ID})
	}
	resolved, err := r.Rules.readTarget(ctx, identity, input, binding, slots)
	if err != nil {
		return empty, err
	}
	value := resolved.target
	actual := &asset.ImageSetTarget{RecordID: value.RecordID, StoreID: value.StoreID, Site: value.Site, ApplicationID: value.ApplicationID, ApplicationMode: value.ApplicationMode, CategoryID: value.CategoryID, ProductTypeID: value.ProductTypeID, AttributesDigest: value.AttributesDigest, VariantsDigest: value.VariantsDigest}
	if !reflect.DeepEqual(target, actual) {
		return empty, asset.ErrApprovalConflict
	}
	observations, err := record.ProbeTargetImages(ctx, r.Images, goods.OfficialDraftInput{Images: slots}, asset.ApprovedAssetInventory{Assets: assets})
	if err != nil || len(observations) != len(assets) {
		return empty, asset.ErrInvalidApproval
	}
	dimensions := make([]goods.OfficialImageDimensions, 0, len(assets))
	for index, item := range assets {
		observation := observations[index]
		if item.GenerationEvidence != nil && (observation.ContentHash != item.GenerationEvidence.ArtifactHash || observation.Width != item.Width || observation.Height != item.Height) {
			return empty, asset.ErrApprovalConflict
		}
		if item.SourceApproval == nil && (observation.Width != item.Width || observation.Height != item.Height) {
			return empty, asset.ErrApprovalConflict
		}
		dimensions = append(dimensions, goods.OfficialImageDimensions{AssetID: item.ID, Width: observation.Width, Height: observation.Height})
		if item.SourceApproval != nil {
			assets[index].Width, assets[index].Height = observation.Width, observation.Height
		}
	}
	if len(resolved.requirements.ValidateMaterialSelection(slots, dimensions)) > 0 {
		return empty, asset.ErrInvalidApproval
	}
	return asset.ImageSetTargetResolution{Target: actual, RequirementDigest: value.RequirementDigest, Placements: positions}, nil
}
