package listingcontrol

import (
	"context"
	"errors"
	"fmt"
	"strings"
)

const (
	StoreStatusEnabled = 0

	QueueModeStoreDedicated = "store-dedicated"

	ReasonDispatchable          = ""
	ReasonStoreDisabled         = "store_disabled"
	ReasonAutoListingDisabled   = "auto_listing_disabled"
	ReasonQueueNotDedicated     = "queue_not_dedicated"
	ReasonNoLiveOwner           = "no_live_owner"
	ReasonStorePaused           = "store_paused"
	ReasonNoCapacity            = "no_capacity"
	ReasonQueueDepthUnavailable = "queue_depth_unavailable"
)

type StoreSource interface {
	ListEnabledAutoListingStores(ctx context.Context, platform string) ([]StoreSnapshot, error)
}

type StoreSnapshot struct {
	TenantID          int64
	StoreID           int64
	Platform          string
	Status            int
	EnableAutoListing *bool
	Name              string
}

type QueueDepthSource interface {
	QueueDepth(ctx context.Context, tenantID, storeID int64) (int64, error)
}

type StoreRuntimeConfig struct {
	MaxQueuedPerStore     int
	OwnerBrowserPoolSize  int
	EnableLegacyQuotaKeys bool
	QueueDepthSource      QueueDepthSource
}

type StoreRuntime struct {
	stores     StoreSource
	runtime    StringRuntime
	queueDepth QueueDepthSource
	quota      *QuotaService
	config     StoreRuntimeConfig
}

type StoreReadiness struct {
	Store        StoreSnapshot
	Dispatchable bool
	Reason       string
	OwnerNode    string
	Mode         string
	Paused       bool
	Capacity     int
	Queued       int64
	Quota        QuotaResult
}

func NewStoreRuntime(stores StoreSource, runtime StringRuntime, config StoreRuntimeConfig) *StoreRuntime {
	queueDepthSource := config.QueueDepthSource
	if queueDepthSource == nil {
		if source, ok := runtime.(QueueDepthSource); ok {
			queueDepthSource = source
		}
	}
	return &StoreRuntime{
		stores:     stores,
		runtime:    runtime,
		queueDepth: queueDepthSource,
		quota: NewQuotaService(runtime, QuotaConfig{
			EnableLegacyQuotaKeys: config.EnableLegacyQuotaKeys,
		}),
		config: config,
	}
}

func (s *StoreRuntime) ListReadiness(ctx context.Context, platform string) ([]StoreReadiness, error) {
	stores, err := s.stores.ListEnabledAutoListingStores(ctx, platform)
	if err != nil {
		return nil, err
	}

	readiness := make([]StoreReadiness, 0, len(stores))
	ownerCounts := make(map[string]int)
	for _, store := range stores {
		item, err := s.evaluateBase(ctx, store)
		if err != nil {
			return nil, err
		}
		readiness = append(readiness, item)
		if item.Mode == QueueModeStoreDedicated && strings.TrimSpace(item.OwnerNode) != "" {
			ownerCounts[item.OwnerNode]++
		}
	}

	for i := range readiness {
		item := &readiness[i]
		item.Capacity = storeCapacity(s.config.MaxQueuedPerStore, s.config.OwnerBrowserPoolSize, ownerCounts[item.OwnerNode])
		if item.Reason != ReasonDispatchable {
			continue
		}

		item.Queued, err = queueDepth(ctx, s.queueDepth, item.Store.TenantID, item.Store.StoreID)
		if err != nil {
			item.Reason = ReasonQueueDepthUnavailable
			item.Dispatchable = false
			continue
		}
		item.Quota, err = s.quota.Check(ctx, item.Store.TenantID, item.Store.StoreID)
		if err != nil {
			if errors.Is(err, ErrQuotaInvalid) {
				item.Reason = ReasonQuotaInvalid
				item.Dispatchable = false
				continue
			}
			return nil, err
		}
		if item.Quota.Blocked {
			item.Reason = item.Quota.Reason
			item.Dispatchable = false
			continue
		}
		if item.Queued >= int64(item.Capacity) {
			item.Reason = ReasonNoCapacity
			item.Dispatchable = false
			continue
		}
		item.Dispatchable = true
	}

	return readiness, nil
}

func (s *StoreRuntime) evaluateBase(ctx context.Context, store StoreSnapshot) (StoreReadiness, error) {
	item := StoreReadiness{Store: store}
	if store.Status != StoreStatusEnabled {
		item.Reason = ReasonStoreDisabled
		return item, nil
	}
	if store.EnableAutoListing == nil || !*store.EnableAutoListing {
		item.Reason = ReasonAutoListingDisabled
		return item, nil
	}

	mode, err := getString(ctx, s.runtime, queueModeKey(store.TenantID, store.StoreID))
	if err != nil {
		return item, err
	}
	item.Mode = mode
	if mode != QueueModeStoreDedicated {
		item.Reason = ReasonQueueNotDedicated
		return item, nil
	}

	owner, err := getString(ctx, s.runtime, queueOwnerKey(store.TenantID, store.StoreID))
	if err != nil {
		return item, err
	}
	item.OwnerNode = strings.TrimSpace(owner)
	if item.OwnerNode == "" {
		item.Reason = ReasonNoLiveOwner
		return item, nil
	}

	paused, err := s.runtime.Exists(ctx, pauseKey(store.Platform, store.TenantID, store.StoreID))
	if err != nil {
		return item, err
	}
	item.Paused = paused
	if paused {
		item.Reason = ReasonStorePaused
		return item, nil
	}

	return item, nil
}

func getString(ctx context.Context, runtime StringRuntime, key string) (string, error) {
	value, err := runtime.Get(ctx, key)
	if err != nil {
		if errors.Is(err, ErrRuntimeKeyNotFound) {
			return "", nil
		}
		return "", err
	}
	return value, nil
}

func queueDepth(ctx context.Context, source QueueDepthSource, tenantID, storeID int64) (int64, error) {
	if source == nil {
		return 0, errors.New("queue depth source unavailable")
	}
	return source.QueueDepth(ctx, tenantID, storeID)
}

func storeCapacity(maxQueuedPerStore, ownerBrowserPoolSize, ownedStoreCount int) int {
	if ownerBrowserPoolSize <= 0 {
		ownerBrowserPoolSize = 1
	}
	if ownedStoreCount <= 0 {
		ownedStoreCount = 1
	}
	ownerCapacity := ownerBrowserPoolSize / ownedStoreCount
	if ownerCapacity < 1 {
		ownerCapacity = 1
	}
	if maxQueuedPerStore > 0 && maxQueuedPerStore < ownerCapacity {
		return maxQueuedPerStore
	}
	return ownerCapacity
}

func queueModeKey(tenantID, storeID int64) string {
	return fmt.Sprintf("listing:queue:mode:%d:%d", tenantID, storeID)
}

func queueOwnerKey(tenantID, storeID int64) string {
	return fmt.Sprintf("listing:queue:owner:%d:%d", tenantID, storeID)
}

func pauseKey(platform string, tenantID, storeID int64) string {
	return fmt.Sprintf("listing:task:pause:%s:%d:%d", strings.ToLower(platform), tenantID, storeID)
}
