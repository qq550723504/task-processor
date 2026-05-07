package submission

import "testing"

func TestSourceFactsReadyBlocks1688LLMOnlyFacts(t *testing.T) {
	t.Parallel()

	ready, message := SourceFactsReady(map[string]string{
		"source_platform":             "1688",
		"source_fact_review_required": "true",
		"source_fact_review_fields":   "selling_points,specifications",
	})

	if ready {
		t.Fatal("expected source facts to block readiness")
	}
	if message == "" {
		t.Fatal("expected review message")
	}
}

func TestSourceFactsReadyAllowsNon1688Sources(t *testing.T) {
	t.Parallel()

	ready, message := SourceFactsReady(map[string]string{
		"source_platform":             "sds",
		"source_fact_review_required": "true",
	})

	if !ready {
		t.Fatalf("ready = false, message = %q", message)
	}
}
