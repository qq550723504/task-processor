package batchcapture

import "testing"

// The application's result contract requires `operationId` for every outcome and
// binds a publication to it (web/listingkit-ui/src/lib/contracts/product-acquisition.ts:7-9
// and :33-43). A render that names a status without one is therefore partial or
// drifted, and recording it would either claim a publication nobody can look up or
// write an empty operation id onto an item that did reach the application.
//
// The rule is asserted here rather than only through the browser because the driver
// has to keep waiting until its control timeout; the browser tests prove the end-to-end
// consequence, and this proves the decision itself.
func TestReadableTerminalResultRequiresAnOperationID(t *testing.T) {
	const id = "3f1c8a52-6f5e-4a1d-9c47-8f2a5b0d6e31"
	cases := []struct {
		name        string
		status      string
		operationID string
		want        bool
	}{
		{name: "published with an operation id", status: "published", operationID: id, want: true},
		{name: "failed with an operation id", status: "failed", operationID: id, want: true},
		{name: "outcome_unknown with an operation id", status: "outcome_unknown", operationID: id, want: true},
		{name: "a status this build does not know", status: "something_new", operationID: id, want: true},
		{name: "published without an operation id", status: "published", operationID: "", want: false},
		{name: "failed without an operation id", status: "failed", operationID: "", want: false},
		{name: "published with a placeholder id", status: "published", operationID: "fixture-operation-1", want: false},
		{name: "a status without an operation id", status: "something_new", operationID: "", want: false},
		{name: "no status at all", status: "", operationID: id, want: false},
		{name: "an all-zero operation id", status: "published", operationID: "00000000-0000-0000-0000-000000000000", want: false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := readableTerminalResult(tc.status, tc.operationID); got != tc.want {
				t.Fatalf("readableTerminalResult(%q, %q) = %v, want %v", tc.status, tc.operationID, got, tc.want)
			}
		})
	}
}
