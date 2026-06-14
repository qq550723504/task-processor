package listingkit

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestGenericSubmissionCallsitesUseListingSubmissionPackage(t *testing.T) {
	t.Parallel()

	files := []string{
		"api/submit_handler.go",
		"service_config_groups.go",
		"service_submit_lease_helper.go",
		"service_submission_collaborators.go",
		"service_submit_contracts.go",
		"service_submit_collaborators.go",
		"shein_submit_readiness.go",
		"submit_remote_attempt_shein.go",
		"task_lifecycle_service.go",
		"task_recovery_durability.go",
		"task_requeue_helpers.go",
		"temporal/client.go",
	}

	for _, rel := range files {
		path := filepath.Join("..", "..", "internal", "listingkit", filepath.FromSlash(rel))
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read %s: %v", path, err)
		}
		content := string(data)
		if strings.Contains(content, "\"task-processor/internal/listingkit/submission\"") {
			t.Fatalf("%s still imports internal/listingkit/submission; use internal/listing/submission for generic submit primitives", path)
		}
	}
}
