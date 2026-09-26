package agentpersistence

import (
	"context"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	"task-processor/internal/agent"
	"task-processor/internal/commercetool"
)

func storeFixture(t *testing.T) (*gorm.DB, *Store) {
	t.Helper()
	dsn := os.Getenv("ISSUE382_TEST_DSN")
	if dsn == "" {
		t.Skip("requires task-isolated PostgreSQL ISSUE382_TEST_DSN")
	}
	root, err := gorm.Open(postgres.Open(dsn), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	require.NoError(t, err)
	schema := "agent502_" + strings.ReplaceAll(uuid.NewString(), "-", "")
	require.NoError(t, root.Exec("CREATE SCHEMA "+schema).Error)
	db, err := gorm.Open(postgres.Open(dsn+" search_path="+schema), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	require.NoError(t, err)
	t.Cleanup(func() {
		raw, _ := db.DB()
		_ = raw.Close()
		_ = root.Exec("DROP SCHEMA " + schema + " CASCADE").Error
		raw, _ = root.DB()
		_ = raw.Close()
	})
	require.NoError(t, InstallSchema(db))
	s, err := New(db)
	require.NoError(t, err)
	return db, s
}

func initialRecord() agent.Record {
	now := time.Now().UTC().Truncate(time.Microsecond)
	return agent.Record{State: agent.State{RunID: uuid.NewString(), Scope: agent.Scope{OrganizationID: "org-1", ActorID: "user-1"}, Fingerprint: strings.Repeat("a", 64), Phase: agent.Running, StartedAt: now, Deadline: now.Add(time.Minute), HumanReviewRequired: true,
		Request: agent.Request{Key: "request-1", Binding: agent.Binding{ContextKind: "acquisition", ContextID: "operation-1", ProductKey: "product-1", CatalogVersion: "1", PublicationID: "publication-1", TargetPlatform: "shein"}, PolicyVersion: "title-review-v1", PromptVersion: "prompt-v1", Limits: agent.Limits{Steps: 8, ModelCalls: 3, Tokens: 100, CostMicros: 100, Currency: "CNY", Runtime: time.Minute}}}}
}

func TestAgentStoreScopedReadAndDurableToolAudit(t *testing.T) {
	_, s := storeFixture(t)
	ctx := context.Background()
	initial := initialRecord()
	run, _, err := s.Claim(ctx, initial, 0)
	require.NoError(t, err)
	got, err := s.Read(ctx, run.State.Scope, run.State.Request.Binding.ContextID, run.State.Request.Key)
	require.NoError(t, err)
	require.Equal(t, run, got)
	_, err = s.Read(ctx, agent.Scope{OrganizationID: "other", ActorID: run.State.Scope.ActorID}, run.State.Request.Binding.ContextID, run.State.Request.Key)
	require.Error(t, err)
	audit := commercetool.AuditRecord{CallID: "call", AgentRunID: run.State.RunID, TenantID: run.State.Scope.OrganizationID, UserID: run.State.Scope.ActorID, BusinessTaskID: run.State.Request.Binding.ContextID, ToolID: "product.canonical.inspect", ToolVersion: "v1.0.0", StartedAt: time.Now().UTC(), FinishedAt: time.Now().UTC(), Outcome: commercetool.AuditOutcomeSucceeded}
	require.NoError(t, s.RecordToolCall(ctx, audit))
	require.NoError(t, s.RecordToolCall(ctx, audit))
	audit.Outcome = commercetool.AuditOutcomeFailed
	require.Error(t, s.RecordToolCall(ctx, audit))
	audit.CallID = "other"
	audit.TenantID = "other"
	require.Error(t, s.RecordToolCall(ctx, audit))
}

func TestAgentStoreConcurrentStartAndResume(t *testing.T) {
	_, s := storeFixture(t)
	ctx := context.Background()
	initial := initialRecord()
	var winners atomic.Int32
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, acquired, err := s.Claim(ctx, initial, 0)
			if err != nil {
				t.Error(err)
			}
			if acquired {
				winners.Add(1)
			}
		}()
	}
	wg.Wait()
	require.EqualValues(t, 1, winners.Load())
	run, acquired, err := s.Claim(ctx, initial, 0)
	require.NoError(t, err)
	require.False(t, acquired)
	run.State.Phase = agent.Interrupted
	run.Checkpoint = []byte("opaque-eino-checkpoint")
	run.State.Usage = agent.Usage{Steps: 2, ModelCalls: 1, Tokens: 10, CostMicros: 2}
	committed, err := s.Commit(ctx, run, run.State.Revision)
	require.NoError(t, err)
	winners.Store(0)
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			resumed, ok, err := s.Claim(ctx, initial, committed.State.Revision)
			if err != nil {
				require.ErrorIs(t, err, agent.ErrConflict)
				return
			}
			if ok {
				winners.Add(1)
				require.Equal(t, committed.State.Usage, resumed.State.Usage)
				require.Equal(t, committed.Checkpoint, resumed.Checkpoint)
				require.Equal(t, committed.State.Deadline, resumed.State.Deadline)
			}
		}()
	}
	wg.Wait()
	require.EqualValues(t, 1, winners.Load())
}

func TestAgentStoreRestartNeverReexecutesRunningOrTerminal(t *testing.T) {
	db, s := storeFixture(t)
	ctx := context.Background()
	initial := initialRecord()
	run, acquired, err := s.Claim(ctx, initial, 0)
	require.NoError(t, err)
	require.True(t, acquired)
	restarted, err := New(db)
	require.NoError(t, err)
	replayed, acquired, err := restarted.Claim(ctx, initialRecord(), 0)
	require.NoError(t, err)
	require.False(t, acquired)
	require.Equal(t, run, replayed)
	_, _, err = restarted.Claim(ctx, initial, run.State.Revision)
	require.ErrorIs(t, err, agent.ErrConflict)
	run.State.Phase = agent.Stopped
	run.State.StopReason = agent.StopModelUnknown
	run.State.PendingInvocationID = "run:step:1"
	committed, err := s.Commit(ctx, run, run.State.Revision)
	require.NoError(t, err)
	replayed, acquired, err = restarted.Claim(ctx, initial, 0)
	require.NoError(t, err)
	require.False(t, acquired)
	require.Equal(t, committed, replayed)
	_, err = s.Commit(ctx, run, run.State.Revision)
	require.ErrorIs(t, err, agent.ErrConflict)
}

func TestAgentStoreBindingAndAtomicCheckpoint(t *testing.T) {
	_, s := storeFixture(t)
	ctx := context.Background()
	initial := initialRecord()
	run, _, err := s.Claim(ctx, initial, 0)
	require.NoError(t, err)
	changed := initial
	changed.State.Request.Binding.CatalogVersion = "2"
	_, _, err = s.Claim(ctx, changed, 0)
	require.ErrorIs(t, err, agent.ErrConflict)
	changed = run
	changed.State.Scope.OrganizationID = "org-other"
	changed.State.Phase = agent.HumanReviewRequired
	_, err = s.Commit(ctx, changed, run.State.Revision)
	require.ErrorIs(t, err, agent.ErrConflict)
	changed = run
	changed.State.Deadline = changed.State.Deadline.Add(time.Hour)
	changed.State.Phase = agent.HumanReviewRequired
	_, err = s.Commit(ctx, changed, run.State.Revision)
	require.ErrorIs(t, err, agent.ErrConflict)
	changed = run
	changed.State.Phase = agent.Interrupted
	changed.Checkpoint = make([]byte, agent.MaxStateBytes)
	_, err = s.Commit(ctx, changed, run.State.Revision)
	require.ErrorIs(t, err, agent.ErrInvalid)
	ctxCancelled, cancel := context.WithCancel(ctx)
	cancel()
	changed = run
	changed.State.Phase = agent.Interrupted
	changed.Checkpoint = []byte("checkpoint")
	_, err = s.Commit(ctxCancelled, changed, run.State.Revision)
	require.Error(t, err)
	replay, acquired, err := s.Claim(ctx, initial, 0)
	require.NoError(t, err)
	require.False(t, acquired)
	require.Equal(t, run, replay)
	other := initialRecord()
	other.State.Scope.OrganizationID = "org-other"
	_, acquired, err = s.Claim(ctx, other, 0)
	require.NoError(t, err)
	require.True(t, acquired)
}
