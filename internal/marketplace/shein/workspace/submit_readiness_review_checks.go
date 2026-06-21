package workspace

import sheinpub "task-processor/internal/publishing/shein"

// BuildManualNotesReadinessCheck builds the warning check for unresolved manual review notes.
func BuildManualNotesReadinessCheck(pkg *sheinpub.Package) ReadinessCheckSpec {
	var notes []string
	if pkg != nil {
		notes = FilterManualReviewNotes(pkg.ReviewNotes)
	}
	return BuildSubmitReadinessCheck(
		"manual_notes",
		"人工备注",
		len(notes) == 0,
		"仍有人工备注未处理，建议在提交前再次确认",
		[]string{"shein.review_notes"},
		"处理备注",
		true,
	)
}

// BuildSourceFactsReadinessCheck builds the blocking check for source-derived facts.
func BuildSourceFactsReadinessCheck(pkg *sheinpub.Package) ReadinessCheckSpec {
	var metadata map[string]string
	if pkg != nil {
		metadata = pkg.Metadata
	}
	ready, message := SourceFactsReady(metadata)
	return BuildSubmitReadinessCheck(
		"source_facts",
		"来源事实",
		ready,
		message,
		[]string{"shein.metadata.source_fact_review_required", "shein.metadata.source_fact_review_fields"},
		"复核来源事实",
		false,
	)
}
