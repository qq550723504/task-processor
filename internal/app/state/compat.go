package state

import (
	"context"

	"task-processor/internal/infra/clients/management"
	rootstate "task-processor/internal/state"
)

type CookieInfo = rootstate.CookieInfo
type CookieManager = rootstate.CookieManager
type DailyCountInfo = rootstate.DailyCountInfo
type DailyCountManager = rootstate.DailyCountManager
type DailyQuotaReservation = rootstate.DailyQuotaReservation
type MemoryManager = rootstate.MemoryManager
type ReListingQueueManager = rootstate.ReListingQueueManager
type ShopPauseInfo = rootstate.ShopPauseInfo
type ShopPauseManager = rootstate.ShopPauseManager
type StoreClient = rootstate.StoreClient

func NewCookieManager() *CookieManager {
	return rootstate.NewCookieManager()
}

func NewDailyCountManager(managementClientMgr *management.ClientManager) *DailyCountManager {
	return rootstate.NewDailyCountManager(managementClientMgr)
}

func NewMemoryManager(ctx context.Context, managementClientMgr *management.ClientManager) *MemoryManager {
	return rootstate.NewMemoryManager(ctx, managementClientMgr)
}

func NewReListingQueueManager() *ReListingQueueManager {
	return rootstate.NewReListingQueueManager()
}

func NewShopPauseManager() *ShopPauseManager {
	return rootstate.NewShopPauseManager()
}
