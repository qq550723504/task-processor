package agentcustomizationpersistence

import (
	"context"
	"database/sql"
	"github.com/google/uuid"
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/stretchr/testify/require"
	"net/url"
	"os"
	"strings"
	"sync"
	d "task-processor/internal/agentcustomization"
	"testing"
)

func fixture(t *testing.T) (*sql.DB, *Store) {
	t.Helper()
	dsn := os.Getenv("ISSUE611_TEST_DSN")
	if dsn == "" {
		t.Skip("requires isolated ISSUE611_TEST_DSN")
	}
	root, e := sql.Open("pgx", dsn)
	require.NoError(t, e)
	name := "agent611_" + strings.ReplaceAll(uuid.NewString(), "-", "")
	_, e = root.Exec("CREATE DATABASE " + name)
	require.NoError(t, e)
	u, e := url.Parse(dsn)
	require.NoError(t, e)
	u.Path = "/" + name
	db, e := sql.Open("pgx", u.String())
	require.NoError(t, e)
	t.Cleanup(func() { db.Close(); root.Exec("DROP DATABASE " + name + " WITH (FORCE)"); root.Close() })
	require.NoError(t, InstallSchema(context.Background(), db))
	s, e := New(db)
	require.NoError(t, e)
	return db, s
}
func submit() d.Command {
	return d.Command{Scope: d.Scope{OrganizationID: "org-a", ActorID: "member-a"}, Key: uuid.NewString(), Operation: "submit", Input: d.Input{Name: "标题需求", Scenario: "商品维护", Direction: "PRODUCT_SUPPLY", Description: "形成建议", ContactName: "测试", ContactMethod: "test-only", Consent: true, Files: []d.Upload{{Name: "参考.csv", Data: []byte("商品,说明\n标题,资料")}}}}
}
func service(t *testing.T, s *Store) *d.Service {
	v, e := d.NewService(s)
	require.NoError(t, e)
	return v
}
func TestFirstConcurrentIntentAndScopeIsolation(t *testing.T) {
	db, store := fixture(t)
	s := service(t, store)
	ctx := context.Background()
	c := submit()
	receipts := make([]d.Receipt, 8)
	errs := make([]error, 8)
	var wg sync.WaitGroup
	for i := range receipts {
		wg.Add(1)
		go func(i int) { defer wg.Done(); receipts[i], errs[i] = s.Execute(ctx, c) }(i)
	}
	wg.Wait()
	for i := range receipts {
		require.NoError(t, errs[i])
		require.Equal(t, receipts[0], receipts[i])
	}
	for table := range map[string]bool{"requests": true, "attachments": true, "events": true, "commands": true} {
		var n int
		require.NoError(t, db.QueryRow("SELECT count(*) FROM agent_customization."+table).Scan(&n))
		require.Equal(t, 1, n)
	}
	changed := c
	changed.Input.Files = []d.Upload{{Name: "参考.csv", Data: []byte("different")}}
	_, e := s.Execute(ctx, changed)
	require.ErrorIs(t, e, d.ErrConflict)
	view, e := s.Read(ctx, c.Scope, receipts[0].RequestID, 0)
	require.NoError(t, e)
	require.Empty(t, view.Request.Input.Files)
	require.Len(t, view.Request.Attachments, 1)
	_, e = s.Read(ctx, d.Scope{OrganizationID: "org-b", ActorID: "member-b"}, receipts[0].RequestID, 0)
	require.ErrorIs(t, e, d.ErrNotFound)
	file := view.Request.Attachments[0]
	_, _, e = s.Download(ctx, d.Scope{OrganizationID: "org-b", ActorID: "member-b"}, view.Request.ID, file.ID)
	require.ErrorIs(t, e, d.ErrNotFound)
	_, data, e := s.Download(ctx, c.Scope, view.Request.ID, file.ID)
	require.NoError(t, e)
	require.Equal(t, c.Input.Files[0].Data, data)
	platform := d.Scope{ActorID: "specialist", Platform: true}
	u := d.Command{Scope: platform, Key: uuid.NewString(), ID: view.Request.ID, Expected: 1, Operation: "progress", Update: d.Update{Stage: d.Evaluating, Note: "已联系并开始评估"}}
	ur, e := s.Execute(ctx, u)
	require.NoError(t, e)
	again, e := s.Execute(ctx, u)
	require.NoError(t, e)
	require.Equal(t, ur, again)
	u.Key = uuid.NewString()
	_, e = s.Execute(ctx, u)
	require.ErrorIs(t, e, d.ErrRevision)
	// A new service/store over the same durable database reads the committed result.
	reopened, e := New(db)
	require.NoError(t, e)
	current, e := service(t, reopened).Read(ctx, c.Scope, view.Request.ID, 0)
	require.NoError(t, e)
	require.Equal(t, "2", current.Request.Version)
	require.Len(t, current.Events, 2)
}
func TestTransactionFailureLeavesNoPartialRequest(t *testing.T) {
	db, store := fixture(t)
	_, e := db.Exec("CREATE FUNCTION agent_customization.fail_event() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN RAISE EXCEPTION 'injected failure'; END $$; CREATE TRIGGER fail_event BEFORE INSERT ON agent_customization.events FOR EACH ROW EXECUTE FUNCTION agent_customization.fail_event()")
	require.NoError(t, e)
	_, e = service(t, store).Execute(context.Background(), submit())
	require.Error(t, e)
	for _, table := range []string{"requests", "attachments", "events", "commands"} {
		var n int
		require.NoError(t, db.QueryRow("SELECT count(*) FROM agent_customization."+table).Scan(&n))
		require.Zero(t, n)
	}
}
func TestProgressCASAndImmutableServingAuthority(t *testing.T) {
	db, store := fixture(t)
	s := service(t, store)
	ctx := context.Background()
	c := submit()
	r, e := s.Execute(ctx, c)
	require.NoError(t, e)
	a := d.Command{Scope: d.Scope{ActorID: "specialist", Platform: true}, ID: r.RequestID, Key: uuid.NewString(), Operation: "progress", Expected: 1, Update: d.Update{Stage: d.Evaluating, Note: "评估"}}
	b := a
	b.Key = uuid.NewString()
	var errs [2]error
	var wg sync.WaitGroup
	for i, v := range []d.Command{a, b} {
		wg.Add(1)
		go func(i int, v d.Command) { defer wg.Done(); _, errs[i] = s.Execute(ctx, v) }(i, v)
	}
	wg.Wait()
	require.True(t, (errs[0] == nil && errs[1] == d.ErrRevision) || (errs[1] == nil && errs[0] == d.ErrRevision))
	role := "agent611_" + strings.ReplaceAll(uuid.NewString(), "-", "")
	_, e = db.Exec("CREATE ROLE " + role + " NOLOGIN")
	require.NoError(t, e)
	require.NoError(t, GrantRuntime(ctx, db, role))
	for _, table := range []string{"events", "attachments", "commands"} {
		var unsafe bool
		require.NoError(t, db.QueryRow("SELECT has_table_privilege($1,$2,'UPDATE,DELETE,TRUNCATE')", role, "agent_customization."+table).Scan(&unsafe))
		require.False(t, unsafe)
	}
}

func TestGrantRejectsInheritedRequestIdentityMutation(t *testing.T) {
	db, _ := fixture(t)
	ctx := context.Background()
	parent := "agent611_" + strings.ReplaceAll(uuid.NewString(), "-", "")
	role := "agent611_" + strings.ReplaceAll(uuid.NewString(), "-", "")
	_, err := db.Exec("CREATE ROLE " + parent + " NOLOGIN; CREATE ROLE " + role + " NOLOGIN; GRANT " + parent + " TO " + role + "; GRANT UPDATE(organization_id) ON agent_customization.requests TO " + parent)
	require.NoError(t, err)
	require.ErrorIs(t, GrantRuntime(ctx, db, role), d.ErrInvalid)
}
