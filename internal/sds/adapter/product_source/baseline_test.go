package sdspod

import "testing"

func TestEvaluateBaselineTreatsReadyCacheWithUnknownValidationAsReusable(t *testing.T) {
	decision := EvaluateBaseline(BaselineSnapshot{
		CacheStatus:      "ready",
		Version:          SupportedBaselineVersion,
		PayloadState:     BaselinePayloadPresent,
		ValidationStatus: "unknown",
	})
	if !decision.Reusable || decision.Status != "ready" {
		t.Fatalf("decision = %+v", decision)
	}
}

func TestEvaluateBaselineRejectsBaselineCachedWithUnknownValidation(t *testing.T) {
	decision := EvaluateBaseline(BaselineSnapshot{
		CacheStatus:      "baseline_cached",
		Version:          SupportedBaselineVersion,
		PayloadState:     BaselinePayloadPresent,
		ValidationStatus: "unknown",
	})
	if decision.Reusable || decision.ReasonCode != "validation_not_ready" {
		t.Fatalf("decision = %+v", decision)
	}
}
