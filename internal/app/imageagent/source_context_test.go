package imageagentapp

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"image"
	"image/png"
	"testing"

	"github.com/stretchr/testify/require"
	"task-processor/internal/authidentity"
	"task-processor/internal/imageagent"
)

type sourceContextFixture struct {
	preparation imageagent.ImageSetPreparation
	denied      error
}

func (f *sourceContextFixture) ReadImageSetSource(context.Context, imageagent.ExecutionIdentity, imageagent.PrepareImageSetInput) (imageagent.ImageSetPreparation, error) {
	return f.preparation, f.denied
}

func TestImageSetSourceContextUsesExactAuthorizedOriginalBytesAndRechecksThem(t *testing.T) {
	catalog, err := imageagent.NormalizeAssetCatalog(imageagent.AssetCatalog{ProductContext: imageagent.ProductContextRef{ProductID: "product", SourceSnapshotVersion: 1, Title: "Original title"}, Assets: []imageagent.AuthorizedAsset{{ID: "first", Type: imageagent.AuthorizedAssetSource, URL: "https://source.example/first.png"}, {ID: "second", Type: imageagent.AuthorizedAssetSource, URL: "https://source.example/second.png"}}})
	require.NoError(t, err)
	source := &sourceContextFixture{preparation: imageagent.ImageSetPreparation{Catalog: catalog, Source: imageagent.ImageSourceBinding{ContextKind: imageagent.ImageSourceAcquisition, ProductID: "product", OperationID: "source", OriginalPublicationID: "publication", OriginalVersion: 1, EffectiveVersion: 1}}}
	var raw bytes.Buffer
	require.NoError(t, png.Encode(&raw, image.NewNRGBA(image.Rect(0, 0, 1024, 1024))))
	content := raw.Bytes()
	reads := []string{}
	reader, err := NewImageSetContextReader(source, nil, func(_ context.Context, asset imageagent.AuthorizedAsset, maximum int64) ([]byte, error) {
		require.EqualValues(t, 1<<20, maximum)
		reads = append(reads, asset.ID)
		return content, nil
	}, 1<<20)
	require.NoError(t, err)
	identity := imageagent.ExecutionIdentity{ScopeProtocol: imageagent.OrganizationScopeProtocol, TenantID: "org", UserID: "actor", MemberID: "member", BusinessTaskID: "source"}
	ctx := authidentity.WithAuthenticatedIdentity(context.Background(), authidentity.AuthenticatedIdentity{TenantID: "org", EffectiveOrganizationID: "org", UserID: "actor", EffectiveMemberID: "member"})
	input := imageagent.PrepareImageSetInput{ContextID: "source", Target: imageagent.ImageTargetSelection{Platform: "product"}, SharedOriginalIDs: []string{"second", "first"}}
	resolved, err := reader.ResolveImageSet(ctx, identity, input)
	require.NoError(t, err)
	require.Equal(t, []string{"first", "second"}, reads)
	require.Len(t, resolved.Observations, 2)
	hash := sha256.Sum256(content)
	require.Equal(t, hex.EncodeToString(hash[:]), resolved.Observations[0].SHA256)
	require.Equal(t, 1024, resolved.Catalog.Assets[0].Width)
	require.Equal(t, resolved.Catalog.Manifest.Hash, resolved.Source.CatalogHash)
	projection := imageagent.RunProjection{Run: imageagent.Run{ID: "run", TenantID: "org", UserID: "actor", MemberID: "member", BusinessTaskID: "source"}, AssetCatalog: resolved.Catalog, Plan: imageagent.Plan{SourceAssetIDs: []string{"first", "second"}, Set: &imageagent.ImageSetPlan{Source: resolved.Source, Target: resolved.Target}, Slots: []imageagent.Slot{{SourceAssetIDs: []string{"second", "first"}, Recipe: &imageagent.ImageSlotRecipe{References: []imageagent.ImageSourceObservation{resolved.Observations[1], resolved.Observations[0]}}}}}}
	beforeGuard := len(reads)
	require.NoError(t, reader.AuthorizeImageSetSource(ctx, identity, projection))
	require.Len(t, reads, beforeGuard, "dispatch ownership checks must not redownload every selected original")
	source.denied = imageagent.ErrIdentityRequired
	require.ErrorIs(t, reader.AuthorizeImageSetSource(ctx, identity, projection), imageagent.ErrIdentityRequired)
	source.denied = nil
	oldURL := source.preparation.Catalog.Assets[0].URL
	source.preparation.Catalog.Assets[0].URL = "https://source.example/replaced.png"
	require.ErrorIs(t, reader.AuthorizeImageSetSource(ctx, identity, projection), imageagent.ErrRevisionConflict)
	source.preparation.Catalog.Assets[0].URL = oldURL
	require.Len(t, reads, beforeGuard)
	require.NoError(t, reader.RevalidateImageSet(ctx, identity, projection))
	var changed bytes.Buffer
	require.NoError(t, png.Encode(&changed, image.NewNRGBA(image.Rect(0, 0, 1000, 1000))))
	content = changed.Bytes()
	require.ErrorIs(t, reader.RevalidateImageSet(ctx, identity, projection), imageagent.ErrRevisionConflict)
	input.SharedOriginalIDs = []string{"forged"}
	before := len(reads)
	_, err = reader.ResolveImageSet(ctx, identity, input)
	require.Error(t, err)
	require.Len(t, reads, before, "reject the reference before any source fetch")
	identity.MemberID = "other-member"
	_, err = reader.ResolveImageSet(ctx, identity, input)
	require.ErrorIs(t, err, imageagent.ErrIdentityRequired)
}
