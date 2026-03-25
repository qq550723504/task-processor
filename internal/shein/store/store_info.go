package store

import (
	"fmt"
	"sync"
	"time"

	"task-processor/internal/core/logger"
	managementAPI "task-processor/internal/infra/clients/management/api"
	"task-processor/internal/shein"

	"github.com/sirupsen/logrus"
)

type storeCacheEntry struct {
	store      *managementAPI.StoreRespDTO
	expireTime time.Time
}

var (
	storeCache      sync.Map
	storeCacheTTL   = 5 * time.Minute
	storeCacheStats struct {
		hits   int64
		misses int64
		mu     sync.Mutex
	}
)

type StoreInfoHandler struct {
	logger      *logrus.Entry
	storeClient interface {
		GetStore(id int64) (*managementAPI.StoreRespDTO, error)
	}
}

func NewStoreInfoHandler(storeClient interface {
	GetStore(id int64) (*managementAPI.StoreRespDTO, error)
}) *StoreInfoHandler {
	return &StoreInfoHandler{
		logger:      logger.GetGlobalLogger("StoreInfoHandler"),
		storeClient: storeClient,
	}
}

func (h *StoreInfoHandler) Name() string {
	return "store_info_handler"
}

func (h *StoreInfoHandler) Handle(ctx *shein.TaskContext) error {
	h.logger.Info("load store info")

	if ctx.Task == nil {
		return fmt.Errorf("task is nil")
	}
	if ctx.Task.StoreID == 0 {
		return fmt.Errorf("store id is empty")
	}

	storeInfo, fromCache := h.getStoreWithCache(ctx.Task.StoreID)
	if fromCache {
		h.logger.Infof("store info loaded from cache: store_id=%d store_name=%s", storeInfo.ID, storeInfo.Name)
		if err := h.validateStoreStatus(ctx, storeInfo); err != nil {
			return err
		}
		ctx.SetStoreInfo(storeInfo)
		return nil
	}

	storeInfo, err := h.storeClient.GetStore(ctx.Task.StoreID)
	if err != nil {
		h.logger.Errorf("get store info failed: %v", err)
		return h.handleStoreError(err)
	}
	if storeInfo == nil {
		return fmt.Errorf("store info is nil")
	}
	if err := h.validateStoreStatus(ctx, storeInfo); err != nil {
		return err
	}

	h.setStoreCache(ctx.Task.StoreID, storeInfo)
	ctx.SetStoreInfo(storeInfo)
	h.logger.Infof("store info loaded: store_id=%d store_name=%s", storeInfo.ID, storeInfo.Name)
	return nil
}

func (h *StoreInfoHandler) validateStoreStatus(ctx *shein.TaskContext, storeInfo *managementAPI.StoreRespDTO) error {
	if !*storeInfo.EnableAutoListing {
		h.logger.Warnf("store auto listing is disabled: store_id=%d", storeInfo.ID)
		h.cleanupStoreResources(ctx)
		h.pauseStore(ctx, "store auto listing is disabled")
		return shein.NewNonRetryableError("store auto listing is disabled", nil)
	}
	return nil
}

func (h *StoreInfoHandler) cleanupStoreResources(ctx *shein.TaskContext) {
	if ctx.MemoryManager != nil {
		logger.GetGlobalLogger("shein/store").Infof("cleanup store resources: tenant_id=%d store_id=%d", ctx.Task.TenantID, ctx.Task.StoreID)
	}
}

func (h *StoreInfoHandler) pauseStore(ctx *shein.TaskContext, reason string) {
	if ctx.MemoryManager != nil {
		ctx.MemoryManager.ShopPauseManager.PauseShop(ctx.Task.TenantID, ctx.Task.StoreID, reason, 24*time.Hour)
		h.logger.Infof("store paused for 24h: tenant_id=%d store_id=%d reason=%s", ctx.Task.TenantID, ctx.Task.StoreID, reason)
	}
}

func (h *StoreInfoHandler) handleStoreError(err error) error {
	if retryableErr, ok := err.(interface{ IsRetryable() bool }); ok {
		if !retryableErr.IsRetryable() {
			return shein.NewNonRetryableError("get store info failed", err)
		}
	}
	return shein.NewRetryableError("get store info failed", err)
}

func (h *StoreInfoHandler) getStoreWithCache(storeID int64) (*managementAPI.StoreRespDTO, bool) {
	if cached, ok := storeCache.Load(storeID); ok {
		entry := cached.(storeCacheEntry)
		if time.Now().Before(entry.expireTime) {
			storeCacheStats.mu.Lock()
			storeCacheStats.hits++
			storeCacheStats.mu.Unlock()
			return entry.store, true
		}
		storeCache.Delete(storeID)
	}

	storeCacheStats.mu.Lock()
	storeCacheStats.misses++
	storeCacheStats.mu.Unlock()
	return nil, false
}

func (h *StoreInfoHandler) setStoreCache(storeID int64, store *managementAPI.StoreRespDTO) {
	storeCache.Store(storeID, storeCacheEntry{store: store, expireTime: time.Now().Add(storeCacheTTL)})
}

func GetStoreCacheStats() (hits, misses int64) {
	storeCacheStats.mu.Lock()
	defer storeCacheStats.mu.Unlock()
	return storeCacheStats.hits, storeCacheStats.misses
}
