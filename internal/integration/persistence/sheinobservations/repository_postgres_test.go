//go:build integration

package sheinobservations

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	pg "github.com/testcontainers/testcontainers-go/modules/postgres"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	"net/url"
	"strings"
	o "task-processor/internal/marketplace/shein/observations"
	"testing"
	"time"
)

func observationDatabase(t *testing.T) (context.Context, *gorm.DB, *Repository) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	t.Cleanup(cancel)
	c, e := pg.Run(ctx, "postgres:16-alpine", pg.WithDatabase("store_observations_test"), pg.WithUsername("installer"), pg.WithPassword(uuid.NewString()), pg.BasicWaitStrategies())
	require.NoError(t, e)
	t.Cleanup(func() { _ = c.Terminate(context.Background()) })
	dsn, e := c.ConnectionString(ctx, "sslmode=disable")
	require.NoError(t, e)
	db, e := gorm.Open(postgres.Open(dsn), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	require.NoError(t, e)
	pool, _ := db.DB()
	t.Cleanup(func() { _ = pool.Close() })
	require.NoError(t, Install(ctx, db))
	password := uuid.NewString()
	require.NoError(t, db.Exec("CREATE ROLE observation_runtime LOGIN PASSWORD '"+password+"'; REVOKE CREATE,TEMP ON DATABASE store_observations_test FROM PUBLIC; REVOKE ALL ON SCHEMA public FROM PUBLIC; GRANT USAGE ON SCHEMA public TO observation_runtime; GRANT CONNECT ON DATABASE store_observations_test TO observation_runtime;").Error)
	require.NoError(t, GrantRuntime(ctx, db, "observation_runtime"))
	u, _ := url.Parse(dsn)
	u.User = url.UserPassword("observation_runtime", password)
	runtime, e := gorm.Open(postgres.Open(u.String()), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	require.NoError(t, e)
	runtimePool, _ := runtime.DB()
	t.Cleanup(func() { _ = runtimePool.Close() })
	r, e := NewRepository(ctx, runtime)
	require.NoError(t, e)
	return ctx, db, r
}
func syncInput(scope o.Scope, store string) (o.BeginInput, []o.Sync) {
	return o.BeginInput{Kind: o.Products, Stores: []string{store}}, []o.Sync{{ID: uuid.NewString(), StoreID: store, Owner: scope, Kind: o.Products, Key: uuid.NewString(), Status: "pending", Revision: 1, CreatedAt: time.Now().UTC(), Binding: o.Binding{OrganizationID: scope.OrganizationID, StoreID: store, ApplicationID: "app-a"}, Progress: o.Checkpoint{Page: 1, Windows: []o.Window{}, Notes: []string{}}}}
}
func TestObservationPostgresAtomicReplayCheckpointAndHeadOrdering(t *testing.T) {
	ctx, _, r := observationDatabase(t)
	scope := o.Scope{"org-a", "actor-a", "member-a"}
	store := uuid.NewString()
	key := uuid.NewString()
	in, children := syncInput(scope, store)
	first, e := r.Begin(ctx, scope, key, "hash-a", in, children)
	require.NoError(t, e)
	require.Len(t, first.Syncs, 1)
	replay, e := r.Begin(ctx, scope, key, "hash-a", in, children)
	require.NoError(t, e)
	require.Equal(t, first.ID, replay.ID)
	_, e = r.Begin(ctx, scope, key, "hash-b", in, children)
	require.ErrorIs(t, e, o.ErrConflict)
	different := scope
	different.MemberID = "replacement-member"
	_, e = r.CommandByKey(ctx, different, key)
	require.ErrorIs(t, e, o.ErrNotFound)
	in, children = syncInput(scope, store)
	second, e := r.Begin(ctx, scope, uuid.NewString(), "hash-second", in, children)
	require.NoError(t, e)
	for _, s := range []o.Sync{second.Syncs[0], first.Syncs[0]} {
		p := o.Product{ID: "spu-a", SKCs: []o.SKC{{ID: "skc-a", Title: "Product", Site: "shein-us", SKUs: []o.SKU{}}}}
		c := s.Progress
		_, e = r.CommitPage(ctx, s, c, []o.Record{{ID: p.ID, StoreID: store, SyncID: s.ID, Product: &p, ObservedAt: time.Now().UTC()}}, "completed", time.Now().UTC())
		require.NoError(t, e)
	}
	heads, _, e := r.Heads(ctx, scope.OrganizationID, o.Products, []string{store})
	require.NoError(t, e)
	require.Len(t, heads, 1)
	require.Equal(t, second.Syncs[0].ID, heads[0].ID)
	_, e = r.Record(ctx, "org-b", store, o.Products, first.Syncs[0].ID, "spu-a")
	require.ErrorIs(t, e, o.ErrNotFound)
	stale := first.Syncs[0]
	_, e = r.CommitPage(ctx, stale, stale.Progress, nil, "running", time.Now().UTC())
	require.ErrorIs(t, e, o.ErrConflict)
}

func TestObservationPostgresPartialHeadAndScopedPagination(t *testing.T) {
	ctx, _, r := observationDatabase(t)
	scope := o.Scope{"org-a", "actor-a", "member-a"}
	store := uuid.NewString()
	in, children := syncInput(scope, store)
	cmd, e := r.Begin(ctx, scope, uuid.NewString(), "a", in, children)
	require.NoError(t, e)
	active, off := 1, 0
	product := o.Product{ID: "spu-a", SKCs: []o.SKC{{ID: "skc-a", Title: "First", Site: "shein-us", SiteStatus: &active, SKUs: []o.SKU{}}, {ID: "skc-b", Title: "Second", Site: "shein-us", SiteStatus: &off, SKUs: []o.SKU{}}}}
	sync := cmd.Syncs[0]
	complete, e := r.CommitPage(ctx, sync, sync.Progress, []o.Record{{ID: product.ID, StoreID: store, SyncID: sync.ID, Product: &product}}, "completed", time.Now().UTC())
	require.NoError(t, e)
	in, children = syncInput(scope, store)
	partial, e := r.Begin(ctx, scope, uuid.NewString(), "b", in, children)
	require.NoError(t, e)
	s := partial.Syncs[0]
	c := s.Progress
	c.Note("missing_site")
	terminal, e := r.CommitPage(ctx, s, c, nil, "completed", time.Now().UTC())
	require.NoError(t, e)
	require.Equal(t, "partial", terminal.Status)
	heads, latest, e := r.Heads(ctx, scope.OrganizationID, o.Products, []string{store})
	require.NoError(t, e)
	require.Equal(t, complete.ID, heads[0].ID)
	require.Equal(t, terminal.ID, latest[0].ID)
	q := o.Query{Kind: o.Products, Limit: 1, Sources: map[string]string{store: complete.ID}}
	page, e := r.List(ctx, scope.OrganizationID, q)
	require.NoError(t, e)
	require.Equal(t, 2, page.Summary.Total)
	require.Equal(t, 1, page.Summary.Active)
	require.Equal(t, 1, page.Summary.OffShelf)
	require.Len(t, page.Items, 1)
	require.NotEmpty(t, page.Next)
	q.After = page.Next
	next, e := r.List(ctx, scope.OrganizationID, q)
	require.NoError(t, e)
	require.Len(t, next.Items, 1)
	require.Equal(t, "skc-b", next.Items[0].Product.SKCs[0].ID)
	q.Status = "active"
	_, e = r.List(ctx, scope.OrganizationID, q)
	require.ErrorIs(t, e, o.ErrConflict)
	_, e = r.List(ctx, "org-b", q)
	require.ErrorIs(t, e, o.ErrConflict)
	q.After = ""
	q.Status = ""
	q.Sources = map[string]string{store: terminal.ID}
	empty, e := r.List(ctx, scope.OrganizationID, q)
	require.NoError(t, e)
	require.Empty(t, empty.Items)
	// A invalid page rolls back both observation rows and its checkpoint.
	in, children = syncInput(scope, store)
	bad, e := r.Begin(ctx, scope, uuid.NewString(), "c", in, children)
	require.NoError(t, e)
	s = bad.Syncs[0]
	_, e = r.CommitPage(ctx, s, s.Progress, []o.Record{{ID: product.ID, StoreID: store, SyncID: s.ID, Product: &product}, {ID: "wrong", StoreID: uuid.NewString(), SyncID: s.ID, Product: &product}}, "running", time.Now().UTC())
	require.Error(t, e)
	unchanged, e := r.ReadSync(ctx, scope.OrganizationID, s.ID)
	require.NoError(t, e)
	require.Equal(t, int64(1), unchanged.Revision)
	_, e = r.Record(ctx, scope.OrganizationID, store, o.Products, s.ID, product.ID)
	require.ErrorIs(t, e, o.ErrNotFound)
}
func TestObservationPostgresMissingOrderTimeDoesNotClaimKnownZero(t *testing.T) {
	ctx, _, r := observationDatabase(t)
	scope := o.Scope{"org-a", "actor-a", "member-a"}
	store := uuid.NewString()
	in, children := syncInput(scope, store)
	in.Kind = o.Orders
	children[0].Kind = o.Orders
	cmd, e := r.Begin(ctx, scope, uuid.NewString(), "orders", in, children)
	require.NoError(t, e)
	s := cmd.Syncs[0]
	order := o.Order{ID: "order-a", Site: "shein-us", Reasons: []int{}, Items: []o.OrderItem{}, Packages: []o.Package{}}
	_, e = r.CommitPage(ctx, s, s.Progress, []o.Record{{ID: order.ID, StoreID: store, SyncID: s.ID, Order: &order}}, "completed", time.Now().UTC())
	require.NoError(t, e)
	out, e := r.List(ctx, scope.OrganizationID, o.Query{Kind: o.Orders, Limit: 20, Sources: map[string]string{store: s.ID}, TodayStart: time.Now().UTC().Truncate(24 * time.Hour)})
	require.NoError(t, e)
	require.Equal(t, 1, out.Summary.TodayUnknown)
}
func TestObservationPostgresLargeRecordsPaginateWithinResponseBudget(t *testing.T) {
	ctx, _, r := observationDatabase(t)
	scope := o.Scope{"org-a", "actor-a", "member-a"}
	store := uuid.NewString()
	in, children := syncInput(scope, store)
	cmd, e := r.Begin(ctx, scope, uuid.NewString(), "large", in, children)
	require.NoError(t, e)
	s := cmd.Syncs[0]
	records := []o.Record{}
	for i := 0; i < 10; i++ {
		p := o.Product{ID: fmt.Sprintf("spu-%02d", i), SKCs: []o.SKC{{ID: "skc-a", Site: "shein-us", SKUs: []o.SKU{}}}}
		for j := 0; j < 200; j++ {
			p.SKCs[0].SKUs = append(p.SKCs[0].SKUs, o.SKU{ID: fmt.Sprintf("sku-%03d", j), SellerSKU: strings.Repeat("a", 800), Prices: []o.Price{}, Costs: []o.Price{}, Inventory: []o.Inventory{}})
		}
		records = append(records, o.Record{ID: p.ID, StoreID: store, SyncID: s.ID, Product: &p})
	}
	_, e = r.CommitPage(ctx, s, s.Progress, records, "completed", time.Now().UTC())
	require.NoError(t, e)
	q := o.Query{Kind: o.Products, Limit: 20, Sources: map[string]string{store: s.ID}}
	seen := map[string]bool{}
	for {
		page, e := r.List(ctx, scope.OrganizationID, q)
		require.NoError(t, e)
		payload, e := json.Marshal(page.Items)
		require.NoError(t, e)
		require.LessOrEqual(t, len(payload), 1<<20)
		require.Equal(t, 10, page.Summary.Total)
		for _, item := range page.Items {
			require.False(t, seen[item.ID])
			seen[item.ID] = true
		}
		if page.Next == "" {
			break
		}
		q.After = page.Next
	}
	require.Len(t, seen, 10)
}
