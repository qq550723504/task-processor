package listingkit

import "testing"

func TestSheinSubmissionRecoveryBoundary(t *testing.T) {
	t.Parallel()

	recoverSource := readNamedFunctionSource(t, "task_submission_recovery_service.go", "recoverSheinSubmitRemote")
	assertSourceContainsAll(t, recoverSource, []string{
		"return s.recoveryRouteRunner.Recover(ctx, recoveredState)",
	})
	assertSourceExcludesAll(t, recoverSource, []string{
		"executeRecoveredSheinSubmitRoute(",
		"SubmissionResponseAcceptedForAction(",
		"recoverSheinSubmitLocally(",
		"recoverSheinSubmitViaRemoteConfirmation(",
	})

	localRouteSource := readNamedFunctionSource(t, "task_submission_recovery_service_route_support.go", "shouldRecoverLocally")
	assertSourceContainsAll(t, localRouteSource, []string{
		"sheinmarketpub.ResponseAcceptedForAction(",
	})
	assertSourceExcludesAll(t, localRouteSource, []string{
		"sheinpub.SubmissionResponseAcceptedForAction(",
	})

	localSource := readNamedFunctionSource(t, "task_submission_recovery_service_route_support.go", "recoverSheinSubmitLocally")
	assertSourceContainsAll(t, localSource, []string{
		"persistSheinRemoteCompletionSuccess(",
		"finalizeRecoveredSheinSubmission(",
	})

	remoteSource := readNamedFunctionSource(t, "task_submission_recovery_service_route_support.go", "recoverSheinSubmitViaRemoteConfirmation")
	assertSourceContainsAll(t, remoteSource, []string{
		"return s.remoteRefreshRunner.Refresh(ctx, state)",
	})

	runnerSource := readNamedFunctionSource(t, "task_submission_recovery_service.go", "newTaskSubmissionRecoveryService")
	assertSourceContainsAll(t, runnerSource, []string{
		"FinishError: func(ctx context.Context, state *sheinRecoveredRemoteState, remoteErr error) (*ListingKitPreview, error) {",
		"return nil, service.persistSheinRecoveredRemoteFailure(ctx, state, remoteErr)",
	})
	assertSourceExcludesAll(t, runnerSource, []string{
		"FinishError:  service.finishRecoveredRemoteRefreshError,",
	})

	durabilitySource := readNamedFunctionSource(t, "task_recovery_durability.go", "restoreRecoveryDurability")
	assertSourceContainsAll(t, durabilitySource, []string{
		"submissiondomain.BuildRecoveryDurabilityRestoreBlock(",
	})
	assertSourceExcludesAll(t, durabilitySource, []string{
		"cloneRetryableBlock(previousBlock)",
		"classifyRetryableTaskFailure(submitErr)",
		"restoreBlock.RecoveryScope = submissiondomain.RetryableRecoveryScopeTask",
		"restoreBlock.BlockedAt = s.currentTime()",
	})

	durabilitySource = readNamedFunctionSource(t, "task_recovery_durability.go", "restoreRecoveryDurability")
	backfillSource := readNamedFunctionSource(t, "task_recovery_backfill.go", "backfillRetryableBlockedTasks")
	recoveredSubmitSource := readNamedFunctionSource(t, "task_recovery_service.go", "submitRecoveredTask")
	failurePersistenceSource := readNamedFunctionSource(t, "task_result_support.go", "persistClassifiedTaskFailure")
	for _, source := range []string{durabilitySource, backfillSource, recoveredSubmitSource, failurePersistenceSource} {
		assertSourceContainsAll(t, source, []string{
			"markTaskBlockedRetryableState(",
		})
		assertSourceExcludesAll(t, source, []string{
			"adaptSubmissionRetryableBlock(block)",
			"adaptSubmissionRetryableBlock(restoreBlock)",
		})
	}
}
