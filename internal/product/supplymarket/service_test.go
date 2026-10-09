package supplymarket

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"task-processor/internal/authidentity"
	"task-processor/internal/product/catalog"
	"task-processor/internal/product/collection"
	"task-processor/internal/product/sourcing"
)

type testAuthority struct {
	scope    collection.Scope
	revoked  bool
	platform string
}

func (a *testAuthority) Authorize(context.Context, string) (collection.Scope, error) {
	if a.revoked {
		return collection.Scope{}, ErrForbidden
	}
	return a.scope, nil
}
func (a *testAuthority) AuthorizePlatform(context.Context) (string, error) {
	if a.revoked || a.platform == "" {
		return "", ErrForbidden
	}
	return a.platform, nil
}

type serviceRepository struct {
	Repository
	before   func()
	commands []Command
	selected *SelectedProduct
}

func (r *serviceRepository) Execute(ctx context.Context, c Command, g Guard) (Receipt, error) {
	if r.before != nil {
		r.before()
	}
	if c.Select != nil {
		selected, err := c.Select(ctx)
		if err != nil {
			return Receipt{}, err
		}
		r.selected = &selected
	}
	if err := g(ctx); err != nil {
		return Receipt{}, err
	}
	r.commands = append(r.commands, c)
	return Receipt{OperationID: c.OperationID}, nil
}

type selectionRepository struct {
	collection.Repository
	item collection.Item
}

func (r *serviceRepository) ListReleases(context.Context, Query) (Page[Release], error) {
	if r.before != nil {
		r.before()
	}
	return Page[Release]{Items: []Release{{Product: PublicProduct{Title: "已发布商品"}}}}, nil
}
func TestMarketReadDoesNotEmitResultsAfterGrantRevocation(t *testing.T) {
	s, a, _, r, ctx, _ := serviceFixture(t)
	r.before = func() { a.revoked = true }
	result, err := s.ListMarket(ctx, Query{Limit: 10})
	require.ErrorIs(t, err, ErrForbidden)
	require.Empty(t, result.Items)
}

func (r *selectionRepository) ReadItem(_ context.Context, scope collection.Scope, id string) (collection.Item, error) {
	if scope.ActorID != "actor-a" || id != r.item.ID {
		return collection.Item{}, collection.ErrNotFound
	}
	return r.item, nil
}

type snapshots struct{ published catalog.PublishedSnapshot }

func (r snapshots) GetSnapshot(context.Context, catalog.SnapshotIdentity, uint64) (catalog.PublishedSnapshot, error) {
	return r.published, nil
}

type effectiveFixture struct{}
type unusedAcquisition struct {
	sourcing.PublishedAcquisitionReader
}

func (effectiveFixture) ReadEffective(_ context.Context, scope collection.Scope, item collection.ItemDetail, version uint64, _ string) (catalog.PublishedSnapshot, error) {
	return catalog.PublishedSnapshot{Identity: catalog.SnapshotIdentity{TenantID: scope.OrganizationID, ProductKey: item.Item.Source.ProductKey}, PublicationID: item.Item.Source.PublicationID, Version: version, Snapshot: item.Product}, nil
}
func serviceFixture(t *testing.T) (*Service, *testAuthority, *selectionRepository, *serviceRepository, context.Context, Mutation) {
	t.Helper()
	a := &testAuthority{scope: collection.Scope{OrganizationID: "org-a", ActorID: "actor-a", MemberID: "member-a"}}
	selected := &selectionRepository{item: collection.Item{ID: uuid.NewString(), Revision: 1, Source: collection.Source{Kind: "own", ProductKey: "own-product", PublicationID: "own-publication", Version: 1}}}
	c, err := collection.NewService(selected, a, unusedAcquisition{})
	require.NoError(t, err)
	c.WithSnapshots(snapshots{published: catalog.PublishedSnapshot{Identity: catalog.SnapshotIdentity{TenantID: a.scope.OrganizationID, ProductKey: selected.item.Source.ProductKey}, PublicationID: selected.item.Source.PublicationID, Version: 1, Snapshot: catalog.ProductSnapshot{Title: "本人商品", Images: []catalog.Image{{URL: "https://images.example.org/art.png"}}}}})
	r := &serviceRepository{}
	s, err := NewService(a, c, effectiveFixture{}, r)
	require.NoError(t, err)
	ctx := authidentity.WithAuthenticatedIdentity(context.Background(), authidentity.AuthenticatedIdentity{TenantID: a.scope.OrganizationID, EffectiveOrganizationID: a.scope.OrganizationID, UserID: a.scope.ActorID, EffectiveMemberID: a.scope.MemberID, TokenExpiresAt: time.Now().Add(time.Minute)})
	i := Mutation{Action: "create_official_draft", Selection: &SelectionInput{ItemID: selected.item.ID, ExpectedRevision: 1, OriginalPublicationID: "own-publication", OriginalVersion: 1, EffectiveVersion: 1}, Supply: &SupplyDeclaration{Stock: 10, MinimumQuantity: 1, Province: "浙江", City: "杭州"}, Disclosure: true}
	return s, a, selected, r, ctx, i
}
func TestDraftRejectsSourceRevisionChangeBeforeCommit(t *testing.T) {
	s, _, selected, r, ctx, i := serviceFixture(t)
	r.before = func() { selected.item.Revision++ }
	_, err := s.ExecuteMember(ctx, uuid.NewString(), i)
	require.ErrorIs(t, err, collection.ErrConflict)
	require.Empty(t, r.commands)
}
func TestDraftRejectsRevocationBeforeCommit(t *testing.T) {
	s, a, _, r, ctx, i := serviceFixture(t)
	r.before = func() { a.revoked = true }
	_, err := s.ExecuteMember(ctx, uuid.NewString(), i)
	require.ErrorIs(t, err, ErrForbidden)
	require.Empty(t, r.commands)
}
func TestPlatformCannotSelectPrivateProductOrPublishForeignDraftThroughBody(t *testing.T) {
	s, a, _, r, ctx, i := serviceFixture(t)
	a.platform = "platform-a"
	i.Action = "publish"
	i.ID = uuid.NewString()
	i.ExpectedRevision = 1
	_, err := s.ExecutePlatform(ctx, uuid.NewString(), i)
	require.ErrorIs(t, err, ErrInvalid)
	require.Empty(t, r.commands)
}
func TestDraftBindsServerSelectionAndPrincipal(t *testing.T) {
	s, a, selected, r, ctx, i := serviceFixture(t)
	key := uuid.NewString()
	got, err := s.ExecuteMember(ctx, key, i)
	require.NoError(t, err)
	require.Len(t, r.commands, 1)
	c := r.commands[0]
	require.Equal(t, a.scope, c.Scope)
	require.Empty(t, c.PlatformActor)
	require.Equal(t, selected.item.Source.PublicationID, r.selected.Source.PublicationID)
	require.Equal(t, "本人商品", r.selected.Product.Title)
	require.Equal(t, collection.StableID(a.scope.OrganizationID, a.scope.ActorID, key), got.OperationID)
}
func TestReadMarketRejectsAbsentContextWithoutPanic(t *testing.T) {
	s, _, _, _, _, _ := serviceFixture(t)
	require.NotPanics(t, func() { _, err := s.ListMarket(nil, Query{Limit: 10}); require.ErrorIs(t, err, ErrForbidden) })
	var absent *Service
	require.NotPanics(t, func() {
		_, err := absent.ReadMarket(context.Background(), uuid.NewString())
		require.ErrorIs(t, err, ErrForbidden)
	})
}
