package listingkitownerreconcile

import (
	"flag"
	"testing"
)

func TestParseFlagsKeepsDryRunAsDefaultAndPreservesArtifacts(t *testing.T) {
	fs := flag.NewFlagSet("listingkit-owner-scope-dry-run", flag.ContinueOnError)
	opts := ParseFlagsFrom(fs,
		"-config", "config/runtime.yaml",
		"-output", "report.json",
		"-safe-backfill-output", "safe.sql",
		"-batch-size", "123",
	)
	if opts.Config != "config/runtime.yaml" || opts.Output != "report.json" || opts.SafeBackfillOutput != "safe.sql" || opts.BatchSize != 123 {
		t.Fatalf("options = %+v", opts)
	}
	if opts.Execute || opts.ConfirmReport != "" {
		t.Fatalf("options must default to dry-run: %+v", opts)
	}
}
