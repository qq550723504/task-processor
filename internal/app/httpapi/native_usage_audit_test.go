package httpapi

import (
	"context"
	"fmt"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"path/filepath"
	"task-processor/internal/aicapability"
	aistore "task-processor/internal/aicapability/store"
	"testing"
	"time"
)

func TestNativeUsageAuditCoverageRequiresBothCurrentNamespaces(t *testing.T) {
	image, product := &gorm.DB{}, &gorm.DB{}
	for _, test := range []struct {
		name     string
		sources  invocationAuditSources
		complete bool
	}{
		{name: "nil"},
		{name: "empty", sources: invocationAuditSources{}},
		{name: "image only", sources: invocationAuditSources{"image": image}},
		{name: "product only", sources: invocationAuditSources{"product": product}},
		{name: "nil image", sources: invocationAuditSources{"image": nil, "product": product}},
		{name: "both", sources: invocationAuditSources{"image": image, "product": product}, complete: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			require.Equal(t, test.complete, (aiUsageAuditReader{sources: test.sources}).Complete())
		})
	}
}

func TestNativeUsageAuditMergesIndependentOwnersWithStableScopedPages(t *testing.T) {
	sources := invocationAuditSources{}
	now := time.Now().UTC().Truncate(time.Microsecond)
	for _, namespace := range []string{"image", "product"} {
		db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), namespace+".db")), &gorm.Config{})
		require.NoError(t, err)
		pool, err := db.DB()
		require.NoError(t, err)
		t.Cleanup(func() { pool.Close() })
		require.NoError(t, aistore.AutoMigrateInvocationLedger(db))
		sources[namespace] = db
		recorder := aistore.NewGormInvocationRecorder(db)
		for _, id := range []string{"a", "z", "other"} {
			org := "org"
			if id == "other" {
				org = "other"
			}
			fact := aicapability.InvocationRecord{InvocationID: id, TenantID: org, UserID: "actor", MemberID: "member", InputHash: "input", StartedAt: now.Add(-time.Second), FinishedAt: now, Operation: aicapability.OperationProductAgentDecision, Outcome: aicapability.InvocationSucceeded, UsageKnown: true, PromptTokens: 2, CompletionTokens: 3, TotalTokens: 5}
			require.NoError(t, recorder.RecordInvocation(context.Background(), fact))
		}
	}
	reader := aiUsageAuditReader{sources: sources}
	first, err := reader.ListObservedAIUsageAudit(context.Background(), "org", 2, nil)
	require.NoError(t, err)
	require.Len(t, first.Items, 2)
	require.Equal(t, "product:z", first.Items[0].EventID)
	require.Equal(t, "product:a", first.Items[1].EventID)
	require.NotNil(t, first.Next)
	second, err := reader.ListObservedAIUsageAudit(context.Background(), "org", 2, first.Next)
	require.NoError(t, err)
	require.Len(t, second.Items, 2)
	require.Equal(t, "image:z", second.Items[0].EventID)
	require.Equal(t, "image:a", second.Items[1].EventID)
	require.Nil(t, second.Next)
	for _, page := range [][]string{{first.Items[0].OrganizationID, first.Items[1].OrganizationID}, {second.Items[0].OrganizationID, second.Items[1].OrganizationID}} {
		require.Equal(t, []string{"org", "org"}, page)
	}
	empty, err := reader.ListObservedAIUsageAudit(context.Background(), "empty", 2, nil)
	require.NoError(t, err)
	require.Empty(t, empty.Items)
}

func TestNativeUsageAuditReadsPastOwnerFiftyRowLimit(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "usage.db")), &gorm.Config{})
	require.NoError(t, err)
	pool, err := db.DB()
	require.NoError(t, err)
	t.Cleanup(func() { pool.Close() })
	require.NoError(t, aistore.AutoMigrateInvocationLedger(db))
	recorder := aistore.NewGormInvocationRecorder(db)
	now := time.Now().UTC().Truncate(time.Microsecond)
	for index := 0; index < 51; index++ {
		fact := aicapability.InvocationRecord{InvocationID: fmt.Sprintf("inv-%03d", index), TenantID: "org", UserID: "actor", MemberID: "member", InputHash: "input", StartedAt: now.Add(-2 * time.Hour), FinishedAt: now.Add(time.Duration(index) * time.Second), Operation: aicapability.OperationProductAgentDecision, Outcome: aicapability.InvocationSucceeded, UsageKnown: true, PromptTokens: 2, CompletionTokens: 3, TotalTokens: 5}
		require.NoError(t, recorder.RecordInvocation(context.Background(), fact))
	}
	reader := aiUsageAuditReader{sources: invocationAuditSources{"product": db}}
	page, err := reader.ListObservedAIUsageAudit(context.Background(), "org", 100, nil)
	require.NoError(t, err)
	require.Len(t, page.Items, 51)
	require.Nil(t, page.Next)
	require.Equal(t, "product:inv-050", page.Items[0].EventID)
	require.Equal(t, "product:inv-000", page.Items[50].EventID)
}
