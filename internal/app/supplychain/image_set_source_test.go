package supplychainapp

import (
	"context"
	"github.com/stretchr/testify/require"
	"task-processor/internal/authidentity"
	"task-processor/internal/imageagent"
	"task-processor/internal/listing/preparation"
	"task-processor/internal/product/asset"
	"task-processor/internal/product/catalog"
	"task-processor/internal/product/collection"
	"testing"
	"time"
)

type selectedImageSourceFixture struct{ source preparation.AuthorizedSource }

func (f selectedImageSourceFixture) Select(context.Context, string) (preparation.AuthorizedSource, error) {
	return f.source, nil
}

func TestSupplySetSourceReusesRetainedSourceOwnerAndLiveExecutionAuthorization(t *testing.T) {
	_, scope, records, _, _, _, auth := uploadFixture(t)
	original := catalog.PublishedSnapshot{Identity: catalog.SnapshotIdentity{TenantID: scope.OrganizationID, ProductKey: records.saved.Source.Source.ProductKey}, PublicationID: records.saved.Source.Source.PublicationID, Version: 1, Snapshot: catalog.ProductSnapshot{Title: "Original", Images: []catalog.Image{{URL: "https://images.example/source.png"}}}}
	collections, err := collection.NewService(uploadCollections{}, auth, uploadUnusedSources{})
	require.NoError(t, err)
	sources := uploadSources{source: records.saved.Source}
	preparations, err := preparation.NewService(sources, collections, auth)
	require.NoError(t, err)
	selector, err := preparation.NewSourceSelector(preparations, collections, sources, uploadCatalog{original})
	require.NoError(t, err)
	selector, err = selector.WithExecution(auth, collection.ExecutionOwnerAuthority{Authorization: auth})
	require.NoError(t, err)
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	ctx = authidentity.WithAuthenticatedIdentity(ctx, authidentity.AuthenticatedIdentity{TenantID: scope.OrganizationID, EffectiveOrganizationID: scope.OrganizationID, UserID: scope.ActorID, EffectiveMemberID: scope.MemberID})
	reader := ImageSetSources{ExecutionSources: selector, ExecutionAuthorization: auth, Products: EffectiveProductReader{}}
	identity := imageagent.ExecutionIdentity{ScopeProtocol: imageagent.OrganizationScopeProtocol, TenantID: scope.OrganizationID, UserID: scope.ActorID, MemberID: scope.MemberID, BusinessTaskID: records.saved.Source.ID}
	input := imageagent.PrepareImageSetInput{ContextID: records.saved.Source.ID, Target: imageagent.ImageTargetSelection{Platform: "product"}}
	result, err := reader.ReadImageSetSource(ctx, identity, input)
	require.NoError(t, err)
	require.Equal(t, records.saved.Source.Source.ProductKey, result.Source.ProductID)
	require.Equal(t, records.saved.Source.ID, result.Source.OperationID)
	require.Equal(t, records.saved.Source.Source.PublicationID, result.Source.OriginalPublicationID)
	require.Len(t, result.Catalog.Assets, 1)
	require.Equal(t, collection.StableID(scope.OrganizationID, records.saved.Source.Source.PublicationID, "source-image", result.Catalog.Assets[0].URL), result.Catalog.Assets[0].ID)
	selected, err := selector.SelectForExecution(ctx, scope, input.ContextID)
	require.NoError(t, err)
	selectionReader := SourceImageReader{Sources: selectedImageSourceFixture{selected}, Products: EffectiveProductReader{}, Authorization: auth}
	selection, err := selectionReader.ReadSourceSelection(ctx, asset.SourceSelectionRequest{ItemID: input.ContextID, OriginalPublicationID: original.PublicationID, OriginalSnapshotVersion: 1, EffectiveCatalogVersion: 1, TargetPlatform: "product"})
	require.NoError(t, err, "generic asset approval uses the same explicit source owner")
	require.Equal(t, "product", selection.TargetPlatform)
	require.Equal(t, result.Catalog.Assets[0].ID, selection.Images[0].ID)
	auth.denied = true
	_, err = reader.ReadImageSetSource(ctx, identity, input)
	require.Error(t, err, "per-dispatch source permissions must be live")
	auth.denied = false
	identity.MemberID = "replaced-member"
	_, err = reader.ReadImageSetSource(ctx, identity, input)
	require.ErrorIs(t, err, imageagent.ErrIdentityRequired)
}
