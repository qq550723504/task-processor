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
	service.metrics.RecordDedupeSharedHit("us")
	service.metrics.RecordRegionGuardOpen("us")
	service.metrics.RecordRegionGuardBlocked("us")
	service.regionGuard.RecordFailure("us", errors.New("captcha challenge detected"))
	service.regionGuard.RecordFailure("us", errors.New("captcha challenge detected"))
	service.amazonProcessor = &AmazonProcessor{qualityMetrics: newQualityMetrics()}
	service.amazonProcessor.qualityMetrics.RecordValidationRetryAttempt()
	service.amazonProcessor.qualityMetrics.RecordValidationRetryRecovered()

	stats := service.GetStats()

	if stats["fetch_total"].(int64) != 4 {
		t.Fatalf("expected fetch_total=4, got %#v", stats["fetch_total"])
	}
	if stats["fetch_success_total"].(int64) != 2 {
		t.Fatalf("expected fetch_success_total=2, got %#v", stats["fetch_success_total"])
	}
	if stats["fetch_failure_total"].(int64) != 2 {
		t.Fatalf("expected fetch_failure_total=2, got %#v", stats["fetch_failure_total"])
	}
	if stats["retryable_failure_total"].(int64) != 1 {
		t.Fatalf("expected retryable_failure_total=1, got %#v", stats["retryable_failure_total"])
	}
	if stats["dedupe_shared_hit_total"].(int64) != 1 {
		t.Fatalf("expected dedupe_shared_hit_total=1, got %#v", stats["dedupe_shared_hit_total"])
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

	failureByType := stats["failure_by_type"].(map[string]int64)
	if failureByType[FetchErrorTypeCaptcha] != 1 {
		t.Fatalf("expected captcha failure count=1, got %#v", failureByType)
	}
	if failureByType[FetchErrorTypeTimeout] != 1 {
		t.Fatalf("expected timeout failure count=1, got %#v", failureByType)
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
	if failureByRegion["us"] != 1 || failureByRegion["uk"] != 1 {
		t.Fatalf("unexpected failure_by_region: %#v", failureByRegion)
	}

	failureByRegionType := stats["failure_by_region_type"].(map[string]map[string]int64)
	if failureByRegionType["us"][FetchErrorTypeCaptcha] != 1 {
		t.Fatalf("expected us/captcha failure count=1, got %#v", failureByRegionType)
	}
	if failureByRegionType["uk"][FetchErrorTypeTimeout] != 1 {
		t.Fatalf("expected uk/timeout failure count=1, got %#v", failureByRegionType)
	}

	regionGuardOpenByRegion := stats["region_guard_open_by_region"].(map[string]int64)
	if regionGuardOpenByRegion["us"] != 1 {
		t.Fatalf("expected us region guard open count=1, got %#v", regionGuardOpenByRegion)
	}

	regionGuardState := stats["region_guard_open_state_by_region"].(map[string]int64)
	if regionGuardState["us"] != 1 {
		t.Fatalf("expected us region guard state to be open, got %#v", regionGuardState)
	}
	if stats["quality_validation_retry_recovery_rate"].(float64) != 1 {
		t.Fatalf("expected quality_validation_retry_recovery_rate=1, got %#v", stats["quality_validation_retry_recovery_rate"])
	}
}
