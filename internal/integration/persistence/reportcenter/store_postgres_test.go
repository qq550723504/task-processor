//go:build integration

package reportcenterpersistence

import (
	"context"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	"net/url"
	"strings"
	"sync"
	rc "task-processor/internal/reportcenter"
	"testing"
	"time"
)

func TestPersonalReportPersistenceAndRuntimeBoundary(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	container, e := tcpostgres.Run(ctx, "postgres:16-alpine", tcpostgres.WithDatabase("reports"), tcpostgres.WithUsername("reports"), tcpostgres.WithPassword("reports"), tcpostgres.BasicWaitStrategies())
	require.NoError(t, e)
	t.Cleanup(func() { require.NoError(t, container.Terminate(context.Background())) })
	dsn, e := container.ConnectionString(ctx, "sslmode=disable")
	require.NoError(t, e)
	admin, e := gorm.Open(postgres.Open(dsn), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	require.NoError(t, e)
	raw, _ := admin.DB()
	t.Cleanup(func() { _ = raw.Close() })
	require.NoError(t, InstallSchema(admin))
	require.ErrorIs(t, VerifySchema(ctx, admin), rc.ErrUnavailable)
	for _, q := range []string{`REVOKE CREATE ON DATABASE reports FROM PUBLIC`, `REVOKE CREATE ON SCHEMA public FROM PUBLIC`, `CREATE ROLE report_runtime LOGIN PASSWORD 'test-only-report-runtime' NOSUPERUSER NOCREATEDB NOCREATEROLE NOBYPASSRLS`, `GRANT CONNECT ON DATABASE reports TO report_runtime`} {
		require.NoError(t, admin.Exec(q).Error)
	}
	require.NoError(t, GrantRuntime(admin, "report_runtime"))
	runtimeURL, e := url.Parse(dsn)
	require.NoError(t, e)
	runtimeURL.User = url.UserPassword("report_runtime", "test-only-report-runtime")
	runtime, e := gorm.Open(postgres.Open(runtimeURL.String()), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	require.NoError(t, e)
	runtimeRaw, _ := runtime.DB()
	t.Cleanup(func() { _ = runtimeRaw.Close() })
	require.NoError(t, VerifySchema(ctx, runtime))
	s, e := New(runtime)
	require.NoError(t, e)
	scope := rc.Scope{OrganizationID: "org-a", ActorID: "actor-a"}
	snap := rc.Snapshot{SourceInfo: rc.SourceInfo{Ref: rc.SourceRef{Kind: "TITLE_REVIEW", ID: uuid.NewString(), Version: "2:accepted"}, Title: "真实标题审核", ProductKey: "product-a"}, Content: rc.Document{SchemaVersion: 1, Sections: []rc.Section{{Title: "历史结果", Fields: []rc.Field{{Label: "标题", Value: "before -> after"}}}}}}
	key := uuid.NewString()
	fp := rc.Fingerprint("save", snap.Ref)
	allow := func(context.Context) error { return nil }
	first, e := s.Save(ctx, scope, key, fp, snap, allow)
	require.NoError(t, e)
	var wg sync.WaitGroup
	errs := make([]error, 8)
	ids := make([]string, 8)
	for i := range errs {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			got, err := s.Save(ctx, scope, uuid.NewString(), fp, snap, allow)
			errs[i] = err
			ids[i] = got.Report.ID
		}(i)
	}
	wg.Wait()
	for i := range errs {
		require.NoError(t, errs[i])
		require.Equal(t, first.Report.ID, ids[i])
	}
	// Separate admin connections exercise actual concurrent uniqueness, not pool serialization.
	concurrent, e := New(admin)
	require.NoError(t, e)
	for i := range errs {
		wg.Add(1)
		go func(i int) { defer wg.Done(); _, errs[i] = concurrent.Save(ctx, scope, key, fp, snap, allow) }(i)
	}
	wg.Wait()
	for _, err := range errs {
		require.NoError(t, err)
	}
	count := int64(0)
	require.NoError(t, admin.Table("report_center.saved_reports").Count(&count).Error)
	require.EqualValues(t, 1, count)
	different := snap
	different.Content = rc.Document{SchemaVersion: 1, Sections: []rc.Section{{Title: "历史结果", Fields: []rc.Field{{Label: "标题", Value: "different"}}}}}
	_, e = s.Save(ctx, scope, uuid.NewString(), fp, different, allow)
	require.ErrorIs(t, e, rc.ErrConflict)
	conflicting := snap
	conflicting.Ref.Version = "2:applied"
	_, e = s.Save(ctx, scope, key, rc.Fingerprint("save", conflicting.Ref), conflicting, allow)
	require.ErrorIs(t, e, rc.ErrConflict)
	_, e = s.Save(ctx, scope, uuid.NewString(), fp, snap, func(context.Context) error { return rc.ErrForbidden })
	require.ErrorIs(t, e, rc.ErrForbidden)
	for _, other := range []rc.Scope{{OrganizationID: "org-a", ActorID: "other"}, {OrganizationID: "org-b", ActorID: "actor-a"}} {
		_, e = s.Read(ctx, other, first.Report.ID)
		require.ErrorIs(t, e, rc.ErrNotFound)
		page, e := s.List(ctx, other, rc.Filter{View: "all", Limit: 20})
		require.NoError(t, e)
		require.Empty(t, page.Items)
	}
	old := uuid.NewString()
	_, e = s.Favorite(ctx, scope, old, rc.Fingerprint("favorite", true), first.Report.ID, true, allow)
	require.NoError(t, e)
	_, e = s.Favorite(ctx, scope, uuid.NewString(), rc.Fingerprint("favorite", false), first.Report.ID, false, allow)
	require.NoError(t, e)
	replay, e := s.Favorite(ctx, scope, old, rc.Fingerprint("favorite", true), first.Report.ID, true, allow)
	require.NoError(t, e)
	require.True(t, replay.Replayed)
	require.False(t, replay.Report.Favorite)
	second, e := New(runtime.Session(&gorm.Session{NewDB: true}))
	require.NoError(t, e)
	stored, e := second.Read(ctx, scope, first.Report.ID)
	require.NoError(t, e)
	require.Equal(t, first.Report.Content, stored.Content)
	summary, e := s.Summary(ctx, scope)
	require.NoError(t, e)
	require.EqualValues(t, 1, summary.Saved)
	require.Zero(t, summary.Favorites)
	page, e := s.List(ctx, scope, rc.Filter{View: "all", Limit: 1, Search: "真实"})
	require.NoError(t, e)
	require.Len(t, page.Items, 1)
	require.Equal(t, first.Report.ID, page.Items[0].ID)
	for _, q := range []string{`UPDATE report_center.saved_reports SET title='tampered'`, `DELETE FROM report_center.saved_reports`, `UPDATE report_center.commands SET fingerprint='` + strings.Repeat("a", 64) + `'`, `UPDATE report_center.favorites SET actor_id='other'`, `CREATE TABLE report_center.evil(id int)`} {
		require.Error(t, runtime.Exec(q).Error, q)
	}
	require.NoError(t, admin.Exec(`CREATE TABLE public.other_owner(id int); GRANT SELECT ON public.other_owner TO report_runtime`).Error)
	require.ErrorIs(t, VerifySchema(ctx, runtime), rc.ErrUnavailable)
}
