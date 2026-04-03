package amazon

import "testing"

func TestParseTaskStatusMetadata(t *testing.T) {
	reasonCode, stage := parseTaskStatusMetadata("NONRETRYABLE: [stage:check_daily_limit] [DAILY_LIMIT_REACHED] limit reached")
	if reasonCode != amazonTaskReasonDailyLimitReached {
		t.Fatalf("reasonCode = %q, want %q", reasonCode, amazonTaskReasonDailyLimitReached)
	}
	if stage != "check_daily_limit" {
		t.Fatalf("stage = %q, want check_daily_limit", stage)
	}
}

func TestShouldPauseAmazonTask(t *testing.T) {
	if !shouldPauseAmazonTask(amazonTaskReasonAuthExpired) {
		t.Fatal("auth expired should pause task")
	}
	if !shouldPauseAmazonTask(amazonTaskReasonDailyLimitReached) {
		t.Fatal("daily limit should pause task")
	}
	if shouldPauseAmazonTask("VALIDATION_FAILED") {
		t.Fatal("validation failure should not pause task")
	}
}

func TestIsAmazonRetryableTaskError(t *testing.T) {
	if isAmazonRetryableTaskError(assertError("NONRETRYABLE: [stage:validate_input] [VALIDATION_FAILED] bad input")) {
		t.Fatal("nonretryable error should not be retried")
	}
	if !isAmazonRetryableTaskError(assertError("[stage:create_listing] [RATE_LIMIT] slow down")) {
		t.Fatal("rate limit error should remain retryable")
	}
}

type assertError string

func (e assertError) Error() string { return string(e) }
