package listingkit

import (
	"os"
	"strings"
	"testing"
)

func TestGenerationReviewMappingBoundary(t *testing.T) {
	t.Parallel()

	navigationSource := readGenerationReviewMappingSource(t, "generation_review_navigation_target.go")
	for _, needle := range []string{
		"func generationReviewFocusKey(platform, slot, capability string) string {",
		"func reviewActionKeyForCapability(capability string) string {",
		"func reviewActionLabelForCapability(capability string) string {",
	} {
		if strings.Contains(navigationSource, needle) {
			t.Fatalf("generation_review_navigation_target.go should not keep forwarding helper %q", needle)
		}
	}

	for _, needle := range []string{
		"listinggeneration.ReviewFocusKey(",
		"listinggeneration.ReviewActionKeyForCapability(",
	} {
		if !strings.Contains(navigationSource, needle) {
			t.Fatalf("generation_review_navigation_target.go should call canonical generation helper %q", needle)
		}
	}

	for _, file := range []string{
		"generation_recovery_summary_support.go",
		"generation_resolved_action_summary.go",
		"generation_review_section_support.go",
		"generation_review_toolbar.go",
		"task_generation_review_preview_read.go",
	} {
		source := readGenerationReviewMappingSource(t, file)
		if strings.Contains(source, "generationReviewFocusKey(") || strings.Contains(source, "reviewActionKeyForCapability(") || strings.Contains(source, "reviewActionLabelForCapability(") {
			t.Fatalf("%s should call canonical generation review mapping helpers directly", file)
		}
	}
	for _, file := range []string{
		"generation_resolved_action_summary.go",
		"generation_review_section_support.go",
	} {
		source := readGenerationReviewMappingSource(t, file)
		if !strings.Contains(source, "listinggeneration.ReviewActionLabelForCapability(") {
			t.Fatalf("%s should call canonical review action label helper directly", file)
		}
	}
}

func readGenerationReviewMappingSource(t *testing.T, file string) string {
	t.Helper()
	source, err := os.ReadFile(file)
	if err != nil {
		t.Fatalf("ReadFile(%s) error = %v", file, err)
	}
	return string(source)
}
