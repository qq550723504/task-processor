package target

import (
	"context"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"task-processor/internal/authidentity"
	"task-processor/internal/listing/preparation"
	"task-processor/internal/marketplace/shein/goods"
	model "task-processor/internal/marketplace/shein/model"
	"task-processor/internal/product/asset"
	"task-processor/internal/product/catalog"
	"task-processor/internal/product/collection"
	"task-processor/internal/product/sourcing"
	"task-processor/internal/storecenter"
	"testing"
	"time"
)

type targetAuth struct{ scope collection.Scope }

func (a targetAuth) Authorize(context.Context, string) (collection.Scope, error) { return a.scope, nil }

type targetCollectionRepo struct{ collection.Repository }
type targetUnusedSources struct {
	sourcing.PublishedAcquisitionReader
}
type targetPrepRepo struct {
	preparation.Repository
	source preparation.SourceItem
}

func (r targetPrepRepo) ReadRetainedSource(_ context.Context, scope collection.Scope, id string) (preparation.SourceItem, error) {
	if scope.ActorID != "actor-a" || id != r.source.ID {
		return preparation.SourceItem{}, preparation.ErrNotFound
	}
	return r.source, nil
}

type targetCatalog struct{ snapshot catalog.PublishedSnapshot }

func (r targetCatalog) GetSnapshot(context.Context, catalog.SnapshotIdentity, uint64) (catalog.PublishedSnapshot, error) {
	return r.snapshot, nil
}

type targetAssets struct{}

func (targetAssets) GetApprovedInventory(context.Context, asset.InventoryScope) (asset.ApprovedAssetInventory, error) {
	return asset.ApprovedAssetInventory{}, asset.ErrApprovedAssetsNotReady
}

type targetRules struct{}

func (targetRules) ReadTargetRules(_ context.Context, scope collection.Scope, storeID string, input goods.OfficialDraftInput) (storecenter.ProductMerchantBinding, goods.OfficialRuleSnapshot, error) {
	return storecenter.ProductMerchantBinding{OrganizationID: scope.OrganizationID, StoreID: storeID, Site: "shein-us", StoreVersion: 1, ConnectionRevision: 1, ApplicationRevision: "v1:self_operated", ApplicationID: "app-a", ApplicationType: storecenter.ApplicationSelfOperated, SupplierIdentityHash: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", ServiceExpiresAt: time.Now().Add(time.Hour)}, goods.OfficialRuleSnapshot{ApplicationMode: model.ModeSelfOperated}, nil
}

type targetRecords struct {
	TargetRepository
	values map[string]TargetReceipt
	hashes map[string]string
	writes int
}

func (r *targetRecords) FindTargetCommand(_ context.Context, _ collection.Scope, key, hash string) (TargetReceipt, error) {
	value, ok := r.values[key]
	if !ok {
		return TargetReceipt{}, ErrNotFound
	}
	if r.hashes[key] != hash {
		return TargetReceipt{}, ErrConflict
	}
	value.Replayed = true
	return value, nil
}
func (r *targetRecords) SaveTarget(ctx context.Context, proof TargetPrepared) (TargetReceipt, error) {
	_, key, hash, value, err := proof.Read(ctx)
	if err != nil {
		return TargetReceipt{}, err
	}
	r.writes++
	receipt := TargetReceipt{Record: value}
	r.values[key] = receipt
	r.hashes[key] = hash
	return receipt, nil
}
func TestTargetRecordUsesRetainedSourceAndPreservesIncompleteFactsForUserCompletion(t *testing.T) {
	scope := collection.Scope{"org-a", "actor-a", "member-a"}
	authority := targetAuth{scope}
	collections, err := collection.NewService(targetCollectionRepo{}, authority, targetUnusedSources{})
	require.NoError(t, err)
	source := preparation.SourceItem{ID: uuid.NewString(), PreparationID: uuid.NewString(), CollectionItemID: uuid.NewString(), CollectionRevision: 1, Source: collection.Source{ProductKey: "product-a", PublicationID: uuid.NewString(), Version: 1, Kind: "own"}}
	repo := targetPrepRepo{source: source}
	preparations, err := preparation.NewService(repo, collections, authority)
	require.NoError(t, err)
	snapshot := catalog.PublishedSnapshot{Identity: catalog.SnapshotIdentity{TenantID: scope.OrganizationID, ProductKey: source.Source.ProductKey}, PublicationID: source.Source.PublicationID, Version: 1, Snapshot: catalog.ProductSnapshot{Title: "Original"}}
	selections, err := preparation.NewSourceSelector(preparations, collections, repo, targetCatalog{snapshot})
	require.NoError(t, err)
	records := &targetRecords{values: map[string]TargetReceipt{}, hashes: map[string]string{}}
	service, err := NewTargetService(TargetDependencies{Sources: selections, Products: OriginalTargetProduct{}, Assets: targetAssets{}, Rules: targetRules{}, Records: records, Authorizer: authority})
	require.NoError(t, err)
	ctx := authidentity.WithAuthenticatedIdentity(context.Background(), authidentity.AuthenticatedIdentity{TenantID: scope.OrganizationID, EffectiveOrganizationID: scope.OrganizationID, UserID: scope.ActorID, EffectiveMemberID: scope.MemberID, TokenExpiresAt: time.Now().Add(time.Minute)})
	key := uuid.NewString()
	input := TargetInput{SourceID: source.ID, StoreID: uuid.NewString(), EffectiveVersion: 1, Draft: goods.OfficialDraftInput{Product: model.PublishProduct{Names: []model.LanguageContent{{Language: "en", Name: "Manual title"}}}}}
	receipt, err := service.Create(ctx, key, input)
	require.NoError(t, err)
	require.Equal(t, source, receipt.Record.Source)
	require.NotEmpty(t, receipt.Record.Result.Issues)
	require.False(t, receipt.Record.Result.ReadyForUpload)
	require.Empty(t, receipt.Record.Result.SubmissionPayload)
	require.Equal(t, 1, records.writes)
	require.Len(t, receipt.Record.ProductHash, 64)
	replay, err := service.Create(ctx, key, input)
	require.NoError(t, err)
	require.True(t, replay.Replayed)
	require.Equal(t, receipt.Record, replay.Record)
	require.Equal(t, 1, records.writes)
	input.Draft.Product.Names[0].Name = "Changed after receipt"
	require.Equal(t, "Manual title", receipt.Record.Input.Draft.Product.Names[0].Name)
	_, err = service.Create(ctx, key, input)
	require.ErrorIs(t, err, ErrConflict)
	input.SourceID = uuid.NewString()
	_, err = service.Create(ctx, uuid.NewString(), input)
	require.ErrorIs(t, err, preparation.ErrNotFound)
	_, _, _, _, err = (TargetPrepared{}).Read(ctx)
	require.ErrorIs(t, err, ErrForbidden)
}
