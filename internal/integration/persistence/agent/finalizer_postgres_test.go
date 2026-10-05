package agentpersistence

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	"task-processor/internal/agent"
)

func TestExpiredRunningFinalizerFencesLateRuntimeCommit(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	container, err := tcpostgres.Run(ctx, "postgres:16-alpine", tcpostgres.WithDatabase("agentfinalizer"),
		tcpostgres.WithUsername("agentfinalizer"), tcpostgres.WithPassword("agentfinalizer"), tcpostgres.BasicWaitStrategies())
	require.NoError(t, err)
	defer func() { require.NoError(t, container.Terminate(context.Background())) }()
	dsn, err := container.ConnectionString(ctx, "sslmode=disable")
	require.NoError(t, err)
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	require.NoError(t, err)
	require.NoError(t, InstallSchema(db))
	store, err := New(db)
	require.NoError(t, err)
	initial := initialRecord()
	run, acquired, err := store.Claim(ctx, initial, 0)
	require.NoError(t, err)
	require.True(t, acquired)
	_, err = store.FinalizeExpiredRunning(ctx, run.State.Scope, run.State.Request.Binding,
		run.State.Request.Key, run.State.Revision, run.State.Deadline.Add(30*time.Second))
	require.ErrorIs(t, err, agent.ErrConflict)
	terminal, err := store.FinalizeExpiredRunning(ctx, run.State.Scope, run.State.Request.Binding,
		run.State.Request.Key, run.State.Revision, run.State.Deadline.Add(31*time.Second))
	require.NoError(t, err)
	require.Equal(t, agent.Stopped, terminal.State.Phase)
	require.Equal(t, agent.StopExecutionOutcomeUnknown, terminal.State.StopReason)
	require.Equal(t, run.State.Revision+1, terminal.State.Revision)
	require.Equal(t, run.State.RunID, terminal.State.RunID)
	require.Equal(t, run.State.Fingerprint, terminal.State.Fingerprint)
	require.Empty(t, terminal.Checkpoint)
	late := run
	late.State.Phase = agent.HumanReviewRequired
	_, err = store.Commit(ctx, late, run.State.Revision)
	require.ErrorIs(t, err, agent.ErrConflict)
	adopted, err := store.FinalizeExpiredRunning(ctx, run.State.Scope, run.State.Request.Binding,
		run.State.Request.Key, run.State.Revision, run.State.Deadline.Add(32*time.Second))
	require.NoError(t, err)
	require.Equal(t, terminal, adopted)
}
