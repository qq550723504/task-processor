package submissionpersistence

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"task-processor/internal/listing/submission"
)

func TestTransactionFinalizerParticipatesInOuterCommitAndCannotIssueSendPermit(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	db, _ := openExecutionPostgres(t, ctx)
	require.NoError(t, InstallSchema(db))
	repository, err := NewRepository(db)
	require.NoError(t, err)
	kernel, err := submission.NewExecutionKernel(repository)
	require.NoError(t, err)
	command := executionCommand("org-a", "atomic-result", "product-us-fixture", `{"supplier_sku":"sku-a"}`)
	acquired, err := kernel.Acquire(ctx, command)
	require.NoError(t, err)
	require.NotNil(t, acquired.Permit)
	claim := permitClaim("org-a", acquired.Permit)
	evidence := providerEvidence(submission.ExecutionSucceeded, "receipt-fixture", time.Now().UTC())
	_, err = NewTransactionFinalizer(ctx, db)
	require.ErrorIs(t, err, submission.ErrExecutionUnavailable, "a pool is not an already-open owner transaction")
	for _, commit := range []bool{false, true} {
		tx := db.WithContext(ctx).Begin()
		require.NoError(t, tx.Error)
		var originalPath, currentPath string
		require.NoError(t, tx.Raw("SHOW search_path").Row().Scan(&originalPath))
		bound, err := NewTransactionFinalizer(ctx, tx)
		require.NoError(t, err)
		require.NoError(t, tx.Raw("SHOW search_path").Row().Scan(&currentPath))
		require.Equal(t, originalPath, currentPath, "the finalizer preserves its caller's owner transaction settings")
		finalizer, err := submission.NewExecutionKernel(bound)
		require.NoError(t, err)
		_, err = finalizer.Acquire(ctx, executionCommand("org-a", "uncommitted", "another-target", `{"title":"new"}`))
		require.ErrorIs(t, err, submission.ErrExecutionUnavailable, "only a separately committed Acquire may mint a send permit")
		completed, err := finalizer.Complete(ctx, claim, evidence)
		require.NoError(t, err)
		require.Equal(t, submission.ExecutionSucceeded, completed.Status)
		visible, err := kernel.Get(ctx, claim.Scope, claim.AttemptID)
		require.NoError(t, err)
		require.Equal(t, submission.ExecutionClaimed, visible.Status, "the finalizer never commits its caller's transaction")
		if commit {
			require.NoError(t, tx.Commit().Error)
		} else {
			require.NoError(t, tx.Rollback().Error)
		}
		visible, err = kernel.Get(ctx, claim.Scope, claim.AttemptID)
		require.NoError(t, err)
		want := submission.ExecutionClaimed
		if commit {
			want = submission.ExecutionSucceeded
		}
		require.Equal(t, want, visible.Status)
	}
	replay, err := kernel.Acquire(ctx, command)
	require.NoError(t, err)
	require.Nil(t, replay.Permit)
	require.True(t, replay.Replayed)
}
