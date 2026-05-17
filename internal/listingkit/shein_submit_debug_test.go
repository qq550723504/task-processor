package listingkit

import "testing"

func TestResolveSheinSubmitDebugDumpDirUsesConfiguredValue(t *testing.T) {
	t.Cleanup(SetSheinSubmitDebugDumpDirForTesting("D:/tmp/shein-submit-dumps"))

	if got := resolveSheinSubmitDebugDumpDir(); got != "D:/tmp/shein-submit-dumps" {
		t.Fatalf("resolveSheinSubmitDebugDumpDir() = %q, want configured path", got)
	}
}
