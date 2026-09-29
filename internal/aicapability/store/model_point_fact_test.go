package store

import (
	"context"
	"github.com/stretchr/testify/require"
	"task-processor/internal/aicapability"
	"testing"
	"time"
)

func TestModelPointSchemaAndScopedCanonicalFact(t *testing.T) {
	db := newInvocationLedgerDB(t)
	require.NoError(t, VerifyModelPointInvocationSchema(context.Background(), db))
	r := NewGormInvocationRecorder(db)
	fact := aicapability.InvocationRecord{InvocationID: "canonical", TenantID: "org", UserID: "user", MemberID: "member", AgentRunID: "run", InputHash: "input", Operation: aicapability.OperationProductAgentDecision, Outcome: aicapability.InvocationDispatched, StartedAt: time.Now().UTC(), MaximumPromptTokens: 100, MaximumCompletionTokens: 50, PointTariff: aicapability.ModelPointTariff{PriceVersion: "synthetic", InputPointsPerMillionTokens: 1, OutputPointsPerMillionTokens: 1}}
	_, err := r.ClaimInvocation(context.Background(), fact)
	require.NoError(t, err)
	got, err := r.ReadModelInvocation(context.Background(), "org", "canonical")
	require.NoError(t, err)
	require.Equal(t, fact.PointTariff, got.PointTariff)
	_, err = r.ReadModelInvocation(context.Background(), "other", "canonical")
	require.Error(t, err)
	// Synthetic obsolete schema is rejected without changing it.
	require.NoError(t, db.Exec("ALTER TABLE ai_invocations DROP COLUMN point_price_version").Error)
	require.Error(t, VerifyModelPointInvocationSchema(context.Background(), db))
	require.False(t, db.Migrator().HasColumn(&invocationRow{}, "point_price_version"))
}
