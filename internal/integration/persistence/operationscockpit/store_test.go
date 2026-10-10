package operationscockpitpersistence

import (
	"context"
	"encoding/json"
	"errors"
	"net/url"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	c "task-processor/internal/operationscockpit"
)

type fixtureBoundary struct {
	revoked      atomic.Bool
	failCommit   atomic.Bool
	revokeCreate atomic.Bool
	calls        atomic.Int64
}

func (b *fixtureBoundary) Current(ctx context.Context, scope c.Scope) (c.Access, error) {
	if ctx.Err() != nil {
		return c.Access{}, ctx.Err()
	}
	if b.revoked.Load() || scope.OrganizationID != "org-a" && scope.OrganizationID != "org-create" {
		return c.Access{}, c.ErrForbidden
	}
	count := b.calls.Add(1)
	if b.failCommit.Load() && count > 1 {
		return c.Access{}, c.ErrForbidden
	}
	return c.Access{GoalsRead: true, GoalsCreate: !(b.revokeCreate.Load() && count > 1), GoalsManage: scope.ActorID == "manager", StoresRead: true, FactsWrite: true}, nil
}
func (b *fixtureBoundary) ReadStores(ctx context.Context, scope c.Scope, ids []string) error {
	if b.revoked.Load() {
		return c.ErrForbidden
	}
	return ctx.Err()
}
func (b *fixtureBoundary) LockStores(ctx context.Context, tx *gorm.DB, scope c.Scope, ids []string) error {
	return b.ReadStores(ctx, scope, ids)
}

func TestPostgresGoalCreatorManagerCASAndReplay(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	container, err := tcpostgres.Run(ctx, "postgres:16-alpine", tcpostgres.WithDatabase("cockpit"), tcpostgres.WithUsername("fixture_owner"), tcpostgres.WithPassword("isolated-cockpit-test"), tcpostgres.BasicWaitStrategies())
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, container.Terminate(context.Background())) })
	dsn, err := container.ConnectionString(ctx, "sslmode=disable")
	require.NoError(t, err)
	owner, err := gorm.Open(postgres.Open(dsn), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	require.NoError(t, err)
	pool, err := owner.DB()
	require.NoError(t, err)
	t.Cleanup(func() { _ = pool.Close() })
	require.NoError(t, Install(ctx, owner))
	require.NoError(t, owner.Exec("CREATE ROLE cockpit_fixture_runtime LOGIN PASSWORD 'isolated-runtime-test' NOSUPERUSER NOCREATEDB NOCREATEROLE NOREPLICATION NOBYPASSRLS").Error)
	require.NoError(t, GrantRuntime(ctx, owner, "cockpit_fixture_runtime"))
	u, err := url.Parse(dsn)
	require.NoError(t, err)
	u.User = url.UserPassword("cockpit_fixture_runtime", "isolated-runtime-test")
	db, err := gorm.Open(postgres.Open(u.String()), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	require.NoError(t, err)
	raw, err := db.DB()
	require.NoError(t, err)
	t.Cleanup(func() { _ = raw.Close() })
	require.NoError(t, VerifyRuntime(ctx, db))
	now := time.Date(2026, 10, 10, 0, 0, 0, 0, time.UTC)
	boundary := &fixtureBoundary{}
	repository, err := New(ctx, db, boundary, func() time.Time { return now })
	require.NoError(t, err)
	storeID := uuid.NewString()
	goal := c.GoalConfig{StoreIDs: []string{storeID}, Frequency: "month", Period: c.Period{Start: "2026-10-01", End: "2026-10-31"}, Profit: 10000, NormalBPS: 9000, AttentionBPS: 7000}
	command := c.Command{Scope: c.Scope{OrganizationID: "org-a", ActorID: "creator"}, Key: uuid.NewString(), Operation: "goal_create", ID: uuid.NewString(), Goal: &goal}
	receipts := make([]c.Receipt, 2)
	errs := make([]error, 2)
	var wg sync.WaitGroup
	for i := range errs {
		wg.Add(1)
		go func(i int) { defer wg.Done(); receipts[i], errs[i] = repository.Execute(ctx, command) }(i)
	}
	wg.Wait()
	// Becoming a creator inside this transaction must not replace the fresh
	// create capability when it is revoked immediately before commit.
	boundary.calls.Store(0)
	boundary.revokeCreate.Store(true)
	withdrawnCreate := command
	withdrawnCreate.Scope.OrganizationID = "org-create"
	withdrawnCreate.Key = uuid.NewString()
	withdrawnCreate.ID = uuid.NewString()
	_, err = repository.Execute(ctx, withdrawnCreate)
	require.ErrorIs(t, err, c.ErrForbidden)
	boundary.revokeCreate.Store(false)
	var withdrawnCount int64
	require.NoError(t, owner.Table("operations_cockpit.goal_heads").Where("organization_id=?", "org-create").Count(&withdrawnCount).Error)
	require.Zero(t, withdrawnCount)
	for i := range errs {
		require.NoError(t, errs[i])
		require.Equal(t, receipts[0], receipts[i])
	}
	var count int64
	require.NoError(t, owner.Table("operations_cockpit.goal_versions").Count(&count).Error)
	require.EqualValues(t, 1, count)
	changed := command
	g := goal
	g.Profit = 20000
	changed.Goal = &g
	_, err = repository.Execute(ctx, changed)
	require.ErrorIs(t, err, c.ErrConflict)
	update := c.Command{Scope: command.Scope, Key: uuid.NewString(), Operation: "goal_update", ID: command.ID, Expected: 1, Goal: &g}
	update.Scope.ActorID = "other"
	_, err = repository.Execute(ctx, update)
	require.ErrorIs(t, err, c.ErrForbidden)
	_, err = repository.HeadMetadata(ctx, update.Scope)
	require.ErrorIs(t, err, c.ErrForbidden)
	update.Scope.ActorID = "manager"
	receipt, err := repository.Execute(ctx, update)
	require.NoError(t, err)
	require.Equal(t, "2", receipt.Revision)
	view, err := repository.Goal(ctx, command.Scope)
	require.NoError(t, err)
	require.Equal(t, "creator", view.CreatorID)
	_, err = repository.Execute(ctx, c.Command{Scope: command.Scope, Key: uuid.NewString(), Operation: "goal_update", ID: command.ID, Expected: 1, Goal: &goal})
	require.ErrorIs(t, err, c.ErrRevision)
	boundary.failCommit.Store(true)
	boundary.calls.Store(0)
	update.Key = uuid.NewString()
	update.Expected = 2
	_, err = repository.Execute(ctx, update)
	require.ErrorIs(t, err, c.ErrForbidden)
	boundary.failCommit.Store(false)
	view, err = repository.Goal(ctx, command.Scope)
	require.NoError(t, err)
	require.EqualValues(t, 2, view.Revision)
	boundary.revoked.Store(true)
	_, err = repository.Execute(ctx, command)
	require.ErrorIs(t, err, c.ErrForbidden)
	boundary.revoked.Store(false)
	// Store-scope withdrawal permits only redacted CAS metadata and a new valid scope.
	denied := &scopeBoundary{fixtureBoundary: boundary, denied: storeID}
	restricted, err := New(ctx, db, denied, func() time.Time { return now })
	require.NoError(t, err)
	metadata, err := restricted.HeadMetadata(ctx, command.Scope)
	require.NoError(t, err)
	require.False(t, metadata.ScopeValid)
	body, err := json.Marshal(metadata)
	require.NoError(t, err)
	require.NotContains(t, string(body), storeID)
	require.NotContains(t, string(body), "creator")
	require.NotContains(t, string(body), "20000")
	_, err = restricted.Goal(ctx, command.Scope)
	require.ErrorIs(t, err, c.ErrForbidden)
	_, err = restricted.Execute(ctx, command)
	require.ErrorIs(t, err, c.ErrForbidden)
	newGoal := goal
	newGoal.StoreIDs = []string{uuid.NewString()}
	_, err = restricted.Execute(ctx, c.Command{Scope: command.Scope, Key: uuid.NewString(), Operation: "goal_update", ID: command.ID, Expected: 2, Goal: &newGoal})
	require.NoError(t, err)
	_, err = restricted.Execute(ctx, c.Command{Scope: command.Scope, Key: uuid.NewString(), Operation: "goal_restore", ID: command.ID, Expected: 3, SourceRevision: 1})
	require.ErrorIs(t, err, c.ErrForbidden)
	var savedFact c.Command
	t.Run("date correction remains one record and rejects overlap", func(t *testing.T) {
		fact := c.FactInput{Period: c.Period{Start: "2026-10-01", End: "2026-10-02"}, Amounts: c.Amounts{Revenue: 1000, Procurement: 300}}
		cmd := c.Command{Scope: command.Scope, Key: uuid.NewString(), Operation: "fact_create", ID: uuid.NewString(), StoreID: storeID, Fact: &fact}
		_, err := repository.Execute(ctx, cmd)
		require.NoError(t, err)
		fact.Period = c.Period{Start: "2026-10-03", End: "2026-10-04"}
		cmd.Key = uuid.NewString()
		cmd.Operation = "fact_update"
		cmd.Expected = 1
		_, err = repository.Execute(ctx, cmd)
		require.NoError(t, err)
		other := cmd
		other.Key = uuid.NewString()
		other.Operation = "fact_create"
		other.Expected = 0
		other.ID = uuid.NewString()
		_, err = repository.Execute(ctx, other)
		require.ErrorIs(t, err, c.ErrOverlap)
		var n int64
		require.NoError(t, owner.Table("operations_cockpit.fact_versions").Where("record_id=?", cmd.ID).Count(&n).Error)
		require.EqualValues(t, 2, n)
		other.StoreID = uuid.NewString()
		other.Operation = "fact_update"
		other.ID = cmd.ID
		other.Expected = 2
		_, err = repository.Execute(ctx, other)
		require.Error(t, err)
		savedFact = cmd
	})
	t.Run("goal and financial revisions share one read snapshot", func(t *testing.T) {
		hooked := &snapshotBoundary{fixtureBoundary: boundary, trigger: newGoal.StoreIDs[0]}
		hooked.beforeFacts = func() {
			newFact := *savedFact.Fact
			newFact.Amounts.Revenue = 2000
			write := savedFact
			write.Key = uuid.NewString()
			write.Expected = 2
			write.Fact = &newFact
			_, e := repository.Execute(ctx, write)
			require.NoError(t, e)
			changedGoal := newGoal
			changedGoal.Profit = 22222
			_, e = repository.Execute(ctx, c.Command{Scope: command.Scope, Key: uuid.NewString(), Operation: "goal_update", ID: command.ID, Expected: 3, Goal: &changedGoal})
			require.NoError(t, e)
		}
		reader, e := New(ctx, db, hooked, func() time.Time { return now })
		require.NoError(t, e)
		query := c.Query{Module: "stores", Period: c.Period{Start: "2026-10-01", End: "2026-10-09"}, StoreIDs: []string{storeID}}
		snapshot, e := reader.Snapshot(ctx, command.Scope, query)
		require.NoError(t, e)
		require.EqualValues(t, 3, snapshot.Goal.Revision)
		require.EqualValues(t, 700, snapshot.Stores[storeID].Totals.NetProfit)
		require.EqualValues(t, 2, snapshot.Stores[storeID].Records[0].Revision)
		require.False(t, snapshot.Stores[storeID].Complete)
		require.Equal(t, "pending_data", snapshot.Evaluation.State)
		current, e := repository.Snapshot(ctx, command.Scope, query)
		require.NoError(t, e)
		require.EqualValues(t, 4, current.Goal.Revision)
		require.EqualValues(t, 1700, current.Stores[storeID].Totals.NetProfit)
		boundary.calls.Store(0)
		boundary.failCommit.Store(true)
		_, e = repository.Snapshot(ctx, command.Scope, query)
		require.ErrorIs(t, e, c.ErrForbidden)
		boundary.failCommit.Store(false)
	})
	t.Run("concurrent maintainers cannot overwrite each other", func(t *testing.T) {
		changes := newGoal
		changes.Profit = 30000
		results := make([]error, 2)
		for i := range results {
			wg.Add(1)
			go func(i int) {
				defer wg.Done()
				_, results[i] = repository.Execute(ctx, c.Command{Scope: command.Scope, Key: uuid.NewString(), Operation: "goal_update", ID: command.ID, Expected: 4, Goal: &changes})
			}(i)
		}
		wg.Wait()
		passed := 0
		stale := 0
		for _, err := range results {
			if err == nil {
				passed++
			} else if errors.Is(err, c.ErrRevision) {
				stale++
			} else {
				t.Fatal(err)
			}
		}
		require.Equal(t, 1, passed)
		require.Equal(t, 1, stale)
	})
	t.Run("concurrent new overlapping periods produce one fact", func(t *testing.T) {
		fact := c.FactInput{Period: c.Period{Start: "2026-10-05", End: "2026-10-06"}, Amounts: c.Amounts{Revenue: 100}}
		results := make([]error, 2)
		for i := range results {
			wg.Add(1)
			go func(i int) {
				defer wg.Done()
				_, results[i] = repository.Execute(ctx, c.Command{Scope: command.Scope, Key: uuid.NewString(), Operation: "fact_create", ID: uuid.NewString(), StoreID: storeID, Fact: &fact})
			}(i)
		}
		wg.Wait()
		passed := 0
		overlap := 0
		for _, err := range results {
			if err == nil {
				passed++
			} else if errors.Is(err, c.ErrOverlap) {
				overlap++
			} else {
				t.Fatal(err)
			}
		}
		require.Equal(t, 1, passed)
		require.Equal(t, 1, overlap)
	})
	// Runtime cannot mutate immutable versions or issue DDL/deletes.
	require.Error(t, db.Exec("DELETE FROM operations_cockpit.goal_heads").Error)
	require.Error(t, db.Exec("UPDATE operations_cockpit.goal_versions SET revision=9").Error)
	require.Error(t, db.Exec("CREATE TABLE operations_cockpit.forbidden(id int)").Error)
}

type scopeBoundary struct {
	*fixtureBoundary
	denied string
}

type snapshotBoundary struct {
	*fixtureBoundary
	trigger     string
	beforeFacts func()
	once        sync.Once
}

func (b *snapshotBoundary) ReadStores(ctx context.Context, scope c.Scope, ids []string) error {
	if err := b.fixtureBoundary.ReadStores(ctx, scope, ids); err != nil {
		return err
	}
	for _, id := range ids {
		if id == b.trigger {
			b.once.Do(b.beforeFacts)
		}
	}
	return nil
}

func (b *scopeBoundary) ReadStores(ctx context.Context, scope c.Scope, ids []string) error {
	for _, id := range ids {
		if id == b.denied {
			return c.ErrForbidden
		}
	}
	return b.fixtureBoundary.ReadStores(ctx, scope, ids)
}
func (b *scopeBoundary) LockStores(ctx context.Context, tx *gorm.DB, scope c.Scope, ids []string) error {
	return b.ReadStores(ctx, scope, ids)
}
