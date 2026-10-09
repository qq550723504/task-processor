package toolmarketpersistence

import (
	"context"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	"net/url"
	"os"
	"strings"
	"sync"
	tm "task-processor/internal/toolmarket"
	"testing"
)

func fixture(t *testing.T) (*gorm.DB, *Store) {
	t.Helper()
	dsn := os.Getenv("TOOLMARKET_TEST_DSN")
	if dsn == "" {
		t.Skip("requires isolated TOOLMARKET_TEST_DSN")
	}
	root, e := gorm.Open(postgres.Open(dsn), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	require.NoError(t, e)
	name := "toolmarket_" + strings.ReplaceAll(uuid.NewString(), "-", "")
	require.NoError(t, root.Exec("CREATE DATABASE "+name).Error)
	u, e := url.Parse(dsn)
	require.NoError(t, e)
	u.Path = "/" + name
	db, e := gorm.Open(postgres.Open(u.String()), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	require.NoError(t, e)
	t.Cleanup(func() {
		raw, _ := db.DB()
		_ = raw.Close()
		_ = root.Exec("DROP DATABASE " + name + " WITH (FORCE)").Error
		raw, _ = root.DB()
		_ = raw.Close()
	})
	require.NoError(t, InstallSchema(db))
	s, e := New(db)
	require.NoError(t, e)
	return db, s
}

var allow = func(context.Context) error { return nil }

func create() tm.Command {
	return tm.Command{Scope: tm.Scope{OrganizationID: "org-a", ActorID: "admin-a"}, Key: uuid.NewString(), Operation: "create", Absent: true, Demand: tm.Demand{Kind: "DATA", Title: "商品需求", Description: "保存商品资料"}}
}
func TestAtomicConcurrentReplayAndTenantIsolation(t *testing.T) {
	db, s := fixture(t)
	ctx := context.Background()
	c := create()
	var wg sync.WaitGroup
	receipts := make([]tm.Receipt, 8)
	errs := make([]error, 8)
	for i := range receipts {
		wg.Add(1)
		go func(i int) { defer wg.Done(); receipts[i], errs[i] = s.Execute(ctx, c, allow) }(i)
	}
	wg.Wait()
	for i := range receipts {
		require.NoError(t, errs[i])
		require.Equal(t, receipts[0], receipts[i])
	}
	var n int64
	require.NoError(t, db.Table("tool_market.requests").Count(&n).Error)
	require.EqualValues(t, 1, n)
	_, e := s.Detail(ctx, tm.Scope{ActorID: "other", OrganizationID: "org-b"}, false, receipts[0].ID)
	require.ErrorIs(t, e, tm.ErrNotFound)
	c.Demand.Title = "different"
	_, e = s.Execute(ctx, c, allow)
	require.ErrorIs(t, e, tm.ErrConflict)
	c.Demand.Title = "商品需求"
	_, e = s.Execute(ctx, c, func(context.Context) error { return tm.ErrForbidden })
	require.ErrorIs(t, e, tm.ErrForbidden)
	// Independent connection/store after a lost response must recover the same fact.
	raw, _ := db.DB()
	second, e := New(db.Session(&gorm.Session{NewDB: true}))
	require.NoError(t, e)
	require.NotNil(t, raw)
	replay, e := second.Execute(ctx, c, allow)
	require.NoError(t, e)
	require.Equal(t, receipts[0], replay)
}
func TestConcurrentProgressAndRollback(t *testing.T) {
	db, s := fixture(t)
	ctx := context.Background()
	r, e := s.Execute(ctx, create(), allow)
	require.NoError(t, e)
	p := tm.Command{Scope: tm.Scope{ActorID: "specialist"}, Platform: true, Key: uuid.NewString(), Operation: "progress", ID: r.ID, Expected: 1, Progress: tm.Progress{Stage: "EVALUATING", Note: "正在人工评估"}}
	var wg sync.WaitGroup
	errs := make([]error, 2)
	for i := range errs {
		wg.Add(1)
		go func(i int) { defer wg.Done(); c := p; c.Key = uuid.NewString(); _, errs[i] = s.Execute(ctx, c, allow) }(i)
	}
	wg.Wait()
	if errs[0] == nil {
		require.ErrorIs(t, errs[1], tm.ErrRevision)
	} else {
		require.ErrorIs(t, errs[0], tm.ErrRevision)
		require.NoError(t, errs[1])
	}
	p.Expected = 2
	p.Progress = tm.Progress{Stage: "PLAN_CONFIRMED", Note: "确认方案"}
	calls := 0
	_, e = s.Execute(ctx, p, func(context.Context) error {
		calls++
		if calls == 2 {
			return tm.ErrForbidden
		}
		return nil
	})
	require.ErrorIs(t, e, tm.ErrForbidden)
	detail, e := s.Detail(ctx, tm.Scope{ActorID: "member", OrganizationID: "org-a"}, false, r.ID)
	require.NoError(t, e)
	require.Equal(t, "2", detail.Request.Revision)
	require.Len(t, detail.Events, 2)
	// Inject a database failure after the state write: neither state nor receipt survives.
	require.NoError(t, db.Exec(`CREATE FUNCTION tool_market.reject_event() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN RAISE EXCEPTION 'test fault'; END $$; CREATE TRIGGER reject_event BEFORE INSERT ON tool_market.events FOR EACH ROW EXECUTE FUNCTION tool_market.reject_event();`).Error)
	_, e = s.Execute(ctx, p, allow)
	require.Error(t, e)
	detail, e = s.Detail(ctx, p.Scope, true, r.ID)
	require.NoError(t, e)
	require.Equal(t, "2", detail.Request.Revision)
	require.NoError(t, db.Exec("DROP TRIGGER reject_event ON tool_market.events").Error)
	rr, e := s.Execute(ctx, p, allow)
	require.NoError(t, e)
	require.Equal(t, "3", rr.Revision)
	p.Progress = tm.Progress{Stage: "DELIVERED", Note: "skip"}
	p.Expected = 3
	p.Key = uuid.NewString()
	_, e = s.Execute(ctx, p, allow)
	require.ErrorIs(t, e, tm.ErrInvalid)
}
func TestFirstActivationCannotOverwriteAndNeedsGuard(t *testing.T) {
	_, s := fixture(t)
	ctx := context.Background()
	c := tm.Command{Scope: tm.Scope{ActorID: "a", OrganizationID: "org-a"}, Key: uuid.NewString(), Operation: "activation", ID: tm.AcquisitionID, Absent: true, Enabled: true}
	_, e := s.Execute(ctx, c, nil)
	require.ErrorIs(t, e, tm.ErrForbidden)
	r, e := s.Execute(ctx, c, allow)
	require.NoError(t, e)
	require.Equal(t, "1", r.Revision)
	c.Key = uuid.NewString()
	_, e = s.Execute(ctx, c, allow)
	require.ErrorIs(t, e, tm.ErrRevision)
	c.Absent = false
	c.Expected = 1
	c.Enabled = false
	r, e = s.Execute(ctx, c, allow)
	require.NoError(t, e)
	require.Equal(t, "2", r.Revision)
	rows, e := s.Activations(ctx, c.Scope)
	require.NoError(t, e)
	require.Len(t, rows, 1)
	require.False(t, rows[0].Enabled)
}
