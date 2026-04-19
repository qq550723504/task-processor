package amazon

import (
	"context"
	"errors"
	"testing"
	"time"

	"task-processor/internal/model"

	"github.com/playwright-community/playwright-go"
)

func TestInstanceProcessorRetriesOnProductQualityFailure(t *testing.T) {
	ip := NewInstanceProcessor(NewURLHelper(), NewProductChecker(), NewProductResultValidator())
	ip.SetQualityControlOptions(true, 2)
	metrics := newQualityMetrics()
	ip.SetQualityMetricsRecorder(metrics)

	attempts := 0
	reloads := 0
	ip.extractProduct = func(ctx context.Context, page playwright.Page, url, zipcode string) (*model.Product, error) {
		attempts++
		if attempts == 1 {
			return nil, &ProductQualityError{Reasons: []string{"title is empty"}}
		}
		return &model.Product{
			Asin:         "B001234567",
			Title:        "Demo Product",
			ImageURL:     "https://example.com/1.jpg",
			FinalPrice:   19.99,
			IsAvailable:  true,
			Availability: "In Stock",
		}, nil
	}
	ip.prepareRetry = func(ctx context.Context, page playwright.Page, waitTimeout time.Duration) error {
		reloads++
		return nil
	}

	product, err := ip.extractWithQualityRetry(context.Background(), nil, "https://www.amazon.com/dp/B001234567", "10001", 5*time.Second)
	if err != nil {
		t.Fatalf("expected retry to recover, got error: %v", err)
	}
	if product == nil || product.Asin != "B001234567" {
		t.Fatalf("expected valid product after retry, got %+v", product)
	}
	if attempts != 2 {
		t.Fatalf("expected 2 extraction attempts, got %d", attempts)
	}
	if reloads != 1 {
		t.Fatalf("expected 1 page retry preparation, got %d", reloads)
	}
	stats := metrics.Snapshot()
	if stats["quality_validation_retry_attempt_total"].(int64) != 1 {
		t.Fatalf("expected 1 quality retry attempt, got %#v", stats["quality_validation_retry_attempt_total"])
	}
	if stats["quality_validation_retry_recovered_total"].(int64) != 1 {
		t.Fatalf("expected 1 quality retry recovery, got %#v", stats["quality_validation_retry_recovered_total"])
	}
}

func TestInstanceProcessorStopsAfterQualityRetryLimit(t *testing.T) {
	ip := NewInstanceProcessor(NewURLHelper(), NewProductChecker(), NewProductResultValidator())
	ip.SetQualityControlOptions(true, 2)
	metrics := newQualityMetrics()
	ip.SetQualityMetricsRecorder(metrics)

	attempts := 0
	ip.extractProduct = func(ctx context.Context, page playwright.Page, url, zipcode string) (*model.Product, error) {
		attempts++
		return nil, &ProductQualityError{Reasons: []string{"primary image is missing"}}
	}
	ip.prepareRetry = func(ctx context.Context, page playwright.Page, waitTimeout time.Duration) error {
		return nil
	}

	product, err := ip.extractWithQualityRetry(context.Background(), nil, "https://www.amazon.com/dp/B001234567", "10001", 5*time.Second)
	if err == nil {
		t.Fatal("expected quality retry to fail after max attempts")
	}
	if product != nil {
		t.Fatalf("expected nil product on failure, got %+v", product)
	}
	if attempts != 2 {
		t.Fatalf("expected 2 extraction attempts, got %d", attempts)
	}
	stats := metrics.Snapshot()
	if stats["quality_validation_retry_attempt_total"].(int64) != 1 {
		t.Fatalf("expected 1 quality retry attempt, got %#v", stats["quality_validation_retry_attempt_total"])
	}
	if stats["quality_validation_retry_recovered_total"].(int64) != 0 {
		t.Fatalf("expected 0 quality retry recoveries, got %#v", stats["quality_validation_retry_recovered_total"])
	}
}

func TestInstanceProcessorResolveDefaultZipcodeForTargetContext(t *testing.T) {
	tests := []struct {
		name               string
		targetURL          string
		matchTargetContext func(page playwright.Page, targetURL string) (bool, error)
		want               string
	}{
		{
			name:      "us-matched-context-skips-fallback",
			targetURL: "https://www.amazon.com/dp/B001234567",
			matchTargetContext: func(page playwright.Page, targetURL string) (bool, error) {
				return true, nil
			},
			want: "",
		},
		{
			name:      "us-mismatched-context-uses-default-zipcode",
			targetURL: "https://www.amazon.com/dp/B001234567",
			matchTargetContext: func(page playwright.Page, targetURL string) (bool, error) {
				return false, nil
			},
			want: "94107",
		},
		{
			name:      "us-context-check-error-uses-default-zipcode",
			targetURL: "https://www.amazon.com/dp/B001234567",
			matchTargetContext: func(page playwright.Page, targetURL string) (bool, error) {
				return false, errors.New("page read failed")
			},
			want: "94107",
		},
		{
			name:      "jp-target-does-not-fallback",
			targetURL: "https://www.amazon.co.jp/dp/B0FLFBXSD2",
			matchTargetContext: func(page playwright.Page, targetURL string) (bool, error) {
				t.Fatalf("matchTargetContext should not be called for non-US targets")
				return false, nil
			},
			want: "",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ip := NewInstanceProcessor(NewURLHelper(), NewProductChecker(), NewProductResultValidator())
			metrics := newQualityMetrics()
			ip.matchTargetContext = tc.matchTargetContext
			ip.SetQualityMetricsRecorder(metrics)

			got := ip.resolveDefaultZipcodeForTargetContext(nil, tc.targetURL)
			if got != tc.want {
				t.Fatalf("resolveDefaultZipcodeForTargetContext(%q)=%q want %q", tc.targetURL, got, tc.want)
			}

			stats := metrics.Snapshot()
			switch tc.name {
			case "us-matched-context-skips-fallback":
				if stats["target_context_skip_total"].(int64) != 1 {
					t.Fatalf("expected target_context_skip_total=1, got %#v", stats["target_context_skip_total"])
				}
				if stats["target_context_fallback_total"].(int64) != 0 {
					t.Fatalf("expected target_context_fallback_total=0, got %#v", stats["target_context_fallback_total"])
				}
			case "us-mismatched-context-uses-default-zipcode":
				if stats["target_context_fallback_total"].(int64) != 1 {
					t.Fatalf("expected target_context_fallback_total=1, got %#v", stats["target_context_fallback_total"])
				}
				fallbackByRegion := stats["target_context_fallback_by_region"].(map[string]int64)
				if fallbackByRegion["us"] != 1 {
					t.Fatalf("expected us fallback count=1, got %#v", fallbackByRegion)
				}
			case "us-context-check-error-uses-default-zipcode":
				if stats["target_context_check_error_total"].(int64) != 1 {
					t.Fatalf("expected target_context_check_error_total=1, got %#v", stats["target_context_check_error_total"])
				}
				if stats["target_context_fallback_total"].(int64) != 1 {
					t.Fatalf("expected target_context_fallback_total=1, got %#v", stats["target_context_fallback_total"])
				}
			case "jp-target-does-not-fallback":
				if stats["target_context_skip_total"].(int64) != 0 ||
					stats["target_context_fallback_total"].(int64) != 0 ||
					stats["target_context_check_error_total"].(int64) != 0 {
					t.Fatalf("expected no target-context metrics for non-US target, got %#v", stats)
				}
			}
		})
	}
}
