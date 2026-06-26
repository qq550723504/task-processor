package listingkit

import (
	"os"
	"strings"
	"testing"
)

func TestSheinSubmitRetryWiringCallsPublishingDirectly(t *testing.T) {
	t.Parallel()

	assertFileAbsent(t, "shein_submit_retry.go")

	for _, path := range []string{
		"service_submit_managed_wiring_support.go",
		"service_submit_temporal_wiring_support.go",
	} {
		path := path
		t.Run(path, func(t *testing.T) {
			t.Parallel()

			src, err := os.ReadFile(path)
			if err != nil {
				t.Fatalf("ReadFile(%s) error = %v", path, err)
			}
			content := string(src)
			for _, needle := range []string{
				"retrySheinSensitiveWordSubmit: func(",
				"return sheinpub.RetrySensitiveWordSubmit(",
				"s.taskSubmissionExecutionOrDefault().executeSheinSubmitRemote",
			} {
				if !strings.Contains(content, needle) {
					t.Fatalf("%s should contain %q", path, needle)
				}
			}
			if strings.Contains(content, "s.retrySheinSensitiveWordSubmit") {
				t.Fatalf("%s should not use service retry wrapper", path)
			}
		})
	}

	src, err := os.ReadFile("../publishing/shein/submit_sensitive_retry.go")
	if err != nil {
		t.Fatalf("ReadFile(submit_sensitive_retry.go) error = %v", err)
	}
	content := string(src)
	if !strings.Contains(content, "sheinmarketpub.ShouldRetrySensitiveWordSubmit(") {
		t.Fatal("sensitive-word retry eligibility should live in marketplace publishing policy")
	}
	if strings.Contains(content, "action != listingsubmission.SubmitActionPublish || response == nil || responseErr == nil") {
		t.Fatal("sensitive-word retry eligibility should not remain inline in legacy publishing orchestration")
	}
}
