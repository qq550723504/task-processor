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
