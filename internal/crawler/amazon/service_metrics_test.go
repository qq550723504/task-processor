package amazon

import (
	"context"
	"errors"
	"testing"

	"task-processor/internal/core/config"
	"task-processor/internal/model"

	"github.com/sirupsen/logrus"
)

func TestServiceStatsIncludeFetchMetrics(t *testing.T) {
	service := &Service{
		config: &config.Config{
			Amazon: config.AmazonConfig{
				Enabled: true,
				Zipcodes: map[string]string{
					"us": "10001",
				},
				CrawlTimeout: 30,
			},
		},
		logger:         logrus.New(),
		domainResolver: NewDomainResolver(),
		metrics:        newServiceMetrics(),
		regionGuard:    newRegionGuard(config.AmazonRegionGuardConfig{Enabled: true, FailureThreshold: 2, EvaluationWindowSeconds: 60, CooldownSeconds: 30}),
	}

	service.processProduct = func(ctx context.Context, url, zipcode string) (*model.Product, error) {
		switch url {
		case "https://www.amazon.com/dp/B001":
			return &model.Product{Asin: "B001", Title: "Demo"}, nil
		default:
			return nil, errors.New("captcha challenge detected")
		}
	}

	if _, _, err := service.FetchProduct(context.Background(), "https://www.amazon.com/dp/B001", "", "us", ""); err != nil {
		t.Fatalf("expected success, got error: %v", err)
	}

	if _, _, err := service.FetchProduct(context.Background(), "https://www.amazon.com/dp/B002", "", "us", ""); err == nil {
		t.Fatalf("expected error for failed fetch")
	}

	service.metrics.RecordSuccess("async_task", "uk")
	service.metrics.RecordFailure("async_task", "uk", errors.New("navigation timeout exceeded"))
	service.metrics.RecordFailure("sync_api", "us", errors.New("初始化浏览器池失败: chromium launch failed"))
	service.metrics.RecordDedupeSharedHit("us")
	service.metrics.RecordDedupeWaitTimeout("us")
	service.metrics.RecordTaskSubmitFailure("enqueue")
	service.metrics.RecordRegionGuardOpen("us")
	service.metrics.RecordRegionGuardBlocked("us")
	service.regionGuard.RecordFailure("us", errors.New("captcha challenge detected"))
	service.regionGuard.RecordFailure("us", errors.New("captcha challenge detected"))
	service.amazonProcessor = &AmazonProcessor{
		qualityMetrics: newQualityMetrics(),
		initErr:        errors.New("初始化浏览器池失败: chromium launch failed"),
	}
	service.amazonProcessor.qualityMetrics.RecordValidationRetryAttempt()
	service.amazonProcessor.qualityMetrics.RecordValidationRetryRecovered()
	service.amazonProcessor.qualityMetrics.RecordTargetContextSkip("us")
	service.amazonProcessor.qualityMetrics.RecordTargetContextFallback("us")
	service.amazonProcessor.qualityMetrics.RecordTargetContextCheckError("us")

	stats := service.GetStats()

	if stats["fetch_total"].(int64) != 5 {
		t.Fatalf("expected fetch_total=4, got %#v", stats["fetch_total"])
	}
	if stats["fetch_success_total"].(int64) != 2 {
		t.Fatalf("expected fetch_success_total=2, got %#v", stats["fetch_success_total"])
	}
	if stats["fetch_failure_total"].(int64) != 3 {
		t.Fatalf("expected fetch_failure_total=2, got %#v", stats["fetch_failure_total"])
	}
	if stats["retryable_failure_total"].(int64) != 2 {
		t.Fatalf("expected retryable_failure_total=1, got %#v", stats["retryable_failure_total"])
	}
	if stats["dedupe_shared_hit_total"].(int64) != 1 {
		t.Fatalf("expected dedupe_shared_hit_total=1, got %#v", stats["dedupe_shared_hit_total"])
	}
	if stats["dedupe_wait_timeout_total"].(int64) != 1 {
		t.Fatalf("expected dedupe_wait_timeout_total=1, got %#v", stats["dedupe_wait_timeout_total"])
	}
	if stats["task_submit_failure_total"].(int64) != 1 {
		t.Fatalf("expected task_submit_failure_total=1, got %#v", stats["task_submit_failure_total"])
	}
	if stats["region_guard_open_total"].(int64) != 1 {
		t.Fatalf("expected region_guard_open_total=1, got %#v", stats["region_guard_open_total"])
	}
	if stats["region_guard_block_total"].(int64) != 1 {
		t.Fatalf("expected region_guard_block_total=1, got %#v", stats["region_guard_block_total"])
	}
	if stats["quality_validation_retry_attempt_total"].(int64) != 1 {
		t.Fatalf("expected quality_validation_retry_attempt_total=1, got %#v", stats["quality_validation_retry_attempt_total"])
	}
	if stats["quality_validation_retry_recovered_total"].(int64) != 1 {
		t.Fatalf("expected quality_validation_retry_recovered_total=1, got %#v", stats["quality_validation_retry_recovered_total"])
	}
	if stats["target_context_skip_total"].(int64) != 1 {
		t.Fatalf("expected target_context_skip_total=1, got %#v", stats["target_context_skip_total"])
	}
	if stats["target_context_fallback_total"].(int64) != 1 {
		t.Fatalf("expected target_context_fallback_total=1, got %#v", stats["target_context_fallback_total"])
	}
	if stats["target_context_check_error_total"].(int64) != 1 {
		t.Fatalf("expected target_context_check_error_total=1, got %#v", stats["target_context_check_error_total"])
	}

	failureByType := stats["failure_by_type"].(map[string]int64)
	if failureByType[FetchErrorTypeCaptcha] != 1 {
		t.Fatalf("expected captcha failure count=1, got %#v", failureByType)
	}
	if failureByType[FetchErrorTypeTimeout] != 1 {
		t.Fatalf("expected timeout failure count=1, got %#v", failureByType)
	}
	if failureByType[FetchErrorTypeProcessorUnavailable] != 1 {
		t.Fatalf("expected processor unavailable failure count=1, got %#v", failureByType)
	}

	successByMode := stats["success_by_mode"].(map[string]int64)
	if successByMode["sync_api"] != 1 || successByMode["async_task"] != 1 {
		t.Fatalf("unexpected success_by_mode: %#v", successByMode)
	}

	successByRegion := stats["success_by_region"].(map[string]int64)
	if successByRegion["us"] != 1 || successByRegion["uk"] != 1 {
		t.Fatalf("unexpected success_by_region: %#v", successByRegion)
	}

	failureByRegion := stats["failure_by_region"].(map[string]int64)
	if failureByRegion["us"] != 2 || failureByRegion["uk"] != 1 {
		t.Fatalf("unexpected failure_by_region: %#v", failureByRegion)
	}

	failureByRegionType := stats["failure_by_region_type"].(map[string]map[string]int64)
	if failureByRegionType["us"][FetchErrorTypeCaptcha] != 1 {
		t.Fatalf("expected us/captcha failure count=1, got %#v", failureByRegionType)
	}
	if failureByRegionType["uk"][FetchErrorTypeTimeout] != 1 {
		t.Fatalf("expected uk/timeout failure count=1, got %#v", failureByRegionType)
	}
	if failureByRegionType["us"][FetchErrorTypeProcessorUnavailable] != 1 {
		t.Fatalf("expected us/processor_unavailable failure count=1, got %#v", failureByRegionType)
	}

	regionGuardOpenByRegion := stats["region_guard_open_by_region"].(map[string]int64)
	if regionGuardOpenByRegion["us"] != 1 {
		t.Fatalf("expected us region guard open count=1, got %#v", regionGuardOpenByRegion)
	}

	regionGuardState := stats["region_guard_open_state_by_region"].(map[string]int64)
	if regionGuardState["us"] != 1 {
		t.Fatalf("expected us region guard state to be open, got %#v", regionGuardState)
	}
	targetContextSkipByRegion := stats["target_context_skip_by_region"].(map[string]int64)
	if targetContextSkipByRegion["us"] != 1 {
		t.Fatalf("expected us target_context_skip count=1, got %#v", targetContextSkipByRegion)
	}
	targetContextFallbackByRegion := stats["target_context_fallback_by_region"].(map[string]int64)
	if targetContextFallbackByRegion["us"] != 1 {
		t.Fatalf("expected us target_context_fallback count=1, got %#v", targetContextFallbackByRegion)
	}
	dedupeWaitTimeoutByRegion := stats["dedupe_wait_timeout_by_region"].(map[string]int64)
	if dedupeWaitTimeoutByRegion["us"] != 1 {
		t.Fatalf("expected us dedupe_wait_timeout count=1, got %#v", dedupeWaitTimeoutByRegion)
	}
	taskSubmitFailureByStage := stats["task_submit_failure_by_stage"].(map[string]int64)
	if taskSubmitFailureByStage["enqueue"] != 1 {
		t.Fatalf("expected enqueue task submit failure count=1, got %#v", taskSubmitFailureByStage)
	}
	if stats["browser_pool_init_error"] != "初始化浏览器池失败: chromium launch failed" {
		t.Fatalf("expected browser_pool_init_error to be present, got %#v", stats["browser_pool_init_error"])
	}
	if stats["quality_validation_retry_recovery_rate"].(float64) != 1 {
		t.Fatalf("expected quality_validation_retry_recovery_rate=1, got %#v", stats["quality_validation_retry_recovery_rate"])
	}
}
