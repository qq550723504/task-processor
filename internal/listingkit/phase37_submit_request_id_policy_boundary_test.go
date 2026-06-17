package listingkit

import (
	"os"
	"strings"
	"testing"
)

func TestSubmitRequestIDPolicyBoundary(t *testing.T) {
	t.Parallel()

	submissionSrc, err := os.ReadFile("../listing/submission/submit_request_id.go")
	if err != nil {
		t.Fatalf("ReadFile(../listing/submission/submit_request_id.go) error = %v", err)
	}
	submissionContent := string(submissionSrc)

	for _, needle := range []string{
		"func ResolveSubmitRequestID(idempotencyKey, requestID string) string {",
		"func DeriveWorkflowRequestID(taskID, action string, requestedAt time.Time) string {",
	} {
		if !strings.Contains(submissionContent, needle) {
			t.Fatalf("submit_request_id.go should contain %q", needle)
		}
	}

	sharedSrc, err := os.ReadFile("service_submit_shared.go")
	if err != nil {
		t.Fatalf("ReadFile(service_submit_shared.go) error = %v", err)
	}
	sharedContent := string(sharedSrc)

	needle := "return listingsubmission.ResolveSubmitRequestID(req.IdempotencyKey, req.RequestID)"
	if !strings.Contains(sharedContent, needle) {
		t.Fatalf("service_submit_shared.go should contain DTO adapter delegation %q", needle)
	}
	if strings.Contains(sharedContent, "func derivedSheinSubmitRequestID(") {
		t.Fatal("service_submit_shared.go should not keep an unused workflow request-id wrapper; call internal/listing/submission directly")
	}
}
