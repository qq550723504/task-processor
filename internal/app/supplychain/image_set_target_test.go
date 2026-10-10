package supplychainapp

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"github.com/stretchr/testify/require"
	"image"
	"image/png"
	"task-processor/internal/authidentity"
	"task-processor/internal/imageagent"
	"task-processor/internal/product/asset"
	"task-processor/internal/product/collection"
	"testing"
)

func TestOfficialImageSetGeneratedMaterialUsesImmutableBytesBeforeRules(t *testing.T) {
	uploader, scope, records, merchant, _, _, _ := uploadFixture(t)
	ctx := authidentity.WithAuthenticatedIdentity(context.Background(), authidentity.AuthenticatedIdentity{TenantID: scope.OrganizationID, EffectiveOrganizationID: scope.OrganizationID, UserID: scope.ActorID, EffectiveMemberID: scope.MemberID})
	source := asset.SourceSelection{TenantID: scope.OrganizationID, ActorID: scope.ActorID, MemberID: scope.MemberID, ItemID: records.saved.Source.ID, ProductKey: records.saved.Source.Source.ProductKey, OriginalPublicationID: records.saved.Source.Source.PublicationID, OriginalSnapshotVersion: 1, EffectiveCatalogVersion: 1, TargetPlatform: "shein"}
	owner := &imageTargetSourceFixture{source: source}
	rules := ImageSetTargetRules{Records: records, Sources: owner, Rules: uploader.dependencies.Rules}
	input := imageagent.PrepareImageSetInput{ContextID: source.ItemID, Target: imageagent.ImageTargetSelection{Platform: "shein", RecordID: records.saved.ID, StoreID: merchant.binding.StoreID, Site: merchant.binding.Site, CategoryID: 123}, OfficialPlacements: map[string]imageagent.OfficialImagePlacement{"main": {Group: "skc", Type: 1, Sort: 1, Site: "shein-us"}}}
	prepared, _, err := rules.ResolveImageSetTarget(ctx, imageagent.ExecutionIdentity{ScopeProtocol: imageagent.OrganizationScopeProtocol, TenantID: source.TenantID, UserID: source.ActorID, MemberID: source.MemberID, BusinessTaskID: source.ItemID}, input, imageagent.ImageSetPreparation{Source: imageagent.ImageSourceBinding{ProductID: source.ProductKey, OperationID: source.ItemID, OriginalPublicationID: source.OriginalPublicationID, OriginalVersion: 1, EffectiveVersion: 1}})
	require.NoError(t, err)
	target := &asset.ImageSetTarget{RecordID: prepared.RecordID, StoreID: prepared.StoreID, Site: prepared.Site, ApplicationID: prepared.ApplicationID, ApplicationMode: prepared.ApplicationMode, CategoryID: prepared.CategoryID, ProductTypeID: prepared.ProductTypeID, AttributesDigest: prepared.AttributesDigest, VariantsDigest: prepared.VariantsDigest}
	var encoded bytes.Buffer
	require.NoError(t, png.Encode(&encoded, image.NewRGBA(image.Rect(0, 0, 1024, 1024))))
	raw := encoded.Bytes()
	hash := sha256.Sum256(raw)
	selected := []asset.ApprovedAsset{{ID: "generated-main", URL: "https://localhost:33644/image-agent-assets/images/published.png", Width: 1024, Height: 1024, GenerationEvidence: &asset.GenerationEvidence{ArtifactHash: hex.EncodeToString(hash[:])}, OfficialPlacement: &asset.ImageOfficialPlacement{Group: "skc", Type: 1, Sort: 1, Site: "shein-us"}}}
	resolver := ImageSetAssetTargetResolver{Rules: rules, Images: NewPublicImageProbe()}
	reads := 0
	resolver.ReadGeneratedBytes = func(_ context.Context, actualSource asset.SourceSelection, approved asset.ApprovedAsset, maximum int64) ([]byte, error) {
		reads++
		require.Equal(t, source, actualSource)
		require.Equal(t, selected[0], approved)
		require.Positive(t, maximum)
		return raw, nil
	}
	resolution, err := resolver.ResolveImageSetTarget(ctx, source, target, selected)
	require.NoError(t, err, "verified generated materials must use immutable storage before official validation")
	require.Equal(t, target, resolution.Target)
	require.Equal(t, 1, reads)
	selected[0].GenerationEvidence.ArtifactHash = collection.Digest("changed")
	_, err = resolver.ResolveImageSetTarget(ctx, source, target, selected)
	require.ErrorIs(t, err, asset.ErrApprovalConflict)
	selected[0].GenerationEvidence.ArtifactHash = hex.EncodeToString(hash[:])
	selected[0].Width = 900
	_, err = resolver.ResolveImageSetTarget(ctx, source, target, selected)
	require.ErrorIs(t, err, asset.ErrApprovalConflict)
	selected[0].Width = 1024
	raw = raw[:40]
	_, err = resolver.ResolveImageSetTarget(ctx, source, target, selected)
	require.ErrorIs(t, err, asset.ErrInvalidApproval, "a valid image header is insufficient")
	resolver.ReadGeneratedBytes = nil
	_, err = resolver.ResolveImageSetTarget(ctx, source, target, selected)
	require.ErrorIs(t, err, asset.ErrInvalidApproval, "missing immutable reader must not fall back to URL download")
	selected[0].GenerationEvidence = nil
	selected[0].SourceApproval = &asset.SourceApprovalProvenance{}
	_, err = resolver.ResolveImageSetTarget(ctx, source, target, selected)
	require.ErrorIs(t, err, asset.ErrInvalidApproval, "original loopback URLs still fail public image policy")
}

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
	source := imageagent.ImageSourceBinding{ContextKind: imageagent.ImageSourceAcquisition, ProductID: records.saved.Source.Source.ProductKey, OperationID: records.saved.Source.ID, OriginalPublicationID: records.saved.Source.Source.PublicationID, OriginalVersion: 1, EffectiveVersion: 1}
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
	var generated bytes.Buffer
	require.NoError(t, png.Encode(&generated, image.NewRGBA(image.Rect(0, 0, 900, 900))))
	assetResolver.ReadGeneratedBytes = func(context.Context, asset.SourceSelection, asset.ApprovedAsset, int64) ([]byte, error) {
		return generated.Bytes(), nil
	}
	_, err = assetResolver.ResolveImageSetTarget(ctx, owner.source, assetTarget, selectedAssets)
	require.ErrorIs(t, err, asset.ErrApprovalConflict, "generated material cannot drift from its original artifact proof")
	input.OfficialPlacements["main"] = imageagent.OfficialImagePlacement{Group: "detail", Type: 7, Sort: 1, Site: "shein-us"}
	_, _, err = reader.ResolveImageSetTarget(ctx, identity, input, imageagent.ImageSetPreparation{Source: source})
	require.ErrorIs(t, err, imageagent.ErrCommandBlocked, "native 1024 cannot satisfy official 3:4 detail dimensions")
	input.OfficialPlacements["main"] = positions["main"]
	input.OfficialPlacements = map[string]imageagent.OfficialImagePlacement{}
	for i := 1; i <= 11; i++ {
		input.OfficialPlacements[fmt.Sprint(i)] = imageagent.OfficialImagePlacement{Group: "skc", Type: 2, Sort: i, Site: "shein-us"}
	}
	_, _, err = reader.ResolveImageSetTarget(ctx, identity, input, imageagent.ImageSetPreparation{Source: source})
	require.Error(t, err, "official per-type limits must block preparation before any paid dispatch")
	input.OfficialPlacements = positions
	owner.denied = true
	_, _, err = reader.ResolveImageSetTarget(ctx, identity, input, imageagent.ImageSetPreparation{Source: source})
	require.Error(t, err)
	owner.denied = false
	input.Target.Site = "shein-fr"
	_, _, err = reader.ResolveImageSetTarget(ctx, identity, input, imageagent.ImageSetPreparation{Source: source})
	require.Error(t, err, "the available US contract cannot stand in for another site")
}
