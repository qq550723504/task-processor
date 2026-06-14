package listingkit

import "testing"

func TestSheinSubmitRemoteStateBoundary(t *testing.T) {
	t.Parallel()

	source := readNamedFunctionSource(t, "submit_remote_state_shein.go", "prepareSheinRemoteSubmitState")
	callNames := readNamedFunctionCallNames(t, "submit_remote_state_shein.go", "prepareSheinRemoteSubmitState")

	assertSourceContainsAll(t, source, []string{
		"supplierCode, snapshot := sheinpub.PrepareSubmissionPersistenceInput(",
	})
	assertSourceExcludesAll(t, source, []string{
		"sheinSubmitSupplierCode(",
		"setSheinSubmitSupplierCode(",
		"setSheinSubmitSnapshot(",
		"sheinpub.BuildSubmitSnapshot(",
	})
	assertFunctionCallsContainAll(t, callNames, []string{
		"PrepareSubmissionPersistenceInput",
	})
}
