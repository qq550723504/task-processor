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

func TestSourceFactsReadyBlocks1688LLMOnlyFactsWithoutFieldList(t *testing.T) {
	t.Parallel()

	ready, message := SourceFactsReady(map[string]string{
		"source_platform":             " 1688 ",
		"source_fact_review_required": " TRUE ",
	})

	if ready {
		t.Fatal("expected source facts to block readiness")
	}
	if message != "1688 来源商品包含缺少抓取依据的 LLM 推断字段，提交前必须复核" {
		t.Fatalf("message = %q, want generic review message", message)
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

func TestSourceFactsReadyAllows1688WithScrapedEvidence(t *testing.T) {
	t.Parallel()

	ready, message := SourceFactsReady(map[string]string{
		"source_platform":             "1688",
		"source_fact_review_required": "false",
	})

	if !ready {
		t.Fatalf("ready = false, message = %q", message)
	}
	if message != "1688 来源事实已具备抓取依据" {
		t.Fatalf("message = %q, want evidence-ready message", message)
	}
}
