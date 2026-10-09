//go:build integration

package projectcenterpersistence

import (
	"context"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	pg "github.com/testcontainers/testcontainers-go/modules/postgres"
	driver "gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	"net/url"
	"sync"
	pc "task-processor/internal/aiworkbench/projectcenter"
	"testing"
	"time"
)

func TestPrivateProjectTransactions(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	c, e := pg.Run(ctx, "postgres:16-alpine", pg.WithDatabase("projects624"), pg.WithUsername("owner"), pg.WithPassword("isolated-test"), pg.BasicWaitStrategies())
	require.NoError(t, e)
	testcontainers.CleanupContainer(t, c)
	dsn, e := c.ConnectionString(ctx, "sslmode=disable")
	require.NoError(t, e)
	db, e := gorm.Open(driver.Open(dsn), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	require.NoError(t, e)
	raw, e := db.DB()
	require.NoError(t, e)
	defer raw.Close()
	require.NoError(t, InstallSchema(db))
	require.NoError(t, db.Exec("CREATE ROLE ai_projects_runtime LOGIN PASSWORD 'test-project-runtime' NOSUPERUSER NOCREATEDB NOCREATEROLE NOREPLICATION NOBYPASSRLS").Error)
	require.NoError(t, GrantRuntime(db))
	// Use the actual restricted serving identity, including row locks and upsert.
	require.NoError(t, db.Transaction(func(tx *gorm.DB) error {
		if e := tx.Exec("SET LOCAL ROLE ai_projects_runtime").Error; e != nil {
			return e
		}
		return VerifySchema(ctx, tx)
	}))
	runtimeURL, e := url.Parse(dsn)
	require.NoError(t, e)
	runtimeURL.User = url.UserPassword("ai_projects_runtime", "test-project-runtime")
	runtimeDB, e := gorm.Open(driver.Open(runtimeURL.String()), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	require.NoError(t, e)
	runtimeRaw, e := runtimeDB.DB()
	require.NoError(t, e)
	defer runtimeRaw.Close()
	runtimeRaw.SetMaxOpenConns(4)
	store, e := New(runtimeDB)
	require.NoError(t, e)
	scope := pc.Scope{OrganizationID: "org-a", ActorID: "actor-a"}
	command := pc.Command{Operation: "create", Fields: &pc.Fields{Title: "项目", Goal: "长期目标", Kind: "OTHER", DueDate: "2026-12-31"}}
	key := uuid.NewString()
	var wg sync.WaitGroup
	out := make([]pc.Receipt, 4)
	errs := make([]error, 4)
	for i := range out {
		wg.Add(1)
		go func(i int) { defer wg.Done(); out[i], errs[i] = store.Commit(ctx, scope, key, command, nil) }(i)
	}
	wg.Wait()
	for i := range out {
		require.NoError(t, errs[i])
		require.Equal(t, out[0].ID, out[i].ID)
	}
	id := out[0].ID
	project, refs, e := store.Get(ctx, scope, id)
	require.NoError(t, e)
	require.Empty(t, refs)
	require.Equal(t, uint64(1), project.Revision)
	_, _, e = store.Get(ctx, pc.Scope{OrganizationID: "org-a", ActorID: "actor-b"}, id)
	require.ErrorIs(t, e, pc.ErrNotFound)
	_, _, e = store.Get(ctx, pc.Scope{OrganizationID: "org-b", ActorID: "actor-a"}, id)
	require.ErrorIs(t, e, pc.ErrNotFound)
	rows, next, e := store.List(ctx, scope, pc.Query{Mode: "active"})
	require.NoError(t, e)
	require.Empty(t, next)
	require.Len(t, rows, 1)
	require.Equal(t, id, rows[0].ID)
	command.Fields.Title = "different"
	_, e = store.Commit(ctx, scope, key, command, nil)
	require.ErrorIs(t, e, pc.ErrConflict)
	add := pc.Command{Operation: "add", ID: id, Expected: 1, Reference: &pc.Reference{Kind: "CONVERSATION", TargetID: uuid.NewString()}}
	addKey := uuid.NewString()
	added, e := store.Commit(ctx, scope, addKey, add, nil)
	require.NoError(t, e)
	require.Equal(t, uint64(2), added.Revision)
	_, e = store.Commit(ctx, scope, uuid.NewString(), pc.Command{Operation: "archive", ID: id, Expected: 1}, nil)
	require.ErrorIs(t, e, pc.ErrRevision)
	_, e = store.Commit(ctx, scope, uuid.NewString(), pc.Command{Operation: "template_save", ID: id, Expected: 2, Name: "个人模板"}, nil)
	require.NoError(t, e)
	templates, _, e := store.Templates(ctx, scope, "")
	require.NoError(t, e)
	require.Len(t, templates, 1)
	require.Empty(t, templates[0].DueDate)
	require.Equal(t, "长期目标", templates[0].Goal)
	_, e = store.Commit(ctx, scope, uuid.NewString(), pc.Command{Operation: "visit", ID: id}, nil)
	require.NoError(t, e)
	recent, _, e := store.List(ctx, scope, pc.Query{Mode: "recent"})
	require.NoError(t, e)
	require.Len(t, recent, 1)
	require.Equal(t, uint64(2), recent[0].Revision)
	_, e = store.Commit(ctx, scope, uuid.NewString(), pc.Command{Operation: "archive", ID: id, Expected: 2}, nil)
	require.NoError(t, e)
	replay, e := store.Commit(ctx, scope, addKey, add, pc.ErrNotFound)
	require.NoError(t, e)
	require.True(t, replay.Replayed)
	require.Equal(t, uint64(2), replay.Revision)
	_, e = store.Commit(ctx, scope, uuid.NewString(), pc.Command{Operation: "remove", ID: id, Expected: 3, SlotID: uuid.NewString()}, nil)
	require.ErrorIs(t, e, pc.ErrArchived)
	_, e = store.Commit(ctx, scope, uuid.NewString(), pc.Command{Operation: "restore", ID: id, Expected: 3}, nil)
	require.NoError(t, e)
	project, refs, e = store.Get(ctx, scope, id)
	require.NoError(t, e)
	require.Len(t, refs, 1)
	require.Equal(t, uint64(4), project.Revision)
	_, e = store.Commit(ctx, scope, uuid.NewString(), pc.Command{Operation: "remove", ID: id, Expected: 4, SlotID: refs[0].SlotID}, nil)
	require.NoError(t, e)
	require.Error(t, runtimeDB.Exec("DELETE FROM ai_workbench_projects.projects WHERE id=?", id).Error)
	require.Error(t, runtimeDB.Exec("UPDATE ai_workbench_projects.projects SET actor_id='someone' WHERE id=?", id).Error)
	require.NoError(t, runtimeDB.Exec("RESET ROLE").Error)
}
