package supplychainapp

import (
	"context"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	record "task-processor/internal/listing/record/target"
	"task-processor/internal/product/catalog"
	"task-processor/internal/product/enrichment"
	"task-processor/internal/product/review"
	"testing"
	"time"
)

func TestSourceImagesIncludesVariantEvidenceAndDeduplicatesURLs(t *testing.T) {
	snapshot := catalog.PublishedSnapshot{Identity: catalog.SnapshotIdentity{TenantID: "org", ProductKey: "product"}, PublicationID: "original", Version: 1, Snapshot: catalog.ProductSnapshot{Images: []catalog.Image{{URL: "https://example.com/main.jpg"}}, Variants: []catalog.Variant{{Images: []catalog.Image{{URL: "https://example.com/variant.jpg"}, {URL: "https://example.com/main.jpg"}}}}}}
	images := SourceImages(snapshot)
	require.Len(t, images, 2)
	require.Equal(t, "https://example.com/variant.jpg", images[1].URL)
	require.NotEqual(t, images[0].ID, images[1].ID)
}

type effectiveReviewFixture struct {
	review.Store
	record review.Record
	reads  int
}

func (f *effectiveReviewFixture) Read(_ context.Context, scope review.Scope, id string) (review.Record, error) {
	f.reads++
	if scope.Admin || scope.Org != f.record.Org || scope.Actor != f.record.Owner || id != f.record.ID {
		return review.Record{}, review.ErrNotFound
	}
	return f.record, nil
}
func (f *effectiveReviewFixture) ReadAppliedPublication(ctx context.Context, scope review.Scope, product string, version uint64, publication string) (review.Record, error) {
	r, err := f.Read(ctx, scope, f.record.ID)
	if err != nil || r.Input.ProductKey != product || r.Receipt == nil || r.Receipt.ProductVersion != version || r.Receipt.PublicationID != publication {
		return review.Record{}, review.ErrNotFound
	}
	return r, nil
}
func TestEffectiveProductRequiresExactOriginalOwnerAndAppliedTitleOnlyReceipt(t *testing.T) {
	upload, scope, records, _, _, _, auth := uploadFixture(t)
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	selected, err := upload.dependencies.Sources.SelectForExecution(ctx, scope, records.saved.Source.ID)
	require.NoError(t, err)
	_, _, original, err := selected.Read(ctx)
	require.NoError(t, err)
	id := uuid.NewString()
	applied := original
	applied.Version = 2
	applied.PublicationID = "review:applied"
	applied.Snapshot.Title = "Reviewed title"
	reviewOwner := &effectiveReviewFixture{record: review.Record{ID: id, Org: scope.OrganizationID, Owner: scope.ActorID, Input: review.CreateInput{ProductKey: original.Identity.ProductKey, BaseVersion: original.Version}, BasePublicationID: original.PublicationID, Policy: "title-review-v1", Before: original.Snapshot.Title, Title: applied.Snapshot.Title, State: "applied", Revision: 2, Receipt: &review.Receipt{ProposalID: id, Revision: 2, ProductVersion: 2, PublicationID: applied.PublicationID, Actor: scope.ActorID, At: time.Now()}}}
	reader := EffectiveProductReader{Reviews: reviewOwner, Snapshots: uploadCatalog{applied}}
	reviewOwner.record.Original = enrichment.Proposal{Changes: []enrichment.FieldChange{{Field: "title", Value: applied.Snapshot.Title}}}
	reader.Snapshots = effectiveVersions{original.Version: original, applied.Version: applied}
	result, err := reader.ReadEffectiveTargetProduct(ctx, selected, 2, id)
	require.NoError(t, err)
	require.Equal(t, applied, result)
	_, err = reader.ReadEffectiveTargetProduct(ctx, selected, 2, "")
	require.ErrorIs(t, err, record.ErrNotReady)
	reviewOwner.record.State = "accepted"
	reviewOwner.record.Receipt = nil
	_, err = reader.ReadEffectiveTargetProduct(ctx, selected, 2, id)
	require.ErrorIs(t, err, record.ErrNotReady)
	reviewOwner.record.State = "applied"
	reviewOwner.record.Receipt = &review.Receipt{ProposalID: id, Revision: 2, ProductVersion: 2, PublicationID: applied.PublicationID, Actor: scope.ActorID, At: time.Now()}
	changed := applied
	changed.Snapshot.Description = "Unreviewed changed field"
	reader.Snapshots = effectiveVersions{original.Version: original, changed.Version: changed}
	_, err = reader.ReadEffectiveTargetProduct(ctx, selected, 2, id)
	require.ErrorIs(t, err, record.ErrNotReady)
	reader.Snapshots = effectiveVersions{original.Version: original, applied.Version: applied}
	reviewOwner.record.Owner = "other-actor"
	_, err = reader.ReadEffectiveTargetProduct(ctx, selected, 2, id)
	require.ErrorIs(t, err, record.ErrNotReady)
	auth.denied = true
	_, err = reader.ReadEffectiveTargetProduct(ctx, selected, original.Version, "")
	require.Error(t, err)
}

type effectiveVersions map[uint64]catalog.PublishedSnapshot

func (v effectiveVersions) GetSnapshot(_ context.Context, id catalog.SnapshotIdentity, version uint64) (catalog.PublishedSnapshot, error) {
	p, ok := v[version]
	if !ok || p.Identity != id {
		return catalog.PublishedSnapshot{}, catalog.ErrSnapshotNotReady
	}
	return p, nil
}
