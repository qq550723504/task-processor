package httpx

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestWriteMetricsResponse(t *testing.T) {
	rec := httptest.NewRecorder()

	writeMetricsResponse(rec, map[string]any{
		"fetch_total":                              int64(4),
		"fetch_success_rate":                       0.5,
		"failure_by_region_type":                   map[string]map[string]int64{"us": {"captcha": 2}, "uk": {"timeout": 1}},
		"failure_by_type":                          map[string]int64{"captcha": 2, "timeout": 1},
		"quality_validation_retry_attempt_total":   int64(3),
		"quality_validation_retry_recovered_total": int64(2),
		"quality_validation_retry_recovery_rate":   0.6666667,
		"region_guard_open_by_region":              map[string]int64{"us": 1},
		"region_guard_block_by_region":             map[string]int64{"us": 2},
		"retryable_failure_by_type":                map[string]int64{"timeout": 1},
		"status_count":                             map[string]int{"success": 3},
		"ignored_string":                           "skip-me",
	})

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rec.Code)
	}
	if contentType := rec.Header().Get("Content-Type"); !strings.Contains(contentType, "text/plain") {
		t.Fatalf("unexpected content type: %s", contentType)
	}

	body := rec.Body.String()
	expected := []string{
		`crawler_fetch_total 4`,
		`crawler_fetch_success_rate 0.5`,
		`crawler_failure_by_region_type{region="uk",type="timeout"} 1`,
		`crawler_failure_by_region_type{region="us",type="captcha"} 2`,
		`crawler_failure_by_type{key="captcha"} 2`,
		`crawler_failure_by_type{key="timeout"} 1`,
		`crawler_quality_validation_retry_attempt_total 3`,
		`crawler_quality_validation_retry_recovered_total 2`,
		`crawler_quality_validation_retry_recovery_rate 0.6666667`,
		`crawler_region_guard_block_by_region{key="us"} 2`,
		`crawler_region_guard_open_by_region{key="us"} 1`,
		`crawler_retryable_failure_by_type{key="timeout"} 1`,
		`crawler_status_count{key="success"} 3`,
	}
	for _, line := range expected {
		if !strings.Contains(body, line) {
			t.Fatalf("expected metrics body to contain %q, body=%s", line, body)
		}
	}
	if strings.Contains(body, "ignored_string") {
		t.Fatalf("expected string stats to be ignored, body=%s", body)
	}
}
