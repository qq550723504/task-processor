package pipeline

import (
	"errors"
	"strings"
	"testing"

	amazonModel "task-processor/internal/amazon/model"
)

func TestNormalizeTaskStatusErrorMarksValidationAsNonRetryable(t *testing.T) {
	err := normalizeTaskStatusError(
		amazonTaskStageValidateInput,
		amazonModel.NewValidationError("title", "", "标题不能为空"),
	)
	if err == nil {
		t.Fatal("expected error")
	}

	got := err.Error()
	if !strings.HasPrefix(got, "NONRETRYABLE: ") {
		t.Fatalf("expected nonretryable prefix, got %q", got)
	}
	if !strings.Contains(got, "[stage:"+amazonTaskStageValidateInput+"]") {
		t.Fatalf("expected stage marker, got %q", got)
	}
	if !strings.Contains(got, "["+amazonTaskReasonValidationFailed+"]") {
		t.Fatalf("expected validation reason, got %q", got)
	}
}

func TestNormalizeTaskStatusErrorKeepsRateLimitRetryable(t *testing.T) {
	err := normalizeTaskStatusError(
		amazonTaskStageCreateListing,
		amazonModel.NewAmazonError(amazonModel.ErrorCodeRateLimit, "rate limit exceeded"),
	)
	if err == nil {
		t.Fatal("expected error")
	}

	got := err.Error()
	if strings.HasPrefix(got, "NONRETRYABLE: ") {
		t.Fatalf("expected retryable error, got %q", got)
	}
	if !strings.Contains(got, "[stage:"+amazonTaskStageCreateListing+"]") {
		t.Fatalf("expected stage marker, got %q", got)
	}
	if !strings.Contains(got, "["+amazonTaskReasonRateLimit+"]") {
		t.Fatalf("expected rate limit reason, got %q", got)
	}
}

func TestNewNonRetryableStageErrorFormatsStructuredMessage(t *testing.T) {
	err := newNonRetryableStageError(
		amazonTaskStageCheckDailyLimit,
		amazonTaskReasonDailyLimitReached,
		"店铺已达到每日上架限额",
	)
	if err == nil {
		t.Fatal("expected error")
	}

	want := "NONRETRYABLE: [stage:check_daily_limit] [DAILY_LIMIT_REACHED] 店铺已达到每日上架限额"
	if err.Error() != want {
		t.Fatalf("error = %q, want %q", err.Error(), want)
	}
}

func TestClassifyAmazonTaskErrorDetectsProductNotFound(t *testing.T) {
	reasonCode, retryable := classifyAmazonTaskError(errors.Join(amazonModel.ErrProductNotFound, errors.New("missing product")))
	if reasonCode != amazonTaskReasonProductNotFound {
		t.Fatalf("reasonCode = %q, want %q", reasonCode, amazonTaskReasonProductNotFound)
	}
	if retryable {
		t.Fatal("product not found should not be retryable")
	}
}
