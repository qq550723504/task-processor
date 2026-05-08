package generation

import (
	"reflect"
	"testing"
)

func TestBuildActionImpactSummarizesItems(t *testing.T) {
	t.Parallel()

	got := BuildActionImpact([]ActionImpactItem{
		{Platform: "shein", QualityGrade: "Good", State: "Failed", Retryable: true},
		{Platform: "shein", QualityGrade: " good ", State: "failed"},
		{Platform: "amazon", QualityGrade: "Needs_Review", State: "Ready", Retryable: true},
		{Platform: "", QualityGrade: "", State: " "},
	})

	if got.MatchedItems != 4 {
		t.Fatalf("MatchedItems = %d, want 4", got.MatchedItems)
	}
	if got.RetryableItems != 2 {
		t.Fatalf("RetryableItems = %d, want 2", got.RetryableItems)
	}
	if want := []string{"shein", "amazon", ""}; !reflect.DeepEqual(got.Platforms, want) {
		t.Fatalf("Platforms = %+v, want %+v", got.Platforms, want)
	}
	if want := []string{"good", "needs_review"}; !reflect.DeepEqual(got.QualityGrades, want) {
		t.Fatalf("QualityGrades = %+v, want %+v", got.QualityGrades, want)
	}
	if want := []string{"failed", "ready"}; !reflect.DeepEqual(got.States, want) {
		t.Fatalf("States = %+v, want %+v", got.States, want)
	}
}

func TestBuildActionImpactHandlesEmptyItems(t *testing.T) {
	t.Parallel()

	got := BuildActionImpact(nil)
	if got.MatchedItems != 0 || got.RetryableItems != 0 || got.Platforms != nil || got.QualityGrades != nil || got.States != nil {
		t.Fatalf("BuildActionImpact(nil) = %+v, want empty impact", got)
	}
}
