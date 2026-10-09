package imageagentapp

import (
	"context"
	"github.com/stretchr/testify/require"
	"task-processor/internal/authidentity"
	"task-processor/internal/imageagent"
	"task-processor/internal/product/asset"
	"testing"
)

func TestImageSetAssetSourceUsesTheSameExactOriginalOwner(t *testing.T) {
	owner := &sourceContextFixture{preparation: imageagent.ImageSetPreparation{Source: imageagent.ImageSourceBinding{ContextKind: imageagent.ImageSourceAcquisition, ProductID: "product", OperationID: "source", OriginalPublicationID: "publication", OriginalVersion: 1, EffectiveVersion: 1}, Catalog: imageagent.AssetCatalog{Assets: []imageagent.AuthorizedAsset{{ID: "original", Type: imageagent.AuthorizedAssetSource, URL: "https://source.example/image.png"}}}}}
	reader := ImageSetSourceSelectionReader{Sources: owner}
	ctx := authidentity.WithAuthenticatedIdentity(context.Background(), authidentity.AuthenticatedIdentity{TenantID: "org", EffectiveOrganizationID: "org", UserID: "actor", EffectiveMemberID: "member"})
	request := asset.SourceSelectionRequest{ContextKind: "acquisition", ItemID: "source", OriginalPublicationID: "publication", OriginalSnapshotVersion: 1, EffectiveCatalogVersion: 1, TargetPlatform: "product"}
	selected, err := reader.ReadSourceSelection(ctx, request)
	require.NoError(t, err)
	require.Equal(t, "member", selected.MemberID)
	require.Equal(t, "product", selected.ProductKey)
	require.Len(t, selected.Images, 1)
	require.Equal(t, asset.ReferenceHash("original", "https://source.example/image.png"), selected.Images[0].ReferenceHash)
	for _, change := range []func(*asset.SourceSelectionRequest){func(r *asset.SourceSelectionRequest) { r.ContextKind = "supply" }, func(r *asset.SourceSelectionRequest) { r.ItemID = "other" }, func(r *asset.SourceSelectionRequest) { r.OriginalPublicationID = "other" }, func(r *asset.SourceSelectionRequest) { r.EffectiveCatalogVersion = 2 }} {
		altered := request
		change(&altered)
		_, err = reader.ReadSourceSelection(ctx, altered)
		require.Error(t, err)
	}
	owner.denied = imageagent.ErrIdentityRequired
	_, err = reader.ReadSourceSelection(ctx, request)
	require.ErrorIs(t, err, imageagent.ErrIdentityRequired)
	_, err = reader.ReadSourceSelection(context.Background(), request)
	require.ErrorIs(t, err, asset.ErrSourceApprovalForbidden)
}
