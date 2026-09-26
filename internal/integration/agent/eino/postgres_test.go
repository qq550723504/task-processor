package einoruntime

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	"task-processor/internal/agent"
	agentstore "task-processor/internal/integration/persistence/agent"
)

func postgresRuntimeStore(t *testing.T) func() agent.Store {
	t.Helper()
	dsn := os.Getenv("ISSUE382_TEST_DSN")
	if dsn == "" {
		t.Skip("requires task-isolated PostgreSQL ISSUE382_TEST_DSN")
	}
	cfg := &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)}
	root, err := gorm.Open(postgres.Open(dsn), cfg)
	require.NoError(t, err)
	schema := "agent_eino502_" + strings.ReplaceAll(uuid.NewString(), "-", "")
	require.NoError(t, root.Exec("CREATE SCHEMA "+schema).Error)
	var opened []*gorm.DB
	t.Cleanup(func() {
		for _, db := range opened {
			raw, _ := db.DB()
			_ = raw.Close()
		}
		_ = root.Exec("DROP SCHEMA " + schema + " CASCADE").Error
		raw, _ := root.DB()
		_ = raw.Close()
	})
	return func() agent.Store {
		db, err := gorm.Open(postgres.Open(dsn+" search_path="+schema), cfg)
		require.NoError(t, err)
		opened = append(opened, db)
		require.NoError(t, agentstore.InstallSchema(db))
		s, err := agentstore.New(db)
		require.NoError(t, err)
		return s
	}
}

func TestPostgresRuntimeCheckpointAcrossNewConnections(t *testing.T) {
	newStore := postgresRuntimeStore(t)
	r, req, model, _, _, _, _ := fixture(t, agent.Action{Kind: "interrupt"}, proposal("supported title"))
	r.config.Store = newStore()
	before, err := r.Start(context.Background(), req)
	require.NoError(t, err)
	require.Equal(t, agent.Interrupted, before.State.Phase)
	require.NotEmpty(t, before.Checkpoint)
	config := r.config
	config.Store = newStore()
	restarted, err := New(config)
	require.NoError(t, err)
	after, err := restarted.Resume(context.Background(), req, before.State.Revision, "confirmed source wording")
	require.NoError(t, err)
	require.Equal(t, agent.HumanReviewRequired, after.State.Phase)
	require.True(t, after.State.Validation.Valid)
	require.Equal(t, before.State.RunID, after.State.RunID)
	require.Equal(t, before.State.Deadline, after.State.Deadline)
	require.Equal(t, before.State.Request, after.State.Request)
	require.Equal(t, before.State.Usage.ModelCalls+1, after.State.Usage.ModelCalls)
	require.Equal(t, 2, model.calls)
	require.Equal(t, "confirmed source wording", model.lastFeedback)
	replay, err := restarted.Start(context.Background(), req)
	require.NoError(t, err)
	require.Equal(t, after, replay)
	require.Equal(t, 2, model.calls)
}

func TestPostgresRuntimeLostModelResponseCannotBeResumed(t *testing.T) {
	newStore := postgresRuntimeStore(t)
	r, req, model, _, _, _, _ := fixture(t, proposal("unused"))
	model.fail = true
	r.config.Store = newStore()
	stopped, err := r.Start(context.Background(), req)
	require.NoError(t, err)
	require.Equal(t, agent.StopModelUnknown, stopped.State.StopReason)
	require.NotEmpty(t, stopped.State.PendingInvocationID)
	config := r.config
	config.Store = newStore()
	restarted, err := New(config)
	require.NoError(t, err)
	replayed, err := restarted.Start(context.Background(), req)
	require.NoError(t, err)
	require.Equal(t, stopped, replayed)
	_, err = restarted.Resume(context.Background(), req, stopped.State.Revision, "")
	require.ErrorIs(t, err, agent.ErrConflict)
	require.Equal(t, 1, model.calls)
}

func TestPostgresRuntimePersistsCancelledAndExpiredRuns(t *testing.T) {
	for _, tc := range []struct {
		name        string
		cancel      bool
		duringModel bool
		reason      agent.StopReason
	}{
		{"cancel before dispatch", true, false, agent.StopCancelled},
		{"runtime before dispatch", false, false, agent.StopRuntime},
		{"cancel during model", true, true, agent.StopModelUnknown},
		{"runtime during model", false, true, agent.StopModelUnknown},
	} {
		t.Run(tc.name, func(t *testing.T) {
			newStore := postgresRuntimeStore(t)
			r, req, model, _, _, _, _ := fixture(t)
			r.config.Store = newStore()
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			if tc.duringModel {
				model.waitStarted = make(chan struct{})
				if tc.cancel {
					go func() {
						select {
						case <-model.waitStarted:
							cancel()
						case <-ctx.Done():
						}
					}()
				} else {
					req.Limits.Runtime = 2 * time.Second
				}
			} else if tc.cancel {
				r.config.Store = afterClaimStore{Store: r.config.Store, afterClaim: cancel}
			} else {
				req.Limits.Runtime = time.Nanosecond
			}
			stopped, err := r.Start(ctx, req)
			require.NoError(t, err)
			require.Equal(t, agent.Stopped, stopped.State.Phase)
			require.Equal(t, tc.reason, stopped.State.StopReason)
			require.Empty(t, stopped.Checkpoint)
			calls := 0
			if tc.duringModel {
				calls = 1
				require.NotEmpty(t, stopped.State.PendingInvocationID)
			} else {
				require.Empty(t, stopped.State.PendingInvocationID)
			}
			require.Equal(t, calls, model.calls)
			config := r.config
			config.Store = newStore()
			restarted, err := New(config)
			require.NoError(t, err)
			replayed, err := restarted.Start(context.Background(), req)
			require.NoError(t, err)
			require.Equal(t, stopped, replayed)
			_, err = restarted.Resume(context.Background(), req, stopped.State.Revision, "")
			require.ErrorIs(t, err, agent.ErrConflict)
			require.Equal(t, calls, model.calls)
		})
	}
}
