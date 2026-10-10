package imageagentapp

import (
	"context"
	"errors"
	"github.com/stretchr/testify/require"
	"task-processor/internal/authidentity"
	"task-processor/internal/imageagent"
	"task-processor/internal/product/catalog"
	"task-processor/internal/product/sourcing"
	"testing"
)

type setAcquisitionReceiptFixture struct {
	published sourcing.PublishedAcquisition
	calls     int
}

func (f *setAcquisitionReceiptFixture) ReadPublished(context.Context, string) (sourcing.PublishedAcquisition, error) {
	f.calls++
	return f.published, nil
}

type setSourceAuthorizationFixture struct {
	denied bool
	calls  int
}

func (f *setSourceAuthorizationFixture) AuthorizeExecution(context.Context, imageagent.ExecutionIdentity) error {
	f.calls++
	if f.denied {
		return imageagent.ErrIdentityRequired
	}
	return nil
}

func TestAcquisitionSetSourceKeepsOriginalIndexesVariantImagesAndRealFacts(t *testing.T) {
	identity := imageagent.ExecutionIdentity{ScopeProtocol: imageagent.OrganizationScopeProtocol, TenantID: "org", UserID: "actor", MemberID: "member", BusinessTaskID: "operation"}
	ctx := authidentity.WithAuthenticatedIdentity(context.Background(), authidentity.AuthenticatedIdentity{TenantID: "org", EffectiveOrganizationID: "org", UserID: "actor", EffectiveMemberID: "member"})
	original := catalog.PublishedSnapshot{Identity: catalog.SnapshotIdentity{TenantID: "org", ProductKey: "product"}, Version: 4, PublicationID: "publication", Snapshot: catalog.ProductSnapshot{Title: "Real product", CategoryPath: []string{"家居", "收纳"}, SellingPoints: []string{"可折叠"}, Images: []catalog.Image{{URL: "https://images.example/main.png"}, {URL: "http://127.0.0.1/private.png"}}, Variants: []catalog.Variant{{SourceID: "blue", Images: []catalog.Image{{URL: "https://images.example/blue.png"}}}}, Specifications: &catalog.Specifications{Dimensions: &catalog.Dimensions{Length: 20, Width: 10, Height: 5, Unit: "cm"}}}}
	receipts := &setAcquisitionReceiptFixture{published: sourcing.PublishedAcquisition{Snapshot: original, Result: sourcing.AcquisitionResult{Operation: sourcing.AcquisitionOperation{ID: "operation", Scope: sourcing.PublicationScope{OrganizationID: "org", ActorID: "actor"}, State: sourcing.AcquisitionPublished}, Publication: &sourcing.PersistedPublication{Receipt: sourcing.PublicationReceipt{OrganizationID: "org", ActorID: "actor", ProductKey: "product", CatalogVersion: 4, CatalogPublicationID: "publication"}}}}}
	authorize := &setSourceAuthorizationFixture{}
	r := AcquisitionImageSetSources{Receipts: receipts, Authorization: authorize}
	input := imageagent.PrepareImageSetInput{ContextID: "operation", Target: imageagent.ImageTargetSelection{Platform: "product"}}
	require.NotPanics(t, func() {
		_, err := r.ReadImageSetSource(nil, identity, input)
		require.ErrorIs(t, err, imageagent.ErrIdentityRequired)
	})
	result, err := r.ReadImageSetSource(ctx, identity, input)
	require.NoError(t, err)
	require.Equal(t, uint64(4), result.Source.OriginalVersion)
	require.Equal(t, uint64(4), result.Source.EffectiveVersion)
	require.Len(t, result.Catalog.Assets, 2)
	require.Equal(t, "catalog-image-1", result.Catalog.Assets[0].ID)
	require.Equal(t, "catalog-image-3", result.Catalog.Assets[1].ID)
	require.Contains(t, result.Evidence["specifications"], "cm")
	require.Contains(t, result.Evidence["selling_points"], "可折叠")
	require.Empty(t, result.Evidence["instructions"])
	require.Empty(t, result.Evidence["accessories"])
	before := receipts.calls
	authorize.denied = true
	_, err = r.ReadImageSetSource(ctx, identity, input)
	require.Error(t, err)
	require.Equal(t, before, receipts.calls)
	authorize.denied = false
	receipts.published.Result.Operation.Scope.ActorID = "other"
	_, err = r.ReadImageSetSource(ctx, identity, input)
	require.ErrorIs(t, err, imageagent.ErrIdentityRequired)
	receipts.published.Result.Operation.Scope.ActorID = "actor"
	input.EffectiveCatalogVersion = 5
	_, err = r.ReadImageSetSource(ctx, identity, input)
	require.True(t, errors.Is(err, imageagent.ErrRevisionConflict))
}
