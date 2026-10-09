package collection

import (
	"context"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"task-processor/internal/authidentity"
	"task-processor/internal/product/catalog"
	"testing"
	"time"
)

type selectionStore struct {
	Repository
	item Item
}

func (s selectionStore) ReadBatch(_ context.Context, scope Scope, id string) (Batch, error) {
	if scope.ActorID != "actor-a" || id != s.item.BatchID {
		return Batch{}, ErrNotFound
	}
	return Batch{ID: id, Revision: 3}, nil
}

func (s selectionStore) ReadItem(_ context.Context, scope Scope, id string) (Item, error) {
	if scope.ActorID != "actor-a" || id != s.item.ID {
		return Item{}, ErrNotFound
	}
	return s.item, nil
}

func TestBatchSelectionBindsPrivateScopeRevisionAndImmutableItemChoices(t *testing.T) {
	scope := Scope{"org-a", "actor-a", "member-a"}
	item := Item{ID: uuid.NewString(), BatchID: uuid.NewString()}
	service, err := NewService(selectionStore{item: item}, &testAuth{scope: scope}, testSources{})
	require.NoError(t, err)
	ctx := authidentity.WithAuthenticatedIdentity(context.Background(), authidentity.AuthenticatedIdentity{TenantID: scope.OrganizationID, EffectiveOrganizationID: scope.OrganizationID, UserID: scope.ActorID, EffectiveMemberID: scope.MemberID, TokenExpiresAt: time.Now().Add(time.Minute)})
	input := BatchSelectionInput{BatchID: item.BatchID, ExpectedRevision: 3, ItemIDs: []string{item.ID}}
	proof, err := service.SelectBatch(ctx, input)
	require.NoError(t, err)
	input.ItemIDs[0] = uuid.NewString()
	gotScope, gotInput, err := proof.Read(ctx)
	require.NoError(t, err)
	require.Equal(t, scope, gotScope)
	require.Equal(t, item.ID, gotInput.ItemIDs[0])
	gotInput.ItemIDs[0] = uuid.NewString()
	_, second, err := proof.Read(ctx)
	require.NoError(t, err)
	require.Equal(t, item.ID, second.ItemIDs[0])
	wrong := authidentity.WithAuthenticatedIdentity(ctx, authidentity.AuthenticatedIdentity{TenantID: scope.OrganizationID, EffectiveOrganizationID: scope.OrganizationID, UserID: "other", EffectiveMemberID: scope.MemberID, TokenExpiresAt: time.Now().Add(time.Minute)})
	_, _, err = proof.Read(wrong)
	require.ErrorIs(t, err, ErrForbidden)
	_, _, err = (AuthorizedBatchSelection{}).Read(ctx)
	require.ErrorIs(t, err, ErrForbidden)
	input.ExpectedRevision = 2
	_, err = service.SelectBatch(ctx, input)
	require.ErrorIs(t, err, ErrConflict)
	input.ExpectedRevision = 3
	input.ItemIDs = []string{item.ID, item.ID}
	_, err = service.SelectBatch(ctx, input)
	require.ErrorIs(t, err, ErrInvalid)
}

type snapshotFixture struct {
	snapshot catalog.PublishedSnapshot
	calls    int
}

func (r *snapshotFixture) GetSnapshot(_ context.Context, id catalog.SnapshotIdentity, version uint64) (catalog.PublishedSnapshot, error) {
	r.calls++
	return r.snapshot, nil
}

func TestSelectionIsPrivateOriginalAndCannotBeReplacedByClientProductIdentity(t *testing.T) {
	scope := Scope{"org-a", "actor-a", "member-a"}
	item := Item{ID: uuid.NewString(), Revision: 3, Source: Source{ProductKey: "product-a", PublicationID: "publication-a", Version: 1, Kind: "own"}}
	reader := &snapshotFixture{snapshot: catalog.PublishedSnapshot{Identity: catalog.SnapshotIdentity{TenantID: scope.OrganizationID, ProductKey: item.Source.ProductKey}, PublicationID: item.Source.PublicationID, Version: 1, Snapshot: catalog.ProductSnapshot{Title: "原始", Images: []catalog.Image{{URL: "https://images.example.org/source.png"}}}}}
	authority := &testAuth{scope: scope}
	service, err := NewService(selectionStore{item: item}, authority, testSources{})
	require.NoError(t, err)
	service.WithSnapshots(reader)
	ctx := authidentity.WithAuthenticatedIdentity(context.Background(), authidentity.AuthenticatedIdentity{TenantID: scope.OrganizationID, EffectiveOrganizationID: scope.OrganizationID, UserID: scope.ActorID, EffectiveMemberID: scope.MemberID, TokenExpiresAt: time.Now().Add(time.Minute)})
	input := SelectionInput{ItemID: item.ID, ExpectedRevision: 3, OriginalPublicationID: item.Source.PublicationID, OriginalVersion: 1}
	selected, err := service.Select(ctx, input)
	require.NoError(t, err)
	detail, err := selected.Read(ctx)
	require.NoError(t, err)
	require.Equal(t, item.Source, detail.Item.Source)
	require.Equal(t, "原始", detail.Product.Title)
	detail.Product.Images[0].URL = "https://attacker.example/replaced.png"
	untouched, err := selected.Read(ctx)
	require.NoError(t, err)
	require.Equal(t, "https://images.example.org/source.png", untouched.Product.Images[0].URL)
	input.ExpectedRevision = 2
	_, err = service.Select(ctx, input)
	require.ErrorIs(t, err, ErrConflict)
	input.ExpectedRevision = 3
	input.OriginalVersion = 2
	_, err = service.Select(ctx, input)
	require.ErrorIs(t, err, ErrConflict)
	authority.scope.ActorID = "other-actor"
	_, err = service.Select(ctx, input)
	require.ErrorIs(t, err, ErrForbidden)
	_, err = (AuthorizedSelection{}).Read(ctx)
	require.ErrorIs(t, err, ErrForbidden)
	_, err = (AuthorizedSelection{}).Read(nil)
	require.ErrorIs(t, err, ErrForbidden)
}
