package accountprofile

import "testing"

func TestMigrationsUseCurrentProfileSchemaBaseline(t *testing.T) {
	if baselineVersion <= 2026092001 {
		t.Fatalf("baselineVersion = %d, want a new baseline after the old profile shape", baselineVersion)
	}
	if auditVersion <= baselineVersion {
		t.Fatalf("auditVersion = %d must follow baselineVersion = %d", auditVersion, baselineVersion)
	}
}
