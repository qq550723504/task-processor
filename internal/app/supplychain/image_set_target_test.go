package supplychainapp

import (
	"context"
	"github.com/stretchr/testify/require"
	"task-processor/internal/authidentity"
	"task-processor/internal/imageagent"
	"task-processor/internal/product/asset"
	"task-processor/internal/product/collection"
	"testing"
)

type imageTargetSourceFixture struct {
	source   asset.SourceSelection
	denied   bool
	requests []asset.SourceSelectionRequest
}

func (f *imageTargetSourceFixture) ReadSourceSelection(_ context.Context, input asset.SourceSelectionRequest) (asset.SourceSelection, error) {
	f.requests = append(f.requests, input)
	if f.denied {
		return asset.SourceSelection{}, asset.ErrSourceApprovalForbidden
	}
	return f.source, nil
}

func TestImageSetTargetUsesActualSavedCategoryBindingAndRejectsUnsupportedNativeSize(t *testing.T) {
	uploader, scope, records, merchant, _, _, _ := uploadFixture(t)
	verified := authidentity.AuthenticatedIdentity{TenantID: scope.OrganizationID, EffectiveOrganizationID: scope.OrganizationID, UserID: scope.ActorID, EffectiveMemberID: scope.MemberID}
	ctx := authidentity.WithAuthenticatedIdentity(context.Background(), verified)
	source := imageagent.ImageSourceBinding{ProductID: records.saved.Source.Source.ProductKey, OperationID: records.saved.Source.ID, OriginalPublicationID: records.saved.Source.Source.PublicationID, OriginalVersion: 1, EffectiveVersion: 1}
	owner := &imageTargetSourceFixture{source: asset.SourceSelection{TenantID: scope.OrganizationID, ActorID: scope.ActorID, MemberID: scope.MemberID, ItemID: records.saved.Source.ID, ProductKey: source.ProductID, OriginalPublicationID: source.OriginalPublicationID, OriginalSnapshotVersion: 1, EffectiveCatalogVersion: 1, TargetPlatform: "shein"}}
	reader := ImageSetTargetRules{Records: records, Sources: owner, Rules: uploader.dependencies.Rules}
	input := imageagent.PrepareImageSetInput{ContextID: source.OperationID, Target: imageagent.ImageTargetSelection{Platform: "shein", RecordID: records.saved.ID, StoreID: merchant.binding.StoreID, Site: merchant.binding.Site, CategoryID: 123}, OfficialPlacements: map[string]imageagent.OfficialImagePlacement{"main": {Group: "skc", SKC: 0, SKU: 0, Type: 1, Sort: 1, Site: "shein-us"}}}
	identity := imageagent.ExecutionIdentity{ScopeProtocol: imageagent.OrganizationScopeProtocol, TenantID: scope.OrganizationID, UserID: scope.ActorID, MemberID: scope.MemberID, BusinessTaskID: source.OperationID}
	target, positions, err := reader.ResolveImageSetTarget(ctx, identity, input, imageagent.ImageSetPreparation{Source: source})
	require.NoError(t, err)
	require.Equal(t, int64(456), target.ProductTypeID)
	require.Equal(t, merchant.binding.ApplicationID, target.ApplicationID)
	require.Equal(t, records.saved.ID, target.RecordID)
	require.Equal(t, "shein-images-v1", target.RequirementVersion)
	require.Equal(t, input.OfficialPlacements, positions)
	require.NotEmpty(t, target.RequirementDigest)
	assetTarget := &asset.ImageSetTarget{RecordID: target.RecordID, StoreID: target.StoreID, Site: target.Site, ApplicationID: target.ApplicationID, ApplicationMode: target.ApplicationMode, CategoryID: target.CategoryID, ProductTypeID: target.ProductTypeID, AttributesDigest: target.AttributesDigest, VariantsDigest: target.VariantsDigest}
	selectedAssets := []asset.ApprovedAsset{{ID: "main", URL: "https://images.example/source.jpg", SourceApproval: &asset.SourceApprovalProvenance{}, OfficialPlacement: &asset.ImageOfficialPlacement{Group: "skc", Type: 1, Sort: 1, Site: "shein-us"}}}
	assetResolver := ImageSetAssetTargetResolver{Rules: reader, Images: uploadProbe{}}
	approval, err := assetResolver.ResolveImageSetTarget(ctx, owner.source, assetTarget, selectedAssets)
	require.NoError(t, err, "selected material may be saved before Listing fills all publish positions")
	require.Equal(t, assetTarget, approval.Target)
	require.Equal(t, 900, selectedAssets[0].Width, "originals with unknown Catalog dimensions use actual bytes")
	require.Equal(t, target.RequirementDigest, approval.RequirementDigest)
	selectedAssets[0].SourceApproval = nil
	selectedAssets[0].GenerationEvidence = &asset.GenerationEvidence{ArtifactHash: collection.Digest("different artifact")}
	_, err = assetResolver.ResolveImageSetTarget(ctx, owner.source, assetTarget, selectedAssets)
	require.ErrorIs(t, err, asset.ErrApprovalConflict, "generated material cannot drift from its original artifact proof")
	input.OfficialPlacements["main"] = imageagent.OfficialImagePlacement{Group: "detail", Type: 7, Sort: 1, Site: "shein-us"}
	_, _, err = reader.ResolveImageSetTarget(ctx, identity, input, imageagent.ImageSetPreparation{Source: source})
	require.ErrorIs(t, err, imageagent.ErrCommandBlocked, "native 1024 cannot satisfy official 3:4 detail dimensions")
	input.OfficialPlacements["main"] = positions["main"]
	owner.denied = true
	_, _, err = reader.ResolveImageSetTarget(ctx, identity, input, imageagent.ImageSetPreparation{Source: source})
	require.Error(t, err)
	owner.denied = false
	input.Target.Site = "shein-fr"
	_, _, err = reader.ResolveImageSetTarget(ctx, identity, input, imageagent.ImageSetPreparation{Source: source})
	require.Error(t, err, "the available US contract cannot stand in for another site")
}
