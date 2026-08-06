package store

import (
	"context"
	"fmt"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	_ "modernc.org/sqlite"
	"task-processor/internal/aicapability"
)

var sqliteSequence atomic.Uint64

func TestGormInvocationRecorderRoundTripSafeNormalizedMetadata(t *testing.T) {
	db := newInvocationLedgerDB(t)
	recorder := NewGormInvocationRecorder(db)
	startedAt := time.Date(2026, 8, 6, 14, 0, 0, 0, time.FixedZone("UTC+8", 8*60*60))
	finishedAt := startedAt.Add(1500 * time.Millisecond)

	err := recorder.RecordInvocation(context.Background(), aicapability.InvocationRecord{
		InvocationID: " invocation-1 ", ParentInvocationID: " parent-1 ", AgentRunID: " run-1 ", TenantID: " tenant-1 ", UserID: " user-1 ", BusinessTaskID: " task-1 ", TraceID: " trace-1 ",
		Capability: " listingkit.studio.image ", Operation: " image_generate ", RouteMode: " shadow ", RouteOutcome: " shadow_decided ", ProviderID: " openai ", ModelID: " gpt-image-1 ", RequestedRoutingKey: " request-key ", RoutingKey: " route-key ", CredentialReference: " credential-ref ",
		PolicyVersion: " policy-v1 ", ConfigurationVersion: " config-v1 ", PromptKey: " prompt-key ", PromptVersion: " prompt-v1 ", PromptScope: " tenant ", PromptHash: " prompt-hash ",
		StartedAt: startedAt, FinishedAt: finishedAt, Attempt: 2, FallbackIndex: 1, PromptTokens: 10, CompletionTokens: 20, TotalTokens: 30, ImageCount: 2, EstimatedCostMicros: 400, Currency: " usd ",
		Outcome: " succeeded ", ErrorCategory: " ", RouteErrorCategory: " policy_denied ", ErrorCode: " ", ProviderRequestID: " request-1 ", UpstreamJobID: " job-1 ", InputHash: " input-hash ", OutputHash: " output-hash ",
	})
	require.NoError(t, err)

	var row invocationRow
	require.NoError(t, db.Where("invocation_id = ?", "invocation-1").First(&row).Error)
	require.Equal(t, "tenant-1", row.TenantID)
	require.Equal(t, "listingkit.studio.image", row.Capability)
	require.Equal(t, "shadow", row.RouteMode)
	require.Equal(t, "usd", row.Currency)
	require.Equal(t, "policy_denied", row.RouteErrorCategory)
	require.Equal(t, int64(1500), row.LatencyMilliseconds)
	require.Equal(t, time.UTC, row.StartedAt.Location())
	require.Equal(t, time.UTC, row.FinishedAt.Location())
	require.Equal(t, startedAt.UTC(), row.StartedAt)
	require.Equal(t, finishedAt.UTC(), row.FinishedAt)
	require.Equal(t, "request-1", row.ProviderRequestID)
	require.Equal(t, "job-1", row.UpstreamJobID)
}

func TestInvocationLedgerHasNoSensitivePayloadColumns(t *testing.T) {
	db := newInvocationLedgerDB(t)
	columns, err := db.Migrator().ColumnTypes(&invocationRow{})
	require.NoError(t, err)
	present := make(map[string]bool, len(columns))
	for _, column := range columns {
		present[column.Name()] = true
	}
	for _, banned := range []string{"api_key", "prompt", "raw_prompt", "response", "raw_response", "image_bytes", "cookie", "authorization"} {
		require.Falsef(t, present[banned], "sensitive column %q must not exist", banned)
	}
}

func TestGormInvocationRecorderRejectsMissingDatabaseBlankIDAndNegativeCounters(t *testing.T) {
	require.EqualError(t, AutoMigrateInvocationLedger(nil), "ai invocation ledger database is nil")
	require.EqualError(t, (*GormInvocationRecorder)(nil).RecordInvocation(context.Background(), aicapability.InvocationRecord{InvocationID: "x"}), "ai invocation recorder database is nil")

	db := newInvocationLedgerDB(t)
	recorder := NewGormInvocationRecorder(db)
	require.EqualError(t, recorder.RecordInvocation(context.Background(), aicapability.InvocationRecord{}), "invocation_id is required")
	require.EqualError(t, recorder.RecordInvocation(context.Background(), aicapability.InvocationRecord{InvocationID: "x", PromptTokens: -1}), "invocation usage and cost counters must not be negative")
}

func newInvocationLedgerDB(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := fmt.Sprintf("file:ai-invocation-ledger-%d?mode=memory&cache=shared", sqliteSequence.Add(1))
	db, err := gorm.Open(sqlite.Dialector{DriverName: "sqlite", DSN: dsn}, &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, AutoMigrateInvocationLedger(db))
	return db
}
