package listingcontrol

import (
	"context"
	"testing"
	"time"
)

func TestQuotaStructuredRemainingZeroBlocks(t *testing.T) {
	runtime := newFakeStringRuntime()
	runtime.set("listing:remaining:quota:v2:10:20", `{"remaining":0,"source":"manual","updatedAt":"2026-06-23T00:00:00Z","expiresAt":"2026-06-24T00:00:00Z"}`, 2*time.Hour)

	result, err := NewQuotaService(runtime, QuotaConfig{}).Check(context.Background(), 10, 20)
	if err != nil {
		t.Fatalf("Check returned error: %v", err)
	}

	if !result.Blocked {
		t.Fatal("expected quota to block")
	}
	if result.Reason != ReasonQuotaExhausted {
		t.Fatalf("expected %q, got %q", ReasonQuotaExhausted, result.Reason)
	}
	if result.Remaining != 0 {
		t.Fatalf("expected remaining 0, got %d", result.Remaining)
	}
	if result.Key != "listing:remaining:quota:v2:10:20" {
		t.Fatalf("expected structured key, got %q", result.Key)
	}
	if result.TTL != 2*time.Hour {
		t.Fatalf("expected TTL 2h, got %v", result.TTL)
	}
	if result.Source != "manual" {
		t.Fatalf("expected source manual, got %q", result.Source)
	}
}

func TestQuotaMalformedStructuredReturnsInvalidError(t *testing.T) {
	runtime := newFakeStringRuntime()
	runtime.set("listing:remaining:quota:v2:10:20", `{bad json`, time.Minute)

	result, err := NewQuotaService(runtime, QuotaConfig{}).Check(context.Background(), 10, 20)
	requireQuotaInvalid(t, err)
	if result.Reason != ReasonQuotaInvalid {
		t.Fatalf("expected %q, got %q", ReasonQuotaInvalid, result.Reason)
	}
	if !result.Blocked {
		t.Fatal("expected invalid quota result to block")
	}
}

func TestQuotaLegacyNoTTLIsIgnoredWhenLegacyDisabled(t *testing.T) {
	runtime := newFakeStringRuntime()
	runtime.set("listing:remaining:quota:10:20", "0", -1)

	result, err := NewQuotaService(runtime, QuotaConfig{EnableLegacyQuotaKeys: false}).Check(context.Background(), 10, 20)
	if err != nil {
		t.Fatalf("Check returned error: %v", err)
	}

	if result.Blocked {
		t.Fatalf("expected legacy quota to be ignored, got reason %q", result.Reason)
	}
	if result.Key != "" {
		t.Fatalf("expected no key when legacy is disabled, got %q", result.Key)
	}
}

func TestQuotaLegacyNoTTLBlocksAndExposesTTLWhenLegacyEnabled(t *testing.T) {
	runtime := newFakeStringRuntime()
	runtime.set("listing:remaining:quota:10:20", "0", -1)

	result, err := NewQuotaService(runtime, QuotaConfig{EnableLegacyQuotaKeys: true}).Check(context.Background(), 10, 20)
	if err != nil {
		t.Fatalf("Check returned error: %v", err)
	}

	if !result.Blocked {
		t.Fatal("expected legacy quota to block")
	}
	if result.Reason != ReasonQuotaExhausted {
		t.Fatalf("expected %q, got %q", ReasonQuotaExhausted, result.Reason)
	}
	if result.Key != "listing:remaining:quota:10:20" {
		t.Fatalf("expected legacy key, got %q", result.Key)
	}
	if result.TTL != -1 {
		t.Fatalf("expected no-TTL marker -1, got %v", result.TTL)
	}
	if result.Source != QuotaSourceLegacy {
		t.Fatalf("expected source %q, got %q", QuotaSourceLegacy, result.Source)
	}
}

func TestStoreRuntimeReportsQuotaInvalidAsNonDispatchable(t *testing.T) {
	auto := true
	runtime := newFakeStringRuntime()
	runtime.set("listing:queue:mode:10:20", "store-dedicated", 0)
	runtime.set("listing:queue:owner:10:20", "node-a", 0)
	runtime.set("listing:remaining:quota:v2:10:20", `{bad json`, time.Minute)

	service := NewStoreRuntime(fakeStoreSource{stores: []StoreSnapshot{{
		TenantID:          10,
		StoreID:           20,
		Platform:          "shein",
		Status:            StoreStatusEnabled,
		EnableAutoListing: &auto,
	}}}, runtime, StoreRuntimeConfig{
		MaxQueuedPerStore:    10,
		OwnerBrowserPoolSize: 2,
	})

	readiness, err := service.ListReadiness(context.Background(), "shein")
	if err != nil {
		t.Fatalf("ListReadiness returned error: %v", err)
	}
	if len(readiness) != 1 {
		t.Fatalf("expected 1 readiness, got %d", len(readiness))
	}
	if readiness[0].Dispatchable {
		t.Fatal("expected invalid quota store to be non-dispatchable")
	}
	if readiness[0].Reason != ReasonQuotaInvalid {
		t.Fatalf("expected %q, got %q", ReasonQuotaInvalid, readiness[0].Reason)
	}
}
