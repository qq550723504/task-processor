package listingkitownerexceptions

import (
	"context"
	"os"
	"strings"
	"testing"
)

func TestRunWithDependenciesRejectsUnapprovedReportBeforeDatabaseAccess(t *testing.T) {
	path := t.TempDir() + "\\report.json"
	if err := os.WriteFile(path, []byte(`{"report_fingerprint":"deadbeefdead","summary":{"finding_groups":0,"unresolved_rows":0},"findings":[]}`), 0600); err != nil {
		t.Fatal(err)
	}

	err := runWithDependencies(context.Background(), Options{Report: path, ConfirmReport: "648cdfab03c4"}, runtimeDependencies{})
	if err == nil || !strings.Contains(err.Error(), "approved") {
		t.Fatalf("err = %v, want approved report validation error", err)
	}
}
