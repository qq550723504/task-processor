package temu

import (
	"errors"
	"testing"
)

func TestClassifyTaskError(t *testing.T) {
	tests := []struct {
		name       string
		err        error
		wantReason string
		wantStage  string
	}{
		{
			name:       "auth expired",
			err:        ErrAuthExpired,
			wantReason: temuTaskReasonAuthExpired,
			wantStage:  temuTaskStageInitStore,
		},
		{
			name:       "duplicate product",
			err:        ErrDuplicateProduct,
			wantReason: temuTaskReasonDuplicateProduct,
			wantStage:  temuTaskStagePublishProduct,
		},
		{
			name:       "retryable error",
			err:        NewRetryableError("temporary timeout", errors.New("timeout")),
			wantReason: temuTaskReasonRetryableFailure,
			wantStage:  temuTaskStagePublishProduct,
		},
		{
			name:       "non retryable fallback",
			err:        NewNonRetryableError("bad data", errors.New("invalid payload")),
			wantReason: temuTaskReasonNonRetryable,
			wantStage:  temuTaskStagePublishProduct,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotReason, gotStage := classifyTaskError(tt.err)
			if gotReason != tt.wantReason {
				t.Fatalf("reason = %q, want %q", gotReason, tt.wantReason)
			}
			if gotStage != tt.wantStage {
				t.Fatalf("stage = %q, want %q", gotStage, tt.wantStage)
			}
		})
	}
}
