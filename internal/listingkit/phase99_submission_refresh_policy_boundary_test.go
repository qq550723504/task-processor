package listingkit

import "testing"

func TestSheinSubmissionRefreshFallbackPolicyBoundary(t *testing.T) {
	t.Parallel()

	source := readNamedFunctionSource(t, "task_submission_refresh_service.go", "resolveSheinSubmitRemoteStatus")
	assertSourceContainsAll(t, source, []string{
		"sheinpub.ResolveSubmissionRemoteRefreshFallbackMessage(",
	})
	assertSourceExcludesAll(t, source, []string{
		"sheinmarketpub.ResolveRemoteRefreshFallbackMessage(",
		"sheinpub.ResolveSubmissionRefreshFallbackMessage(",
	})
}

func TestSheinSubmissionRemoteLookupPolicyBoundary(t *testing.T) {
	t.Parallel()

	refreshFileSource := readTaskGenerationSourceFile(t, "task_submission_refresh_selection.go")
	assertSourceExcludesAll(t, refreshFileSource, []string{
		"type sheinSubmissionRefreshRemoteInputs = sheinpub.SubmissionRemoteLookupInputs",
		"func buildSubmissionRefreshRemoteInputs(",
		"sheinmarketpub.BuildRemoteConfirmationPolicy(",
		"sheinpub.BuildSubmissionRemoteLookupInputs(",
		"submissiondomain.BuildRefreshRemotePolicy(",
		"sheinpub.BuildSubmissionRefreshLookupInputs(",
	})
	refreshServiceSource := readTaskGenerationSourceFile(t, "task_submission_refresh_service.go")
	assertSourceExcludesAll(t, refreshServiceSource, []string{
		"copySheinRemoteStatusRequest(",
	})

	refreshRequestSource := readNamedFunctionSource(t, "task_submission_refresh_selection.go", "buildSubmissionRefreshRequest")
	assertSourceContainsAll(t, refreshRequestSource, []string{
		"submissiondomain.ResolveRefreshRequestID(",
		"sheinpub.BuildSubmissionRefreshRemoteLookupInputs(",
	})
	assertSourceExcludesAll(t, refreshRequestSource, []string{
		"sheinpub.BuildSubmissionRefreshRequest(",
	})

	recoverySource := readNamedFunctionSource(t, "task_submission_recovery_remote.go", "buildSheinRemoteStatusRequest")
	assertSourceContainsAll(t, recoverySource, []string{
		"sheinpub.BuildSubmissionRecoveryRemoteLookupInputs(",
	})
	assertSourceExcludesAll(t, recoverySource, []string{
		"sheinpub.BuildSubmissionRecoveryLookupInputs(",
	})

	recoveryFileSource := readTaskGenerationSourceFile(t, "task_submission_recovery_remote.go")
	assertSourceExcludesAll(t, recoveryFileSource, []string{
		"type sheinRemoteRefreshState = sheinpub.SubmissionRemoteLookupInputs",
		"func buildSheinRemoteRefreshState(",
	})
}

func TestSheinSubmissionConfirmRemotePolicyBoundary(t *testing.T) {
	t.Parallel()

	source := readNamedFunctionSource(t, "task_submission_recovery_remote.go", "resolveSheinSubmitRemoteStatus")
	assertSourceContainsAll(t, source, []string{
		"sheinpub.BuildSubmissionConfirmRemoteUpdateFromResolution(",
	})
	assertSourceExcludesAll(t, source, []string{
		"sheinmarketpub.ResolveRemoteConfirmationDecision(",
		"sheinmarketpub.ResolveRemoteConfirmationUpdateMessage(",
		"sheinmarketpub.ResolveRemoteResolutionSPUName(",
		"sheinpub.ResolveSubmissionConfirmRemoteUpdate(",
	})

	remoteSource := readTaskGenerationSourceFile(t, "task_submission_recovery_remote.go")
	assertSourceExcludesAll(t, remoteSource, []string{
		"type sheinRemoteConfirmation = sheinpub.SubmissionConfirmRemoteUpdate",
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
		"type sheinSubmissionRefreshValidationRequest struct",
		"func buildSubmissionRefreshValidationRequest(",
		"func loadSubmissionRefreshMutationPackage(",
		"func loadSubmissionRefreshPackageState(",
		"func validateSubmissionRefreshAction(",
		"func validateSubmissionRefreshRequest(",
		"func submissionRefreshActionMatches(",
		"func submissionRefreshRequestMatches(",
	})
}
