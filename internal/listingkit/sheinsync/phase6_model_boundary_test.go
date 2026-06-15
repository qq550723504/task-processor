package sheinsync

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSheinModelFilesOwnSeparatedFamilies(t *testing.T) {
	t.Parallel()

	dir := "."

	rootFile := readSheinModelFileContent(t, filepath.Join(dir, "model.go"))
	productFile := readSheinModelFileContent(t, filepath.Join(dir, "model_sync_products.go"))
	jobFile := readSheinModelFileContent(t, filepath.Join(dir, "model_sync_jobs.go"))
	candidateFile := readSheinModelFileContent(t, filepath.Join(dir, "model_candidates.go"))
	enrollmentFile := readSheinModelFileContent(t, filepath.Join(dir, "model_enrollment.go"))

	assertSheinModelContainsAll(t, rootFile,
		"type SheinCostPriceSource string",
		"func sheinFloat64Ptr(v float64) *float64",
	)
	assertSheinModelNotContainsAny(t, rootFile,
		"type SheinSyncedProductRecord struct {",
		"type SheinSyncJobRecord struct {",
		"type SheinActivityCandidateRecord struct {",
		"type SheinActivityEnrollmentRunRecord struct {",
	)

	assertSheinModelContainsAll(t, productFile,
		"type SheinSyncedProductRecord struct {",
		"func (SheinSyncedProductRecord) TableName() string",
		"func ApplyEffectiveCostPrice(record *SheinSyncedProductRecord)",
	)
	assertSheinModelNotContainsAny(t, productFile,
		"type SheinSyncJobRecord struct {",
		"type SheinActivityCandidateRecord struct {",
		"type SheinActivityEnrollmentRunRecord struct {",
	)

	assertSheinModelContainsAll(t, jobFile,
		"type SheinSyncTriggerMode string",
		"type SheinSyncJobStatus string",
		"type SheinSyncJobRecord struct {",
	)
	assertSheinModelNotContainsAny(t, jobFile,
		"type SheinSyncedProductRecord struct {",
		"type SheinActivityCandidateRecord struct {",
		"type SheinActivityEnrollmentRunRecord struct {",
	)

	assertSheinModelContainsAll(t, candidateFile,
		"type SheinCandidateEligibilityStatus string",
		"type SheinCandidateReviewStatus string",
		"type SheinActivityCandidateRecord struct {",
	)
	assertSheinModelNotContainsAny(t, candidateFile,
		"type SheinSyncedProductRecord struct {",
		"type SheinSyncJobRecord struct {",
		"type SheinActivityEnrollmentRunRecord struct {",
	)

	assertSheinModelContainsAll(t, enrollmentFile,
		"type SheinEnrollmentRunTriggerMode string",
		"type SheinEnrollmentRunStatus string",
		"type SheinEnrollmentItemStatus string",
		"type SheinActivityEnrollmentRunRecord struct {",
		"type SheinActivityEnrollmentItemRecord struct {",
	)
	assertSheinModelNotContainsAny(t, enrollmentFile,
		"type SheinSyncedProductRecord struct {",
		"type SheinSyncJobRecord struct {",
		"type SheinActivityCandidateRecord struct {",
	)
}

func readSheinModelFileContent(t *testing.T, path string) string {
	t.Helper()

	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(content)
}

func assertSheinModelContainsAll(t *testing.T, content string, snippets ...string) {
	t.Helper()

	for _, snippet := range snippets {
		if !strings.Contains(content, snippet) {
			t.Fatalf("expected content to contain %q", snippet)
		}
	}
}

func assertSheinModelNotContainsAny(t *testing.T, content string, snippets ...string) {
	t.Helper()

	for _, snippet := range snippets {
		if strings.Contains(content, snippet) {
			t.Fatalf("expected content to exclude %q", snippet)
		}
	}
}
