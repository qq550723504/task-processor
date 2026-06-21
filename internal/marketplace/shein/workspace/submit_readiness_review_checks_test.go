package workspace

import (
	"testing"

	sheinpub "task-processor/internal/publishing/shein"
)

func TestBuildManualNotesReadinessCheckWarnsForManualNotes(t *testing.T) {
	t.Parallel()

	got := BuildManualNotesReadinessCheck(&sheinpub.Package{
		ReviewNotes: []string{"SHEIN 信息需要人工复核"},
	})

	if got.Key != "manual_notes" || got.OK || !got.WarningOnly {
		t.Fatalf("manual notes check = %+v, want warning manual_notes blocker state", got)
	}
	if got.Taxonomy.Severity != "warning" {
		t.Fatalf("taxonomy = %+v, want warning severity", got.Taxonomy)
	}
}

func TestBuildManualNotesReadinessCheckIgnoresAutoNotes(t *testing.T) {
	t.Parallel()

	got := BuildManualNotesReadinessCheck(&sheinpub.Package{
		ReviewNotes: []string{AutoReviewNotes[0]},
	})

	if !got.OK {
		t.Fatalf("manual notes check = %+v, want auto cookie note ignored", got)
	}
}

func TestBuildSourceFactsReadinessCheckBlocksLLMOnly1688Facts(t *testing.T) {
	t.Parallel()

	got := BuildSourceFactsReadinessCheck(&sheinpub.Package{
		Metadata: map[string]string{
			"source_platform":             "1688",
			"source_fact_review_required": "true",
			"source_fact_review_fields":   "material,color",
		},
	})

	if got.Key != "source_facts" || got.OK || got.WarningOnly {
		t.Fatalf("source facts check = %+v, want blocking source_facts", got)
	}
	if got.Message == "" || len(got.FieldPaths) != 2 {
		t.Fatalf("source facts check = %+v, want message and field paths", got)
	}
}
