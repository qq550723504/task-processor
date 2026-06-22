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
}
