package productimage

import (
	"errors"
	"fmt"
	"testing"

	"task-processor/internal/aicapability"
	"task-processor/internal/infra/clients/grsai"
	"task-processor/internal/shared/aiidentity"
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

func TestClassifyProcessFailureTreatsCapabilityPolicyDenialAsNoRetry(t *testing.T) {
	err := aicapability.NewError(aicapability.ErrorPolicyDenied, string(aicapability.OperationProductImageSceneGenerate), nil)

	if got := ClassifyProcessFailure(err); got != FailureDispositionNoRetry {
		t.Fatalf("ClassifyProcessFailure() = %q, want %q", got, FailureDispositionNoRetry)
	}
}

func TestClassifyProcessFailureTreatsIdentityIntegrityAsNoRetry(t *testing.T) {
	tests := []struct {
		name string
		err  error
	}{
		{name: "missing envelope", err: aiidentity.ErrMissingIdentity},
		{name: "malformed envelope", err: fmt.Errorf("restore: %w", aiidentity.ErrIdentityIntegrity)},
		{name: "governed capability rejection", err: aicapability.NewError(aicapability.ErrorIdentityIntegrity, string(aicapability.OperationProductImageSceneGenerate), nil)},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ClassifyProcessFailure(tt.err); got != FailureDispositionNoRetry {
				t.Fatalf("ClassifyProcessFailure() = %q, want %q", got, FailureDispositionNoRetry)
			}
			if !IsIdentityIntegrityError(tt.err) {
				t.Fatalf("IsIdentityIntegrityError(%v) = false, want true", tt.err)
			}
		})
	}

	if IsIdentityIntegrityError(errors.New("provider unavailable")) {
		t.Fatal("ordinary provider failure classified as identity integrity")
	}
}
