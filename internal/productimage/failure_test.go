package productimage

import (
	"fmt"
	"testing"

	"task-processor/internal/infra/clients/grsai"
)

func TestClassifyProcessFailureTreatsModerationAsNoRetry(t *testing.T) {
	err := fmt.Errorf("extract_subject failed: %w", &grsai.JobError{
		Reason: "output_moderation",
		Detail: "blocked by provider moderation",
	})

	if got := ClassifyProcessFailure(err); got != FailureDispositionNoRetry {
		t.Fatalf("ClassifyProcessFailure() = %q, want %q", got, FailureDispositionNoRetry)
	}
}

func TestClassifyProcessFailureKeepsTimeoutRetryable(t *testing.T) {
	err := fmt.Errorf("render_white_bg failed: %w", &grsai.JobError{
		Reason: "error",
		Detail: "google gemini timeout",
	})

	if got := ClassifyProcessFailure(err); got != FailureDispositionRetryable {
		t.Fatalf("ClassifyProcessFailure() = %q, want %q", got, FailureDispositionRetryable)
	}
}

func TestClassifyProcessFailureTreatsAPIKeyErrorsAsNoRetry(t *testing.T) {
	err := fmt.Errorf("render_white_bg failed after 1200ms: apikey error")

	if got := ClassifyProcessFailure(err); got != FailureDispositionNoRetry {
		t.Fatalf("ClassifyProcessFailure() = %q, want %q", got, FailureDispositionNoRetry)
	}
}

func TestClassifyProcessFailureTreatsQuotaErrorsAsNoRetry(t *testing.T) {
	err := fmt.Errorf("render_gallery failed: provider returned insufficient balance")

	if got := ClassifyProcessFailure(err); got != FailureDispositionNoRetry {
		t.Fatalf("ClassifyProcessFailure() = %q, want %q", got, FailureDispositionNoRetry)
	}
}
