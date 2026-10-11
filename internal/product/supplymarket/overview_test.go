package supplymarket

import (
	"context"
	"fmt"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"task-processor/internal/product/catalog"
	"task-processor/internal/product/collection"
	"testing"
	"time"
)

type overviewStore struct {
	collection.Repository
	items map[string]collection.Item
}

func (r overviewStore) ReadItem(_ context.Context, scope collection.Scope, id string) (collection.Item, error) {
	if scope.ActorID != "actor-a" {
		return collection.Item{}, collection.ErrForbidden
	}
	item, ok := r.items[id]
	if !ok {
		return item, collection.ErrNotFound
	}
	return item, nil
}

type overviewCollection struct {
	*collection.Service
	pages      []collection.Page[collection.Item]
	afterBatch func()
}

func (r *overviewCollection) ListItems(_ context.Context, batch string, q collection.Query) (collection.Page[collection.Item], error) {
	if batch != "" {
		return collection.Page[collection.Item]{}, collection.ErrInvalid
	}
	for i, p := range r.pages {
		if i == 0 && q.After == "" || i > 0 && q.After == r.pages[i-1].NextCursor {
			return p, nil
		}
	}
	return collection.Page[collection.Item]{}, collection.ErrUnavailable
}
func (r *overviewCollection) ReadBatch(context.Context, string) (collection.Batch, error) {
	if r.afterBatch != nil {
		r.afterBatch()
	}
	return collection.Batch{Name: "真实分组"}, nil
}

type overviewEffective struct {
	effectiveFixture
	optimized map[string]bool
	fail      string
	delay     map[string]time.Duration
}

func (r overviewEffective) ReadChoice(ctx context.Context, _ collection.Scope, item collection.ItemDetail) (ProductChoice, error) {
	if d := r.delay[item.Item.ID]; d > 0 {
		select {
		case <-time.After(d):
		case <-ctx.Done():
			return ProductChoice{}, ctx.Err()
		}
	}
	if item.Item.ID == r.fail {
		return ProductChoice{}, ErrUnavailable
	}
	return ProductChoice{Selection: SelectionInput{ItemID: item.Item.ID, ExpectedRevision: item.Item.Revision}, Product: PublicProduct{Title: item.Item.ID, Images: []string{"https://images.example.org/real.png"}}, Optimized: r.optimized[item.Item.ID]}, nil
}

type overviewRepository struct {
	Repository
	counts ApplicationCounts
	before func()
	scope  collection.Scope
}

func (r *overviewRepository) CountApplications(_ context.Context, scope collection.Scope) (ApplicationCounts, error) {
	r.scope = scope
	if r.before != nil {
		r.before()
	}
	return r.counts, nil
}
func overviewFixture(t *testing.T, n int) (*Service, *testAuthority, *overviewCollection, *overviewRepository, context.Context, []collection.Item) {
	t.Helper()
	_, a, _, _, ctx, _ := serviceFixture(t)
	items := make([]collection.Item, n)
	store := overviewStore{items: map[string]collection.Item{}}
	eff := overviewEffective{optimized: map[string]bool{}, delay: map[string]time.Duration{}}
	for i := range items {
		items[i] = collection.Item{ID: uuid.NewString(), BatchID: uuid.NewString(), Revision: 1, Source: collection.Source{Kind: "own", ProductKey: "own-product", PublicationID: "own-publication", Version: 1}}
		store.items[items[i].ID] = items[i]
		eff.optimized[items[i].ID] = true
	}
	c, e := collection.NewService(store, a, unusedAcquisition{})
	require.NoError(t, e)
	c.WithSnapshots(snapshots{catalog.PublishedSnapshot{Identity: catalog.SnapshotIdentity{TenantID: "org-a", ProductKey: "own-product"}, PublicationID: "own-publication", Version: 1, Snapshot: catalog.ProductSnapshot{Title: "原始商品", Images: []catalog.Image{{URL: "https://images.example.org/original.png"}}}}})
	selected := &overviewCollection{Service: c}
	if n == 0 {
		selected.pages = []collection.Page[collection.Item]{{Items: []collection.Item{}, Total: 0}}
	}
	for start := 0; start < n; start += 50 {
		end := min(start+50, n)
		p := collection.Page[collection.Item]{Items: items[start:end], Total: int64(n)}
		if end < n {
			p.NextCursor = items[end-1].ID
		}
		selected.pages = append(selected.pages, p)
	}
	repo := &overviewRepository{counts: ApplicationCounts{Reviewing: 27, SupplementRequired: 4, Approved: 7}}
	s, e := NewService(a, selected, eff, repo)
	require.NoError(t, e)
	return s, a, selected, repo, ctx, items
}
func TestApplicationOverviewCountsAllPagesAndKeepsOrderedPreview(t *testing.T) {
	s, _, _, repo, ctx, items := overviewFixture(t, 55)
	eff := s.effective.(overviewEffective)
	eff.optimized[items[0].ID] = false
	eff.delay[items[1].ID] = 20 * time.Millisecond
	eff.delay[items[2].ID] = time.Millisecond
	s.effective = eff
	got, err := s.ApplicationOverview(ctx)
	require.NoError(t, err)
	require.EqualValues(t, 54, got.EligibleProducts)
	require.Equal(t, repo.counts, got.ApplicationCounts)
	require.Equal(t, collection.Scope{"org-a", "actor-a", "member-a"}, repo.scope)
	require.Len(t, got.Products, 3)
	for i, p := range got.Products {
		require.Equal(t, items[i+1].ID, p.ItemID)
		require.Equal(t, "真实分组", p.GroupName)
		require.Equal(t, "https://images.example.org/real.png", p.ThumbnailURL)
		require.Equal(t, "own", p.Source)
	}
}
func TestApplicationOverviewNeverEmitsPartialOrRevokedFacts(t *testing.T) {
	for _, kind := range []string{"choice-failure", "over-budget", "duplicate", "repeated-cursor", "total-drift", "revision-drift", "member-drift", "revoked", "cancelled"} {
		t.Run(kind, func(t *testing.T) {
			s, a, c, r, ctx, items := overviewFixture(t, 2)
			switch kind {
			case "choice-failure":
				eff := s.effective.(overviewEffective)
				eff.fail = items[1].ID
				s.effective = eff
			case "over-budget":
				c.pages[0].Total = 2001
			case "duplicate":
				c.pages[0].Items[1] = items[0]
			case "repeated-cursor":
				c.pages[0].NextCursor = items[1].ID
				c.pages = append(c.pages, c.pages[0])
			case "revoked":
				c.afterBatch = func() { a.revoked = true }
			case "member-drift":
				c.afterBatch = func() { a.scope.MemberID = "member-b" }
			case "revision-drift":
				c.pages[0].Items[0].Revision++
			case "total-drift":
				c.pages[0].NextCursor = items[1].ID
				c.pages = append(c.pages, collection.Page[collection.Item]{Total: 3})
			case "cancelled":
				cancelled, cancel := context.WithCancel(ctx)
				cancel()
				ctx = cancelled
			}
			got, err := s.ApplicationOverview(ctx)
			require.Error(t, err)
			require.Zero(t, got.EligibleProducts)
			require.Empty(t, got.Products)
			require.Zero(t, got.Reviewing)
			_ = r
		})
	}
}
func TestApplicationOverviewBudgetBoundaryAndTrueEmpty(t *testing.T) {
	for _, n := range []int{0, 2000} {
		t.Run(fmt.Sprint(n), func(t *testing.T) {
			s, _, _, _, ctx, _ := overviewFixture(t, n)
			got, err := s.ApplicationOverview(ctx)
			require.NoError(t, err)
			require.EqualValues(t, n, got.EligibleProducts)
			require.Len(t, got.Products, min(n, 3))
		})
	}
}
