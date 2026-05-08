package generation

import (
	"reflect"
	"testing"
)

func TestReviewActionKeyAndLabelForCapability(t *testing.T) {
	t.Parallel()

	if got := ReviewActionKeyForCapability("measurement_preview"); got != ActionReviewMeasurementPreviews {
		t.Fatalf("ReviewActionKeyForCapability() = %q, want %q", got, ActionReviewMeasurementPreviews)
	}
	if got := ReviewActionLabelForCapability("measurement_preview"); got != "Review Measurement Previews" {
		t.Fatalf("ReviewActionLabelForCapability() = %q, want measurement label", got)
	}
	if got := ReviewActionKeyForCapability("unknown"); got != ActionReviewReadyAssets {
		t.Fatalf("ReviewActionKeyForCapability(unknown) = %q, want %q", got, ActionReviewReadyAssets)
	}
	if got := ReviewActionLabelForCapability("unknown"); got != "Review Previews" {
		t.Fatalf("ReviewActionLabelForCapability(unknown) = %q, want fallback label", got)
	}
}

func TestPreviewCapabilitySecondaryActionsFollowsSpecOrder(t *testing.T) {
	t.Parallel()

	actions, actionKeys := PreviewCapabilitySecondaryActions(map[string]int{
		"subject_preview":     2,
		"measurement_preview": 1,
		"badge_preview":       0,
	})

	if want := []string{"Review Measurement Previews", "Review Subject Previews"}; !reflect.DeepEqual(actions, want) {
		t.Fatalf("actions = %+v, want %+v", actions, want)
	}
	if want := []string{ActionReviewMeasurementPreviews, ActionReviewSubjectPreviews}; !reflect.DeepEqual(actionKeys, want) {
		t.Fatalf("actionKeys = %+v, want %+v", actionKeys, want)
	}
}
