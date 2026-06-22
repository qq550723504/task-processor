package listingkit

import (
	"os"
	"strings"
	"testing"
)

func TestSheinReviewStateBoundary(t *testing.T) {
	t.Parallel()
	fileSource, err := os.ReadFile("workflow_review_state.go")
	if err != nil {
		t.Fatalf("ReadFile(workflow_review_state.go) error = %v", err)
	}
	if !strings.Contains(string(fileSource), `sheinworkspace "task-processor/internal/marketplace/shein/workspace"`) {
		t.Fatal("workflow_review_state.go should call marketplace SHEIN workspace directly")
	}
	assertFileAbsent(t, "workspace/shein/review_bridge.go")

	t.Run("inspection review flow calls workspace directly", func(t *testing.T) {
		t.Parallel()

		source := readNamedFunctionSource(t, "workflow_review_state.go", "applySheinInspectionReviewToSummary")
		callNames := readNamedFunctionCallNames(t, "workflow_review_state.go", "applySheinInspectionReviewToSummary")

		assertSourceContainsAll(t, source, []string{
			"reasons := sheinworkspace.InspectionReviewReasons(result.Shein)",
		})
		assertSourceExcludesAll(t, source, []string{
			"sheinInspectionReviewReasons(result)",
			"result.Shein.Inspection.Summary",
			"result.Shein.ReviewNotes",
			"SHEIN 信息需要人工复核",
		})
		assertFunctionCallsContainAll(t, callNames, []string{
			"InspectionReviewReasons",
		})
	})

	t.Run("cookie review flow calls workspace directly", func(t *testing.T) {
		t.Parallel()

		workflowContent := string(fileSource)
		assertSourceContainsAll(t, workflowContent, []string{
			"cookieNotes := sheinworkspace.CookieUnavailableReviewNotes(result.Shein)",
			"sheinworkspace.IsCookieUnavailableText(reason)",
		})
		assertSourceExcludesAll(t, workflowContent, []string{
			"func sheinCookieUnavailableReviewNotes(",
			"func stripSheinCookieUnavailableReviewNotes(",
			"func filterOutSheinCookieUnavailableReviewNotes(",
			"func sheinCookieUnavailable(",
			"func isSheinCookieUnavailableText(",
			"pkg.CategoryResolution.ReviewNotes",
			"pkg.AttributeResolution.ReviewNotes",
			"pkg.SaleAttributeResolution.ReviewNotes",
			`strings.Contains(text, "cookie 不可用")`,
			`strings.Contains(text, "店铺 cookie")`,
		})

		taskResultSource := readNamedFunctionSource(t, "task_result_support.go", "refreshSheinTaskResultState")
		assertSourceContainsAll(t, taskResultSource, []string{
			"sheinworkspace.StripCookieUnavailableReviewNotes(result.Shein)",
			"result.Summary.Warnings = sheinworkspace.FilterOutCookieUnavailableReviewNotes(result.Summary.Warnings)",
		})

		readinessSource := readNamedFunctionSource(t, "shein_submit_readiness_checks_support.go", "buildSheinSubmitReadinessChecks")
		assertSourceContainsAll(t, readinessSource, []string{
			"!sheinworkspace.HasCookieUnavailableReviewNotes(pkg)",
		})
	})
}
