package workspace

import "testing"

func TestBuildSubmitReadinessCheckCopiesFieldsAndAttachesTaxonomy(t *testing.T) {
	t.Parallel()

	fieldPaths := []string{"shein.category_id"}
	got := BuildSubmitReadinessCheck(
		"category",
		"类目骨架",
		false,
		"category missing",
		fieldPaths,
		"确认类目",
		false,
	)
	fieldPaths[0] = "mutated"

	if got.Key != "category" || got.Label != "类目骨架" {
		t.Fatalf("check identity = %q/%q, want category label", got.Key, got.Label)
	}
	if got.OK {
		t.Fatal("OK = true, want false")
	}
	if got.FieldPaths[0] != "shein.category_id" {
		t.Fatalf("FieldPaths = %#v, want defensive copy", got.FieldPaths)
	}
	if got.Taxonomy.BlockerKey == "" || got.Taxonomy.Severity == "" {
		t.Fatalf("Taxonomy = %+v, want populated taxonomy", got.Taxonomy)
	}
}

func TestBuildSubmitReadinessCheckUsesWarningTaxonomy(t *testing.T) {
	t.Parallel()

	got := BuildSubmitReadinessCheck("manual_notes", "人工备注", false, "review notes", nil, "处理备注", true)

	if !got.WarningOnly {
		t.Fatal("WarningOnly = false, want true")
	}
	if got.Taxonomy.Severity != "warning" {
		t.Fatalf("Taxonomy severity = %q, want warning", got.Taxonomy.Severity)
	}
}
