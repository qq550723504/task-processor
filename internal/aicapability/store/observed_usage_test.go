package store

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"task-processor/internal/aicapability"
)

func TestObservedUsageReadsNativeFactsWithStableScope(t *testing.T) {
	db := newInvocationLedgerDB(t)
	recorder := NewGormInvocationRecorder(db)
	now := time.Now().UTC().Truncate(time.Microsecond)
	for _, row := range []struct {
		org, id string
		known   bool
		outcome aicapability.InvocationOutcome
	}{{"A", "z", true, aicapability.InvocationSucceeded}, {"B", "a", true, aicapability.InvocationSucceeded}, {"B", "b", true, aicapability.InvocationUsageObservedFailed}, {"B", "unknown", false, aicapability.InvocationSucceeded}} {
		record := aicapability.InvocationRecord{TenantID: row.org, UserID: "actor", MemberID: "member", InvocationID: row.id, AgentRunID: "run", InputHash: "input", Operation: aicapability.OperationProductAgentDecision, Outcome: aicapability.InvocationDispatched, StartedAt: now.Add(-time.Second)}
		won, err := recorder.ClaimInvocation(context.Background(), record)
		require.NoError(t, err)
		require.True(t, won)
		record.Outcome, record.FinishedAt, record.UsageKnown = row.outcome, now, row.known
		if row.known {
			record.PromptTokens = 3
			record.CompletionTokens = 2
			record.TotalTokens = 5
		}
		require.NoError(t, recorder.RecordInvocation(context.Background(), record))
	}
	first, err := recorder.ListObservedUsage(context.Background(), "B", "product", 1, nil)
	require.NoError(t, err)
	require.Len(t, first.Items, 1)
	require.Equal(t, "b", first.Items[0].InvocationID)
	require.EqualValues(t, 5, first.Items[0].Tokens)
	require.NotNil(t, first.Next)
	next, err := recorder.ListObservedUsage(context.Background(), "B", "product", 1, first.Next)
	require.NoError(t, err)
	require.Len(t, next.Items, 1)
	require.Equal(t, "a", next.Items[0].InvocationID)
	require.Nil(t, next.Next)
	_, err = recorder.ListObservedUsage(context.Background(), "B", "product", 51, nil)
	require.Error(t, err)
	empty, err := recorder.ListObservedUsage(context.Background(), "empty", "image", 10, nil)
	require.NoError(t, err)
	require.Empty(t, empty.Items)
}

func TestObservedUsageRejectsIncompleteEmptyLedgerSchema(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.Exec(`CREATE TABLE ai_invocations (
		invocation_id TEXT, tenant_id TEXT, usage_known BOOLEAN, outcome TEXT,
		total_tokens INTEGER, member_id TEXT, finished_at DATETIME,
		completion_tokens INTEGER
	)`).Error)
	_, err = NewGormInvocationRecorder(db).ListObservedUsage(context.Background(), "unused", "image", 1, nil)
	require.Error(t, err, "a missing prompt_tokens column must not look like complete empty usage history")
}
