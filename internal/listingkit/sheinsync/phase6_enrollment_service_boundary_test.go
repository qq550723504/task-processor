package sheinsync

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSheinEnrollmentServiceFilesOwnSeparatedFamilies(t *testing.T) {
	t.Parallel()

	dir := "."

	rootFile := readSheinEnrollmentServiceFileContent(t, filepath.Join(dir, "enrollment_service.go"))
	executeFile := readSheinEnrollmentServiceFileContent(t, filepath.Join(dir, "enrollment_service_execute.go"))
	candidateFile := readSheinEnrollmentServiceFileContent(t, filepath.Join(dir, "enrollment_service_candidates.go"))
	outcomeFile := readSheinEnrollmentServiceFileContent(t, filepath.Join(dir, "enrollment_service_outcome.go"))

	assertSheinEnrollmentServiceContainsAll(t, rootFile,
		"type SheinEnrollmentService interface {",
		"type SheinEnrollmentRepository interface {",
		"type sheinEnrollmentService struct {",
		"func NewSheinEnrollmentService(",
	)
	assertSheinEnrollmentServiceNotContainsAny(t, rootFile,
		"func (s *sheinEnrollmentService) ExecuteSheinActivityEnrollment(",
		"func (s *sheinEnrollmentService) listCandidates(",
		"func (s *sheinEnrollmentService) persistEnrollmentOutcome(",
	)

	assertSheinEnrollmentServiceContainsAll(t, executeFile,
		"func (s *sheinEnrollmentService) ExecuteAutoSheinActivityEnrollment(",
		"func (s *sheinEnrollmentService) ExecuteSheinActivityEnrollment(",
		"func (s *sheinEnrollmentService) validate(",
		"func (s *sheinEnrollmentService) executeCandidates(",
	)
	assertSheinEnrollmentServiceNotContainsAny(t, executeFile,
		"func (s *sheinEnrollmentService) listCandidates(",
		"func (s *sheinEnrollmentService) persistEnrollmentOutcome(",
	)

	assertSheinEnrollmentServiceContainsAll(t, candidateFile,
		"func (s *sheinEnrollmentService) listCandidates(",
		"func (s *sheinEnrollmentService) listCandidatesByIDs(",
		"func (s *sheinEnrollmentService) listCandidatesByPage(",
		"func filterExecutableSheinCandidates(",
	)
	assertSheinEnrollmentServiceNotContainsAny(t, candidateFile,
		"func (s *sheinEnrollmentService) ExecuteSheinActivityEnrollment(",
		"func (s *sheinEnrollmentService) persistEnrollmentOutcome(",
	)

	assertSheinEnrollmentServiceContainsAll(t, outcomeFile,
		"func (s *sheinEnrollmentService) persistEnrollmentOutcome(",
		"func mapSheinEnrollmentResults(",
		"func buildSheinEnrollmentItems(",
		"func joinSheinEnrollmentErrors(",
	)
	assertSheinEnrollmentServiceNotContainsAny(t, outcomeFile,
		"func (s *sheinEnrollmentService) ExecuteSheinActivityEnrollment(",
		"func (s *sheinEnrollmentService) listCandidates(",
	)
}

func readSheinEnrollmentServiceFileContent(t *testing.T, path string) string {
	t.Helper()

	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(content)
}

func assertSheinEnrollmentServiceContainsAll(t *testing.T, content string, snippets ...string) {
	t.Helper()

	for _, snippet := range snippets {
		if !strings.Contains(content, snippet) {
			t.Fatalf("expected content to contain %q", snippet)
		}
	}
}

func assertSheinEnrollmentServiceNotContainsAny(t *testing.T, content string, snippets ...string) {
	t.Helper()

	for _, snippet := range snippets {
		if strings.Contains(content, snippet) {
			t.Fatalf("expected content to exclude %q", snippet)
		}
	}
}
