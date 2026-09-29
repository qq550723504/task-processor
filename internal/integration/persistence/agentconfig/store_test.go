package agentconfigpersistence

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
	"sync/atomic"
	"task-processor/internal/agent"
	"task-processor/internal/agentconfig"
	runstore "task-processor/internal/integration/persistence/agent"
	"testing"
	"time"
)

func fixture(t *testing.T) (*gorm.DB, *Store, *runstore.Store) {
	t.Helper()
	dsn := os.Getenv("ISSUE573_TEST_DSN")
	if dsn == "" {
		t.Skip("requires task-isolated PostgreSQL ISSUE573_TEST_DSN")
	}
	root, e := gorm.Open(postgres.Open(dsn), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	require.NoError(t, e)
	name := "agent573_" + strings.ReplaceAll(uuid.NewString(), "-", "")
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
	require.NoError(t, runstore.InstallSchema(db))
	require.NoError(t, InstallSchema(db))
	s, e := New(db)
	require.NoError(t, e)
	runs, e := runstore.New(db)
	require.NoError(t, e)
	return db, s, runs
}
func command(scope agent.Scope, operation string, revision uint64) agentconfig.Command {
	return agentconfig.Command{Scope: scope, Key: uuid.NewString(), AgentID: "product.title.agent", Operation: operation, Expected: revision}
}
func TestConfigurationReceiptsAndExactDefault(t *testing.T) {
	_, s, _ := fixture(t)
	ctx := context.Background()
	scope := agent.Scope{OrganizationID: "org-a", ActorID: "actor-a"}
	_, e := s.ReadAgent(ctx, scope, "product.title.agent")
	require.ErrorIs(t, e, agentconfig.ErrNotFound)
	enable := command(scope, "enable", 0)
	enable.Absent = true
	r, e := s.Execute(ctx, enable)
	require.NoError(t, e)
	require.Equal(t, "1", r.Revision)
	create := command(scope, "create-template", 0)
	create.Input = agentconfig.TemplateInput{Name: "first", TargetPlatform: "shein"}
	tr, e := s.Execute(ctx, create)
	require.NoError(t, e)
	def := command(scope, "default", 1)
	def.Default = &agentconfig.TemplateRef{TemplateID: tr.TemplateID, Revision: "1"}
	_, e = s.Execute(ctx, def)
	require.NoError(t, e)
	update := command(scope, "update-template", 1)
	update.TemplateID = tr.TemplateID
	update.Input = agentconfig.TemplateInput{Name: "second", TargetPlatform: "amazon"}
	_, e = s.Execute(ctx, update)
	require.NoError(t, e)
	current, e := s.ReadAgent(ctx, scope, enable.AgentID)
	require.NoError(t, e)
	require.Equal(t, "1", current.DefaultTemplate.Revision)
	old, e := s.ReadTemplate(ctx, scope, enable.AgentID, tr.TemplateID, 1)
	require.NoError(t, e)
	require.Equal(t, "first", old.Name)
	replay, e := s.Execute(ctx, enable)
	require.NoError(t, e)
	require.Equal(t, r, replay)
	changed := enable
	changed.Operation = "disable"
	changed.Absent = false
	changed.Expected = 1
	_, e = s.Execute(ctx, changed)
	require.ErrorIs(t, e, agentconfig.ErrConflict)
	arch := command(scope, "archive-template", 2)
	arch.TemplateID = tr.TemplateID
	_, e = s.Execute(ctx, arch)
	require.ErrorIs(t, e, agentconfig.ErrDefault)
	_, e = s.ReadTemplate(ctx, agent.Scope{OrganizationID: "org-b", ActorID: "actor-a"}, enable.AgentID, tr.TemplateID, 1)
	require.ErrorIs(t, e, agentconfig.ErrNotFound)
	clear := command(scope, "default", 2)
	_, e = s.Execute(ctx, clear)
	require.NoError(t, e)
	_, e = s.Execute(ctx, arch)
	require.NoError(t, e)
	replay, e = s.Execute(ctx, update)
	require.NoError(t, e)
	require.Equal(t, "2", replay.Version)
	// Mutable base availability cannot mask a committed receipt or a key conflict.
	eligibleCalls := 0
	replay, e = s.Execute(ctx, update, func(context.Context) error { eligibleCalls++; return agentconfig.ErrUnavailable })
	require.NoError(t, e)
	require.Equal(t, "2", replay.Version)
	require.Zero(t, eligibleCalls)
	update.Input.Name = "changed"
	_, e = s.Execute(ctx, update, func(context.Context) error { eligibleCalls++; return agentconfig.ErrUnavailable })
	require.ErrorIs(t, e, agentconfig.ErrConflict)
	require.Zero(t, eligibleCalls)
}

func TestRuntimePrivilegesPreserveImmutableFacts(t *testing.T) {
	db, _, _ := fixture(t)
	ctx := context.Background()
	role := "agent573_" + strings.ReplaceAll(uuid.NewString(), "-", "")
	require.NoError(t, db.Exec("CREATE ROLE "+role+" NOLOGIN NOSUPERUSER NOCREATEDB NOCREATEROLE NOBYPASSRLS").Error)
	t.Cleanup(func() { _ = db.Exec("DROP OWNED BY " + role).Error; _ = db.Exec("DROP ROLE " + role).Error })
	require.NoError(t, GrantRuntime(db, role))
	require.NoError(t, db.Transaction(func(tx *gorm.DB) error {
		if e := tx.Exec("SET LOCAL ROLE " + role).Error; e != nil {
			return e
		}
		store, e := New(tx)
		if e != nil {
			return e
		}
		c := command(agent.Scope{OrganizationID: "org", ActorID: "admin"}, "enable", 0)
		c.Absent = true
		_, e = store.Execute(ctx, c)
		if e != nil {
			return e
		}
		c = command(agent.Scope{OrganizationID: "org", ActorID: "admin"}, "create-template", 0)
		c.Input = agentconfig.TemplateInput{Name: "privilege test", TargetPlatform: "amazon"}
		receipt, e := store.Execute(ctx, c)
		if e != nil {
			return e
		}
		c = command(c.Scope, "update-template", 1)
		c.TemplateID = receipt.TemplateID
		c.Input = agentconfig.TemplateInput{Name: "second", TargetPlatform: "shein"}
		_, e = store.Execute(ctx, c)
		return e
	}))
	for _, table := range []string{"template_revisions", "start_snapshots", "commands", "organization_agents", "templates"} {
		var allowed bool
		require.NoError(t, db.Raw("SELECT has_table_privilege(?,?,'DELETE,TRUNCATE')", role, "agent_configuration."+table).Scan(&allowed).Error)
		require.False(t, allowed)
		if table == "template_revisions" || table == "start_snapshots" {
			require.NoError(t, db.Raw("SELECT has_table_privilege(?,?,'UPDATE')", role, "agent_configuration."+table).Scan(&allowed).Error)
			require.False(t, allowed)
		}
	}
}

func TestRuntimePrivilegesRejectInheritedWriteAuthority(t *testing.T) {
	for _, tc := range []struct{ name, grant string }{
		{"schema-create", "GRANT CREATE ON SCHEMA agent_configuration TO "},
		{"command-delete", "GRANT DELETE ON agent_configuration.commands TO "},
		{"snapshot-column-update", "GRANT UPDATE (payload) ON agent_configuration.start_snapshots TO "},
		{"command-identity-update", "GRANT UPDATE (fingerprint) ON agent_configuration.commands TO "},
		{"owner-set-role", ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			db, _, _ := fixture(t)
			role := "agent573_" + strings.ReplaceAll(uuid.NewString(), "-", "")
			donor := "agent573_" + strings.ReplaceAll(uuid.NewString(), "-", "")
			require.NoError(t, db.Exec("CREATE ROLE "+role+" NOLOGIN").Error)
			t.Cleanup(func() { _ = db.Exec("DROP OWNED BY " + role).Error; _ = db.Exec("DROP ROLE " + role).Error })
			if tc.grant == "" {
				// NOINHERIT still allows explicitly assuming a member role.
				require.NoError(t, db.Exec("GRANT agent_configuration_owner TO "+role+" WITH INHERIT FALSE").Error)
			} else {
				require.NoError(t, db.Exec("CREATE ROLE "+donor+" NOLOGIN").Error)
				t.Cleanup(func() { _ = db.Exec("DROP OWNED BY " + donor).Error; _ = db.Exec("DROP ROLE " + donor).Error })
				require.NoError(t, db.Exec(tc.grant+donor).Error)
				require.NoError(t, db.Exec("GRANT "+donor+" TO "+role).Error)
			}
			err := db.Transaction(func(tx *gorm.DB) error { return GrantRuntime(tx, role) })
			require.ErrorIs(t, err, agentconfig.ErrInvalid)
		})
	}
}
func TestSnapshotAdmissionAndDisableABA(t *testing.T) {
	db, s, runs := fixture(t)
	ctx := context.Background()
	scope := agent.Scope{OrganizationID: "org-a", ActorID: "actor-a"}
	limits := agent.Limits{Steps: 8, ModelCalls: 3, Tokens: 100, CostMicros: 100, Currency: "CNY", Runtime: time.Minute}
	start := agentconfig.StartCommand{Scope: scope, AgentID: "product.title.agent", AgentVersion: "v1.0.0", Request: agent.Request{Key: uuid.NewString(), Binding: agent.Binding{ContextKind: "acquisition", ContextID: uuid.NewString(), ProductKey: "p", CatalogVersion: "1", PublicationID: "pub", TargetPlatform: "shein"}, PolicyVersion: "title-review-v1", PromptVersion: "product-title-agent-v1", Limits: limits}}
	_, e := s.Prepare(ctx, start)
	require.ErrorIs(t, e, agentconfig.ErrNotEnabled)
	enable := command(scope, "enable", 0)
	enable.Absent = true
	_, e = s.Execute(ctx, enable)
	require.NoError(t, e)
	snap, e := s.Prepare(ctx, start)
	require.NoError(t, e)
	start.Request.Limits.Tokens++
	adopted, e := s.Prepare(ctx, start)
	require.NoError(t, e)
	require.Equal(t, snap, adopted)
	changed := start
	changed.Request.Binding.TargetPlatform = "amazon"
	_, e = s.Prepare(ctx, changed)
	require.ErrorIs(t, e, agentconfig.ErrConflict)
	request := snap.Request
	request.ConfigurationSnapshotRef = agent.ConfigurationSnapshotRef{Kind: agentconfig.SnapshotKind, ID: snap.ID, Digest: snap.Digest}
	now := time.Now().UTC().Truncate(time.Microsecond)
	initial := agent.Record{State: agent.State{RunID: uuid.NewString(), Scope: scope, Request: request, Fingerprint: strings.Repeat("a", 64), Phase: agent.Running, StartedAt: now, Deadline: now.Add(time.Minute), HumanReviewRequired: true}}
	guard, e := NewGuard(db, s, runs, func(string, string) (agent.Limits, error) { return limits, nil })
	require.NoError(t, e)
	tight := limits
	tight.Tokens--
	tightGuard, e := NewGuard(db, s, runs, func(string, string) (agent.Limits, error) { return tight, nil })
	require.NoError(t, e)
	_, acquired, e := tightGuard.Claim(ctx, initial, 0)
	require.ErrorIs(t, e, agentconfig.ErrChanged)
	require.False(t, acquired)
	_, e = s.Execute(ctx, command(scope, "disable", 1))
	require.NoError(t, e)
	_, e = s.Execute(ctx, command(scope, "enable", 2))
	require.NoError(t, e)
	_, acquired, e = guard.Claim(ctx, initial, 0)
	require.ErrorIs(t, e, agentconfig.ErrChanged)
	require.False(t, acquired)
	start.Request.Key = uuid.NewString()
	start.Request.Limits = limits
	snap, e = s.Prepare(ctx, start)
	require.NoError(t, e)
	initial.State.Request = snap.Request
	initial.State.Request.ConfigurationSnapshotRef = agent.ConfigurationSnapshotRef{Kind: agentconfig.SnapshotKind, ID: snap.ID, Digest: snap.Digest}
	var winners atomic.Int32
	var wg sync.WaitGroup
	for i := 0; i < 6; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, won, err := guard.Claim(ctx, initial, 0)
			if err != nil {
				t.Error(err)
			}
			if won {
				winners.Add(1)
			}
		}()
	}
	wg.Wait()
	require.EqualValues(t, 1, winners.Load())
	run, _, e := guard.Claim(ctx, initial, 0)
	require.NoError(t, e)
	_, e = s.Execute(ctx, command(scope, "disable", 3))
	require.NoError(t, e)
	_, acquired, e = guard.Claim(ctx, initial, 0)
	require.NoError(t, e)
	require.False(t, acquired)
	run.State.Phase = agent.Interrupted
	run.Checkpoint = []byte("checkpoint")
	run, e = guard.Commit(ctx, run, run.State.Revision)
	require.NoError(t, e)
	_, acquired, e = guard.Claim(ctx, initial, run.State.Revision)
	require.ErrorIs(t, e, agentconfig.ErrNotEnabled)
	require.False(t, acquired)
	_, e = s.Execute(ctx, command(scope, "enable", 4))
	require.NoError(t, e)
	_, acquired, e = tightGuard.Claim(ctx, initial, run.State.Revision)
	require.ErrorIs(t, e, agentconfig.ErrChanged)
	require.False(t, acquired)
	_, acquired, e = guard.Claim(ctx, initial, run.State.Revision)
	require.NoError(t, e)
	require.True(t, acquired)
}
