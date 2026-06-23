package listingcontrol

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestStoreRuntimeDispatchesEnabledDedicatedStoreWithOwnerAndCapacity(t *testing.T) {
	ctx := context.Background()
	auto := true
	runtime := newFakeStringRuntime()
	runtime.set("listing:queue:mode:10:20", "store-dedicated", 0)
	runtime.set("listing:queue:owner:10:20", "node-a", 0)

	service := NewStoreRuntime(fakeStoreSource{stores: []StoreSnapshot{{
		TenantID:          10,
		StoreID:           20,
		Platform:          "shein",
		Status:            StoreStatusEnabled,
		EnableAutoListing: &auto,
		Name:              "main",
	}}}, runtime, StoreRuntimeConfig{
		MaxQueuedPerStore:    10,
		OwnerBrowserPoolSize: 3,
	})

	readiness, err := service.ListReadiness(ctx, "shein")
	if err != nil {
		t.Fatalf("ListReadiness returned error: %v", err)
	}
	if len(readiness) != 1 {
		t.Fatalf("expected 1 readiness, got %d", len(readiness))
	}
	got := readiness[0]
	if !got.Dispatchable {
		t.Fatalf("expected store to be dispatchable, got reason %q", got.Reason)
	}
	if got.OwnerNode != "node-a" {
		t.Fatalf("expected owner node-a, got %q", got.OwnerNode)
	}
	if got.Mode != "store-dedicated" {
		t.Fatalf("expected dedicated mode, got %q", got.Mode)
	}
	if got.Capacity != 3 {
		t.Fatalf("expected capacity 3, got %d", got.Capacity)
	}
	if got.Queued != 0 {
		t.Fatalf("expected queued 0, got %d", got.Queued)
	}
}

func TestStoreRuntimeSkipsDisabledStore(t *testing.T) {
	auto := true
	got := singleReadiness(t, StoreSnapshot{
		TenantID:          10,
		StoreID:           20,
		Platform:          "shein",
		Status:            1,
		EnableAutoListing: &auto,
	})

	if got.Dispatchable {
		t.Fatal("expected disabled store to be non-dispatchable")
	}
	if got.Reason != ReasonStoreDisabled {
		t.Fatalf("expected %q, got %q", ReasonStoreDisabled, got.Reason)
	}
}

func TestStoreRuntimeSkipsNilOrFalseAutoListing(t *testing.T) {
	falseAuto := false
	tests := []struct {
		name  string
		store StoreSnapshot
	}{
		{
			name: "nil",
			store: StoreSnapshot{
				TenantID: 10,
				StoreID:  20,
				Platform: "shein",
				Status:   StoreStatusEnabled,
			},
		},
		{
			name: "false",
			store: StoreSnapshot{
				TenantID:          10,
				StoreID:           20,
				Platform:          "shein",
				Status:            StoreStatusEnabled,
				EnableAutoListing: &falseAuto,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := singleReadiness(t, tt.store)
			if got.Dispatchable {
				t.Fatal("expected auto-listing-disabled store to be non-dispatchable")
			}
			if got.Reason != ReasonAutoListingDisabled {
				t.Fatalf("expected %q, got %q", ReasonAutoListingDisabled, got.Reason)
			}
		})
	}
}

func TestStoreRuntimeSkipsMissingOwner(t *testing.T) {
	auto := true
	runtime := newFakeStringRuntime()
	runtime.set("listing:queue:mode:10:20", "store-dedicated", 0)

	got := singleReadinessWithRuntime(t, runtime, StoreSnapshot{
		TenantID:          10,
		StoreID:           20,
		Platform:          "shein",
		Status:            StoreStatusEnabled,
		EnableAutoListing: &auto,
	})

	if got.Dispatchable {
		t.Fatal("expected missing owner store to be non-dispatchable")
	}
	if got.Reason != ReasonNoLiveOwner {
		t.Fatalf("expected %q, got %q", ReasonNoLiveOwner, got.Reason)
	}
}

func TestStoreRuntimeSkipsNonDedicatedMode(t *testing.T) {
	auto := true
	runtime := newFakeStringRuntime()
	runtime.set("listing:queue:mode:10:20", "shared", 0)
	runtime.set("listing:queue:owner:10:20", "node-a", 0)

	got := singleReadinessWithRuntime(t, runtime, StoreSnapshot{
		TenantID:          10,
		StoreID:           20,
		Platform:          "shein",
		Status:            StoreStatusEnabled,
		EnableAutoListing: &auto,
	})

	if got.Dispatchable {
		t.Fatal("expected shared-mode store to be non-dispatchable")
	}
	if got.Reason != ReasonQueueNotDedicated {
		t.Fatalf("expected %q, got %q", ReasonQueueNotDedicated, got.Reason)
	}
}

func TestStoreRuntimeSkipsPausedStore(t *testing.T) {
	auto := true
	runtime := newFakeStringRuntime()
	runtime.set("listing:queue:mode:10:20", "store-dedicated", 0)
	runtime.set("listing:queue:owner:10:20", "node-a", 0)
	runtime.set("listing:task:pause:shein:10:20", "paused", time.Minute)

	got := singleReadinessWithRuntime(t, runtime, StoreSnapshot{
		TenantID:          10,
		StoreID:           20,
		Platform:          "SHEIN",
		Status:            StoreStatusEnabled,
		EnableAutoListing: &auto,
	})

	if got.Dispatchable {
		t.Fatal("expected paused store to be non-dispatchable")
	}
	if !got.Paused {
		t.Fatal("expected paused flag")
	}
	if got.Reason != ReasonStorePaused {
		t.Fatalf("expected %q, got %q", ReasonStorePaused, got.Reason)
	}
}

func TestStoreRuntimeCapacityUsesOwnerStoreCountAndBrowserPool(t *testing.T) {
	auto := true
	runtime := newFakeStringRuntime()
	runtime.set("listing:queue:mode:10:20", "store-dedicated", 0)
	runtime.set("listing:queue:owner:10:20", "node-a", 0)
	runtime.set("listing:queue:mode:10:21", "store-dedicated", 0)
	runtime.set("listing:queue:owner:10:21", "node-a", 0)
	runtime.set("listing:queue:mode:10:22", "store-dedicated", 0)
	runtime.set("listing:queue:owner:10:22", "node-b", 0)

	service := NewStoreRuntime(fakeStoreSource{stores: []StoreSnapshot{
		{TenantID: 10, StoreID: 20, Platform: "shein", Status: StoreStatusEnabled, EnableAutoListing: &auto},
		{TenantID: 10, StoreID: 21, Platform: "shein", Status: StoreStatusEnabled, EnableAutoListing: &auto},
		{TenantID: 10, StoreID: 22, Platform: "shein", Status: StoreStatusEnabled, EnableAutoListing: &auto},
	}}, runtime, StoreRuntimeConfig{
		MaxQueuedPerStore:    10,
		OwnerBrowserPoolSize: 5,
	})

	readiness, err := service.ListReadiness(context.Background(), "shein")
	if err != nil {
		t.Fatalf("ListReadiness returned error: %v", err)
	}
	byStore := map[int64]StoreReadiness{}
	for _, item := range readiness {
		byStore[item.Store.StoreID] = item
	}
	if byStore[20].Capacity != 2 {
		t.Fatalf("expected node-a capacity 2, got %d", byStore[20].Capacity)
	}
	if byStore[22].Capacity != 5 {
		t.Fatalf("expected node-b capacity 5, got %d", byStore[22].Capacity)
	}
}

func singleReadiness(t *testing.T, store StoreSnapshot) StoreReadiness {
	t.Helper()
	runtime := newFakeStringRuntime()
	runtime.set("listing:queue:mode:10:20", "store-dedicated", 0)
	runtime.set("listing:queue:owner:10:20", "node-a", 0)
	return singleReadinessWithRuntime(t, runtime, store)
}

func singleReadinessWithRuntime(t *testing.T, runtime *fakeStringRuntime, store StoreSnapshot) StoreReadiness {
	t.Helper()
	service := NewStoreRuntime(fakeStoreSource{stores: []StoreSnapshot{store}}, runtime, StoreRuntimeConfig{
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
	return readiness[0]
}

type fakeStoreSource struct {
	stores []StoreSnapshot
	err    error
}

func (f fakeStoreSource) ListEnabledAutoListingStores(ctx context.Context, platform string) ([]StoreSnapshot, error) {
	if f.err != nil {
		return nil, f.err
	}
	return f.stores, nil
}

type fakeStringRuntime struct {
	values map[string]fakeRuntimeValue
}

type fakeRuntimeValue struct {
	value string
	ttl   time.Duration
}

func newFakeStringRuntime() *fakeStringRuntime {
	return &fakeStringRuntime{values: make(map[string]fakeRuntimeValue)}
}

func (f *fakeStringRuntime) set(key, value string, ttl time.Duration) {
	f.values[key] = fakeRuntimeValue{value: value, ttl: ttl}
}

func (f *fakeStringRuntime) Get(ctx context.Context, key string) (string, error) {
	value, ok := f.values[key]
	if !ok {
		return "", ErrRuntimeKeyNotFound
	}
	return value.value, nil
}

func (f *fakeStringRuntime) Exists(ctx context.Context, key string) (bool, error) {
	_, ok := f.values[key]
	return ok, nil
}

func (f *fakeStringRuntime) TTL(ctx context.Context, key string) (time.Duration, error) {
	value, ok := f.values[key]
	if !ok {
		return 0, ErrRuntimeKeyNotFound
	}
	return value.ttl, nil
}

func (f *fakeStringRuntime) QueueDepth(ctx context.Context, tenantID, storeID int64) (int64, error) {
	return 0, nil
}

func requireQuotaInvalid(t *testing.T, err error) {
	t.Helper()
	if !errors.Is(err, ErrQuotaInvalid) {
		t.Fatalf("expected ErrQuotaInvalid, got %v", err)
	}
}
