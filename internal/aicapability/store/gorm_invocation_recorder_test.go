package store

import (
	"context"
	"errors"
	"fmt"
	"sync"
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

func TestInvocationClaimGrantsOnlyOneDispatch(t *testing.T) {
	db := newInvocationLedgerDB(t)
	recorder := NewGormInvocationRecorder(db)
	record := aicapability.InvocationRecord{InvocationID: "agent-run:step:1", TenantID: "org", UserID: "actor", MemberID: "member", AgentRunID: "agent-run", InputHash: "input", StartedAt: time.Now().UTC(), Outcome: aicapability.InvocationDispatched, Operation: aicapability.OperationProductAgentDecision}
	var winners atomic.Int32
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			acquired, err := recorder.ClaimInvocation(context.Background(), record)
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
	changed := record
	changed.InputHash = "different"
	acquired, err := recorder.ClaimInvocation(context.Background(), changed)
	require.Error(t, err)
	require.False(t, acquired)
	changed = record
	changed.TenantID = "other"
	acquired, err = recorder.ClaimInvocation(context.Background(), changed)
	require.Error(t, err)
	require.False(t, acquired)
	terminal := record
	terminal.Outcome = aicapability.InvocationSucceeded
	terminal.FinishedAt = time.Now().UTC()
	terminal.UsageKnown = true
	terminal.PromptTokens = 3
	terminal.CompletionTokens = 2
	terminal.TotalTokens = 5
	require.NoError(t, recorder.RecordInvocation(context.Background(), terminal))
	acquired, err = recorder.ClaimInvocation(context.Background(), record)
	require.NoError(t, err)
	require.False(t, acquired)
}

func TestAgentObservedInvalidOutputSettlesCurrentUsage(t *testing.T) {
	db := newInvocationLedgerDB(t)
	recorder := NewGormInvocationRecorder(db)
	settler := &recordingInvocationUsageSettler{}
	recorder.SetUsageSettler(settler)
	record := aicapability.InvocationRecord{InvocationID: "agent-invalid", TenantID: "org", UserID: "actor", MemberID: "member", AgentRunID: "run", InputHash: "input", StartedAt: time.Now().UTC(), Outcome: aicapability.InvocationDispatched, Operation: aicapability.OperationProductAgentDecision}
	acquired, err := recorder.ClaimInvocation(context.Background(), record)
	require.NoError(t, err)
	require.True(t, acquired)
	record.Outcome = aicapability.InvocationUsageObservedFailed
	record.FinishedAt = time.Now().UTC()
	record.UsageKnown = true
	record.PromptTokens = 3
	record.CompletionTokens = 2
	record.TotalTokens = 5
	record.ErrorCode = "invalid_agent_output"
	require.NoError(t, recorder.RecordInvocation(context.Background(), record))
	require.Equal(t, 1, settler.settleCalls)
	require.Zero(t, settler.releaseCalls)
}

func TestGormInvocationRecorderRoundTripSafeNormalizedMetadata(t *testing.T) {
	db := newInvocationLedgerDB(t)
	recorder := NewGormInvocationRecorder(db)
	startedAt := time.Date(2026, 8, 6, 14, 0, 0, 0, time.FixedZone("UTC+8", 8*60*60))
	finishedAt := startedAt.Add(1500 * time.Millisecond)

	err := recorder.RecordInvocation(context.Background(), aicapability.InvocationRecord{
		InvocationID: " invocation-1 ", ParentInvocationID: " parent-1 ", AgentRunID: " run-1 ", TenantID: " tenant-1 ", UserID: " user-1 ", MemberID: " member-1 ", BusinessTaskID: " task-1 ", TraceID: " trace-1 ",
		Capability: " product.image.scene ", Operation: " product_image_generate ", RouteMode: " active ", RouteOutcome: " routed ", ProviderID: " openai ", ModelID: " gpt-image-1 ", RequestedRoutingKey: " request-key ", RoutingKey: " route-key ", CredentialReference: " credential-ref ",
		PolicyVersion: " policy-v1 ", ConfigurationVersion: " config-v1 ", PromptKey: " prompt-key ", PromptVersion: " prompt-v1 ", PromptScope: " tenant ", PromptHash: " prompt-hash ",
		StartedAt: startedAt, FinishedAt: finishedAt, Attempt: 2, FallbackIndex: 1, PromptTokens: 10, CompletionTokens: 20, TotalTokens: 30, ImageCount: 2, EstimatedCostMicros: 400, Currency: " usd ",
		Outcome: " succeeded ", ErrorCategory: " ", RouteErrorCategory: " policy_denied ", ErrorCode: " ", ProviderRequestID: " request-1 ", UpstreamJobID: " job-1 ", InputHash: " input-hash ", OutputHash: " output-hash ", CacheStatus: aicapability.CacheStatusHit,
	})
	require.NoError(t, err)

	var row invocationRow
	require.NoError(t, db.Where("invocation_id = ?", "invocation-1").First(&row).Error)
	require.Equal(t, "tenant-1", row.TenantID)
	require.Equal(t, "member-1", row.MemberID)
	require.Equal(t, "product.image.scene", row.Capability)
	require.Equal(t, "active", row.RouteMode)
	require.Equal(t, "usd", row.Currency)
	require.Equal(t, "policy_denied", row.RouteErrorCategory)
	require.Equal(t, int64(1500), row.LatencyMilliseconds)
	require.Equal(t, time.UTC, row.StartedAt.Location())
	require.Equal(t, time.UTC, row.FinishedAt.Location())
	require.Equal(t, startedAt.UTC(), row.StartedAt)
	require.Equal(t, finishedAt.UTC(), row.FinishedAt)
	require.Equal(t, "request-1", row.ProviderRequestID)
	require.Equal(t, "job-1", row.UpstreamJobID)
	require.Equal(t, "hit", row.CacheStatus)
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

func TestGormInvocationRecorderDefaultsBlankCacheStatusToNotApplicable(t *testing.T) {
	db := newInvocationLedgerDB(t)
	recorder := NewGormInvocationRecorder(db)
	require.NoError(t, recorder.RecordInvocation(context.Background(), aicapability.InvocationRecord{InvocationID: "invocation-default-cache"}))

	var row invocationRow
	require.NoError(t, db.Where("invocation_id = ?", "invocation-default-cache").First(&row).Error)
	require.Equal(t, "not_applicable", row.CacheStatus)
}

func TestGormInvocationRecorderDurableDispatchBoundaryIsUpdatedByFinalFact(t *testing.T) {
	db := newInvocationLedgerDB(t)
	recorder := NewGormInvocationRecorder(db)
	started := time.Date(2026, 9, 21, 2, 0, 0, 0, time.UTC)
	record := aicapability.InvocationRecord{InvocationID: "invocation-dispatch", TenantID: "tenant-1", UserID: "user-1", MemberID: "member-1", InputHash: "input-1", StartedAt: started, Outcome: aicapability.InvocationDispatched}
	require.NoError(t, recorder.RecordInvocation(context.Background(), record))
	found, ok, err := recorder.FindInvocation(context.Background(), "tenant-1", "member-1", "invocation-dispatch", "input-1")
	require.NoError(t, err)
	require.True(t, ok)
	require.Equal(t, aicapability.InvocationDispatched, found.Outcome)

	record.FinishedAt = started.Add(time.Second)
	record.Outcome = aicapability.InvocationSucceeded
	require.NoError(t, recorder.RecordInvocation(context.Background(), record))
	found, ok, err = recorder.FindInvocation(context.Background(), "tenant-1", "member-1", "invocation-dispatch", "input-1")
	require.NoError(t, err)
	require.True(t, ok)
	require.Equal(t, aicapability.InvocationSucceeded, found.Outcome)
}

func TestGormInvocationRecorderRetainsReservationForUnknownUsageSuccess(t *testing.T) {
	db := newInvocationLedgerDB(t)
	settler := &recordingInvocationUsageSettler{}
	recorder := NewGormInvocationRecorder(db)
	recorder.SetUsageSettler(settler)

	require.NoError(t, recorder.RecordInvocation(context.Background(), aicapability.InvocationRecord{
		InvocationID: "invocation-unknown-usage",
		TenantID:     "tenant-1",
		UserID:       "user-1",
		MemberID:     "member-1",
		Outcome:      aicapability.InvocationSucceeded,
		UsageKnown:   false,
	}))
	require.Zero(t, settler.releaseCalls)
	require.Zero(t, settler.settleCalls)
}

func TestGormInvocationRecorderRecoversDispatchedFailureWithoutRedispatch(t *testing.T) {
	db := newInvocationLedgerDB(t)
	settler := &recordingInvocationUsageSettler{}
	recorder := NewGormInvocationRecorder(db)
	recorder.SetUsageSettler(settler)
	started := time.Date(2026, 9, 21, 3, 0, 0, 0, time.UTC)
	require.NoError(t, recorder.RecordInvocation(context.Background(), aicapability.InvocationRecord{
		InvocationID: "invocation-recover-failed", TenantID: "tenant-1", UserID: "user-1", MemberID: "member-1", InputHash: "input-1", StartedAt: started, Outcome: aicapability.InvocationDispatched,
	}))
	require.NoError(t, recorder.ResolveDispatchedInvocation(context.Background(), aicapability.InvocationRecord{
		InvocationID: "invocation-recover-failed", TenantID: "tenant-1", MemberID: "member-1", InputHash: "input-1", FinishedAt: started.Add(time.Minute), Outcome: aicapability.InvocationFailed,
	}))
	found, ok, err := recorder.FindInvocation(context.Background(), "tenant-1", "member-1", "invocation-recover-failed", "input-1")
	require.NoError(t, err)
	require.True(t, ok)
	require.Equal(t, aicapability.InvocationFailed, found.Outcome)
	require.Equal(t, 1, settler.releaseCalls)
	require.Zero(t, settler.settleCalls)
}

func TestGormInvocationRecorderRecoversDispatchedSuccessOnlyWithObservedUsage(t *testing.T) {
	db := newInvocationLedgerDB(t)
	settler := &recordingInvocationUsageSettler{}
	recorder := NewGormInvocationRecorder(db)
	recorder.SetUsageSettler(settler)
	started := time.Date(2026, 9, 21, 4, 0, 0, 0, time.UTC)
	require.NoError(t, recorder.RecordInvocation(context.Background(), aicapability.InvocationRecord{
		InvocationID: "invocation-recover-success", TenantID: "tenant-1", UserID: "user-1", MemberID: "member-1", InputHash: "input-2", StartedAt: started, Outcome: aicapability.InvocationDispatched,
	}))
	require.ErrorContains(t, recorder.ResolveDispatchedInvocation(context.Background(), aicapability.InvocationRecord{
		InvocationID: "invocation-recover-success", TenantID: "tenant-1", MemberID: "member-1", InputHash: "input-2", FinishedAt: started.Add(time.Minute), Outcome: aicapability.InvocationSucceeded,
	}), "observed token usage")
	require.NoError(t, recorder.ResolveDispatchedInvocation(context.Background(), aicapability.InvocationRecord{
		InvocationID: "invocation-recover-success", TenantID: "tenant-1", MemberID: "member-1", InputHash: "input-2", FinishedAt: started.Add(time.Minute), Outcome: aicapability.InvocationSucceeded,
		PromptTokens: 7, CompletionTokens: 5, TotalTokens: 12, UsageKnown: true,
	}))
	require.Equal(t, 1, settler.settleCalls)
	require.Zero(t, settler.releaseCalls)
}

func TestGormInvocationRecorderObservedFailedReviewSettlesAndReplaysWithoutRelease(t *testing.T) {
	db := newInvocationLedgerDB(t)
	settler := &recordingInvocationUsageSettler{}
	recorder := NewGormInvocationRecorder(db)
	recorder.SetUsageSettler(settler)
	started := time.Date(2026, 9, 21, 5, 0, 0, 0, time.UTC)
	base := aicapability.InvocationRecord{InvocationID: "review-observed-failed", TenantID: "tenant-1", UserID: "user-1", MemberID: "member-1", InputHash: "input-3", Operation: aicapability.OperationProductImageReview, StartedAt: started, Outcome: aicapability.InvocationDispatched}
	require.NoError(t, recorder.RecordInvocation(context.Background(), base))
	terminal := base
	terminal.FinishedAt = started.Add(time.Minute)
	terminal.Outcome = aicapability.InvocationUsageObservedFailed
	terminal.UsageKnown = true
	terminal.PromptTokens, terminal.CompletionTokens, terminal.TotalTokens = 7, 5, 12
	require.NoError(t, recorder.RecordInvocation(context.Background(), terminal))
	require.Equal(t, 1, settler.settleCalls)
	require.Zero(t, settler.releaseCalls)
	require.NoError(t, recorder.RecordInvocation(context.Background(), terminal))
	require.Equal(t, 2, settler.settleCalls, "commercial owner makes same-fact settlement idempotent")
	require.Zero(t, settler.releaseCalls)
	changedUsage := terminal
	changedUsage.PromptTokens, changedUsage.TotalTokens = 8, 13
	require.ErrorContains(t, recorder.RecordInvocation(context.Background(), changedUsage), "identity conflict")
	changedMetadata := terminal
	changedMetadata.ErrorCode = "different_review_failure"
	require.ErrorContains(t, recorder.RecordInvocation(context.Background(), changedMetadata), "identity conflict")
	found, ok, err := recorder.FindInvocation(context.Background(), "tenant-1", "member-1", "review-observed-failed", "input-3")
	require.NoError(t, err)
	require.True(t, ok)
	require.Equal(t, aicapability.InvocationUsageObservedFailed, found.Outcome)
	require.Equal(t, 12, found.TotalTokens)
	require.Empty(t, found.ErrorCode)
	require.Equal(t, 2, settler.settleCalls, "conflicting replay must not reach commercial owner")
	terminal.Outcome = aicapability.InvocationSucceeded
	require.ErrorContains(t, recorder.RecordInvocation(context.Background(), terminal), "outcome conflict")
}

func TestGormInvocationRecoveryRequiresObservedReviewUsageForFailedOutput(t *testing.T) {
	db := newInvocationLedgerDB(t)
	settler := &recordingInvocationUsageSettler{}
	recorder := NewGormInvocationRecorder(db)
	recorder.SetUsageSettler(settler)
	started := time.Date(2026, 9, 21, 6, 0, 0, 0, time.UTC)
	base := aicapability.InvocationRecord{InvocationID: "review-recover-invalid", TenantID: "tenant-1", UserID: "user-1", MemberID: "member-1", InputHash: "input-4", Operation: aicapability.OperationProductImageReview, StartedAt: started, Outcome: aicapability.InvocationDispatched}
	require.NoError(t, recorder.RecordInvocation(context.Background(), base))
	terminal := aicapability.InvocationRecord{InvocationID: base.InvocationID, TenantID: base.TenantID, MemberID: base.MemberID, InputHash: base.InputHash, FinishedAt: started.Add(time.Minute), Outcome: aicapability.InvocationUsageObservedFailed}
	require.ErrorContains(t, recorder.ResolveDispatchedInvocation(context.Background(), terminal), "observed token usage")
	terminal.UsageKnown = true
	terminal.PromptTokens, terminal.CompletionTokens, terminal.TotalTokens = 7, 5, 12
	require.NoError(t, recorder.ResolveDispatchedInvocation(context.Background(), terminal))
	require.Equal(t, 1, settler.settleCalls)
	require.Zero(t, settler.releaseCalls)
	require.NoError(t, recorder.ResolveDispatchedInvocation(context.Background(), terminal))
	require.Equal(t, 2, settler.settleCalls)
	terminal.Outcome = aicapability.InvocationFailed
	require.ErrorContains(t, recorder.ResolveDispatchedInvocation(context.Background(), terminal), "outcome conflict")
}

func TestObservedFailedReviewSettlementFailureKeepsDurableTerminalForRetry(t *testing.T) {
	db := newInvocationLedgerDB(t)
	settler := &recordingInvocationUsageSettler{settleErr: errors.New("commercial database unavailable")}
	recorder := NewGormInvocationRecorder(db)
	recorder.SetUsageSettler(settler)
	started := time.Date(2026, 9, 21, 7, 0, 0, 0, time.UTC)
	record := aicapability.InvocationRecord{InvocationID: "review-settle-retry", TenantID: "tenant-1", UserID: "user-1", MemberID: "member-1", InputHash: "input-5", Operation: aicapability.OperationProductImageReview, StartedAt: started, Outcome: aicapability.InvocationDispatched}
	require.NoError(t, recorder.RecordInvocation(context.Background(), record))
	record.Outcome = aicapability.InvocationUsageObservedFailed
	record.FinishedAt = started.Add(time.Minute)
	record.UsageKnown = true
	record.PromptTokens, record.CompletionTokens, record.TotalTokens = 7, 5, 12
	require.ErrorContains(t, recorder.RecordInvocation(context.Background(), record), "commercial database unavailable")
	found, ok, err := recorder.FindInvocation(context.Background(), "tenant-1", "member-1", record.InvocationID, record.InputHash)
	require.NoError(t, err)
	require.True(t, ok)
	require.Equal(t, aicapability.InvocationUsageObservedFailed, found.Outcome)
	require.Zero(t, settler.releaseCalls)
	settler.settleErr = nil
	require.NoError(t, recorder.RecordInvocation(context.Background(), found))
	require.Equal(t, 2, settler.settleCalls)
	require.Zero(t, settler.releaseCalls)
}

func TestObservedFailedReviewRequiresExistingDispatchedReview(t *testing.T) {
	db := newInvocationLedgerDB(t)
	settler := &recordingInvocationUsageSettler{}
	recorder := NewGormInvocationRecorder(db)
	recorder.SetUsageSettler(settler)
	terminal := aicapability.InvocationRecord{InvocationID: "unreserved-invalid-review", TenantID: "tenant-1", UserID: "user-1", MemberID: "member-1", Operation: aicapability.OperationProductImageReview, InputHash: "input-6", Outcome: aicapability.InvocationUsageObservedFailed, FinishedAt: time.Now().UTC(), UsageKnown: true, PromptTokens: 7, CompletionTokens: 5, TotalTokens: 12}
	require.ErrorContains(t, recorder.RecordInvocation(context.Background(), terminal), "durable dispatched")
	require.Zero(t, settler.settleCalls)
	var count int64
	require.NoError(t, db.Model(&invocationRow{}).Where("invocation_id = ?", terminal.InvocationID).Count(&count).Error)
	require.Zero(t, count)
	terminal.Outcome = aicapability.InvocationDispatched
	terminal.Operation = aicapability.OperationProductImageSceneGenerate
	terminal.FinishedAt = time.Time{}
	terminal.UsageKnown = false
	terminal.PromptTokens, terminal.CompletionTokens, terminal.TotalTokens = 0, 0, 0
	require.NoError(t, recorder.RecordInvocation(context.Background(), terminal))
	terminal.Outcome = aicapability.InvocationUsageObservedFailed
	terminal.FinishedAt = time.Now().UTC()
	terminal.UsageKnown = true
	terminal.PromptTokens, terminal.CompletionTokens, terminal.TotalTokens = 7, 5, 12
	terminal.Operation = aicapability.OperationProductImageReview
	require.ErrorContains(t, recorder.RecordInvocation(context.Background(), terminal), "identity conflict")
	require.Zero(t, settler.settleCalls)
}

func TestObservedFailedReviewRecoveryPreservesDispatchedProvenance(t *testing.T) {
	db := newInvocationLedgerDB(t)
	recorder := NewGormInvocationRecorder(db)
	started := time.Date(2026, 9, 21, 8, 0, 0, 0, time.UTC)
	base := aicapability.InvocationRecord{
		InvocationID: "review-preserve-provenance", TenantID: "tenant-1", UserID: "user-1", MemberID: "member-1", InputHash: "input-7", Operation: aicapability.OperationProductImageReview,
		AgentRunID: "run-1", BusinessTaskID: "receipt-1", StartedAt: started, Outcome: aicapability.InvocationDispatched,
		ProviderID: "provider-a", ModelID: "review-model-a", RoutingKey: "route-a", CredentialReference: "credential-ref-a", ConfigurationVersion: "config-a", PromptKey: "product-image-review", RouteOutcome: aicapability.RouteOutcomeActive,
	}
	require.NoError(t, recorder.RecordInvocation(context.Background(), base))
	terminal := aicapability.InvocationRecord{InvocationID: base.InvocationID, TenantID: base.TenantID, MemberID: base.MemberID, InputHash: base.InputHash, FinishedAt: started.Add(time.Minute), Outcome: aicapability.InvocationUsageObservedFailed, UsageKnown: true, PromptTokens: 7, CompletionTokens: 5, TotalTokens: 12, ErrorCode: "invalid_review_output"}
	require.NoError(t, recorder.ResolveDispatchedInvocation(context.Background(), terminal))
	found, ok, err := recorder.FindInvocation(context.Background(), base.TenantID, base.MemberID, base.InvocationID, base.InputHash)
	require.NoError(t, err)
	require.True(t, ok)
	require.Equal(t, base.RoutingKey, found.RoutingKey)
	require.Equal(t, base.CredentialReference, found.CredentialReference)
	require.Equal(t, base.ConfigurationVersion, found.ConfigurationVersion)
	require.Equal(t, base.PromptKey, found.PromptKey)
	require.Equal(t, base.RouteOutcome, found.RouteOutcome)
	require.Equal(t, "invalid_review_output", found.ErrorCode)
}

type recordingInvocationUsageSettler struct {
	releaseCalls int
	settleCalls  int
	settleErr    error
}

func (s *recordingInvocationUsageSettler) SettleAIInvocationUsage(context.Context, string, string, string, int64, time.Time) error {
	s.settleCalls++
	return s.settleErr
}

func (s *recordingInvocationUsageSettler) ReserveAIInvocationUsage(context.Context, string, string, string, int64, time.Time) error {
	return nil
}

func (s *recordingInvocationUsageSettler) ReleaseAIInvocationUsage(context.Context, string, string) error {
	s.releaseCalls++
	return nil
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
