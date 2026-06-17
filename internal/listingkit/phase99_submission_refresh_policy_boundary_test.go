package listingkit

import "testing"

func TestSheinSubmissionRefreshFallbackPolicyBoundary(t *testing.T) {
	t.Parallel()

	source := readNamedFunctionSource(t, "task_submission_refresh_service.go", "resolveSheinSubmitRemoteStatus")
	assertSourceContainsAll(t, source, []string{
		"sheinmarketpub.ResolveRemoteRefreshFallbackMessage(",
	})
	assertSourceExcludesAll(t, source, []string{
		"sheinpub.ResolveSubmissionRefreshFallbackMessage(",
	})
}

func TestSheinSubmissionRemoteLookupPolicyBoundary(t *testing.T) {
	t.Parallel()

	refreshSource := readNamedFunctionSource(t, "task_submission_refresh_selection.go", "buildSubmissionRefreshRemoteInputs")
	assertSourceContainsAll(t, refreshSource, []string{
		"submissiondomain.BuildRefreshRemotePolicy(",
		"sheinpub.BuildSubmissionRemoteLookupInputs(",
	})
	assertSourceExcludesAll(t, refreshSource, []string{
		"sheinpub.BuildSubmissionRefreshLookupInputs(",
	})

	refreshRequestSource := readNamedFunctionSource(t, "task_submission_refresh_selection.go", "buildSubmissionRefreshRequest")
	assertSourceContainsAll(t, refreshRequestSource, []string{
		"submissiondomain.ResolveRefreshRequestID(",
		"buildSubmissionRefreshRemoteInputs(",
	})
	assertSourceExcludesAll(t, refreshRequestSource, []string{
		"sheinpub.BuildSubmissionRefreshRequest(",
	})

	recoverySource := readNamedFunctionSource(t, "task_submission_recovery_remote.go", "buildSheinRemoteRefreshState")
	assertSourceContainsAll(t, recoverySource, []string{
		"sheinmarketpub.BuildRemoteConfirmationPolicy(",
		"sheinpub.BuildSubmissionRemoteLookupInputs(",
	})
	assertSourceExcludesAll(t, recoverySource, []string{
		"sheinpub.BuildSubmissionRecoveryLookupInputs(",
	})
}

func TestSheinSubmissionConfirmRemotePolicyBoundary(t *testing.T) {
	t.Parallel()

	source := readNamedFunctionSource(t, "task_submission_recovery_remote.go", "resolveSheinSubmitRemoteStatus")
	assertSourceContainsAll(t, source, []string{
		"sheinmarketpub.ResolveRemoteConfirmationDecision(",
		"sheinmarketpub.ResolveRemoteConfirmationUpdateMessage(",
	})
	assertSourceExcludesAll(t, source, []string{
		"sheinpub.ResolveSubmissionConfirmRemoteUpdate(",
	})
}

func TestSheinSubmissionRefreshMutationValidationBoundary(t *testing.T) {
	t.Parallel()

	source := readTaskGenerationSourceFile(t, "task_submission_refresh_mutation.go")
	assertSourceContainsAll(t, source, []string{
		"submissiondomain.RefreshActionMatches(",
		"submissiondomain.RefreshRequestMatches(",
	})
	assertSourceExcludesAll(t, source, []string{
		"sheinpub.ResolveSubmissionRefreshValidation(",
		"func submissionRefreshActionMatches(",
		"func submissionRefreshRequestMatches(",
	})
}
