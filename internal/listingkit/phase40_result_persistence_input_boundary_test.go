package listingkit

import (
	"os"
	"strings"
	"testing"
)

func TestResultPersistenceInputBoundary(t *testing.T) {
	t.Parallel()

	submissionSrc, err := os.ReadFile("../listing/submission/result_persistence_service.go")
	if err != nil {
		t.Fatalf("ReadFile(../listing/submission/result_persistence_service.go) error = %v", err)
	}
	submissionContent := string(submissionSrc)

	for _, needle := range []string{
		"func BuildSuccessPersistenceInput[TTask, TResult, TPackage, TResponse any](",
		"func BuildFailurePersistenceInput[TTask, TResult, TPackage, TResponse any](",
	} {
		if !strings.Contains(submissionContent, needle) {
			t.Fatalf("result_persistence_service.go should contain %q", needle)
		}
	}

	for _, file := range []string{
		"task_submission_state_service.go",
		"task_temporal_submission_persistence_service.go",
	} {
		src, err := os.ReadFile(file)
		if err != nil {
			t.Fatalf("ReadFile(%s) error = %v", file, err)
		}
		content := string(src)
		for _, needle := range []string{
			"return submissiondomain.BuildSuccessPersistenceInput(in)",
			"return submissiondomain.BuildFailurePersistenceInput(in)",
		} {
			if !strings.Contains(content, needle) {
				t.Fatalf("%s should delegate result persistence input mapping via %q", file, needle)
			}
		}
	}
}
