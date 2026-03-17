// Package store 提供SHEIN平台的处理模块
package store

import (
	"fmt"
	"sync"
	management_api "task-processor/internal/infra/clients/management/api"
	"task-processor/internal/shein"
	"time"

	"github.com/sirupsen/logrus"
)

// storeCacheEntry 店铺缓存条目（参考TEMU实现）
type storeCacheEntry struct {
	store      *management_api.StoreRespDTO
	expireTime time.Time
}

// 全局店铺信息缓存（5分钟过期，参考TEMU设计）
var (
	storeCache      sync.Map // key: storeID (int64), value: storeCacheEntry
	storeCacheTTL   = 5 * time.Minute
	storeCacheStats struct {
		hits   int64
		misses int64
		mu     sync.Mutex
	}
)

// StoreInfoHandler 获取店铺信息处理器（参考TEMU简洁设计）
type StoreInfoHandler struct {
	logger      *logrus.Entry
	storeClient interface {
		GetStore(id int64) (*management_api.StoreRespDTO, error)
	}
}

// NewStoreInfoHandler 创建新的店铺信息处理器
func NewStoreInfoHandler(storeClient interface {
	GetStore(id int64) (*management_api.StoreRespDTO, error)
}) *StoreInfoHandler {
	return &StoreInfoHandler{
		logger:      logrus.WithField("handler", "StoreInfoHandler"),
		storeClient: storeClient,
	}
}

// Name 返回处理器名称
func (h *StoreInfoHandler) Name() string {
	return "店铺信息处理器"
}

// Handle 处理任务（参考TEMU的简洁实现）
func (h *StoreInfoHandler) Handle(ctx *shein.TaskContext) error {
	h.logger.Info("开始获取店铺信息")

	// 检查任务上下文中的必要数据
	if ctx.Task == nil {
		return fmt.Errorf("任务信息为空")
	}

	if ctx.Task.StoreID == 0 {
		return fmt.Errorf("店铺ID为空")
	}

	// 尝试从缓存获取
	storeInfo, fromCache := h.getStoreWithCache(ctx.Task.StoreID)
	if fromCache {
		h.logger.Infof("✅ 从缓存获取店铺信息: StoreID=%d, StoreName=%s", storeInfo.ID, storeInfo.Name)

		// 检查店铺状态
		if err := h.validateStoreStatus(ctx, storeInfo); err != nil {
			return err
		}

		ctx.StoreInfo = storeInfo
		return nil
	}

	// 缓存未命中，从API获取
	storeInfo, err := h.storeClient.GetStore(ctx.Task.StoreID)
	if err != nil {
		h.logger.Errorf("获取店铺信息失败: %v", err)
		return h.handleStoreError(err)
	}

	if storeInfo == nil {
		return fmt.Errorf("店铺信息为空")
	}

	// 检查店铺状态
	if err := h.validateStoreStatus(ctx, storeInfo); err != nil {
		return err
	}

	// 存入缓存
	h.setStoreCache(ctx.Task.StoreID, storeInfo)

	// 将店铺信息存储到上下文中
	ctx.StoreInfo = storeInfo

	h.logger.Infof("成功获取店铺信息（已缓存）: StoreID=%d, StoreName=%s",
		storeInfo.ID, storeInfo.Name)
	return nil
}

// validateStoreStatus 验证店铺状态（SHEIN特有逻辑）
func (h *StoreInfoHandler) validateStoreStatus(ctx *shein.TaskContext, storeInfo *management_api.StoreRespDTO) error {
	if !*storeInfo.EnableAutoListing {
		h.logger.Warnf("店铺未开启自动上架: StoreID=%d", storeInfo.ID)

		// 清理相关缓存
		h.cleanupStoreResources(ctx)

		// 暂停店铺
		h.pauseStore(ctx, "店铺未开启自动上架")

		return shein.NewNonRetryableError("店铺未开启自动上架", nil)
	}
	return nil
}

// cleanupStoreResources 清理店铺相关资源
func (h *StoreInfoHandler) cleanupStoreResources(ctx *shein.TaskContext) {
	// 通过内存管理器清理相关缓存
	if ctx.MemoryManager != nil {
		logrus.Infof("正在清理店铺 %d:%d 的相关缓存", ctx.Task.TenantID, ctx.Task.StoreID)
	}
}

// pauseStore 暂停店铺
func (h *StoreInfoHandler) pauseStore(ctx *shein.TaskContext, reason string) {
	if ctx.MemoryManager != nil {
		ctx.MemoryManager.ShopPauseManager.PauseShop(
			ctx.Task.TenantID,
			ctx.Task.StoreID,
			reason,
			24*time.Hour, // 暂停24小时
		)
		h.logger.Infof("已暂停店铺 %d:%d 上架24小时，原因: %s", ctx.Task.TenantID, ctx.Task.StoreID, reason)
	}
}

// handleStoreError 处理店铺错误
func (h *StoreInfoHandler) handleStoreError(err error) error {
	// 检查原始错误是否已经是不可重试的
	if retryableErr, ok := err.(interface{ IsRetryable() bool }); ok {
		if !retryableErr.IsRetryable() {
			return shein.NewNonRetryableError("获取店铺基础信息失败", err)
		}
	}
	// 网络错误或临时性错误可重试，数据错误不可重试
	return shein.NewRetryableError("获取店铺基础信息失败", err)
}

// getStoreWithCache 从缓存获取店铺信息（参考TEMU实现）
func (h *StoreInfoHandler) getStoreWithCache(storeID int64) (*management_api.StoreRespDTO, bool) {
	if cached, ok := storeCache.Load(storeID); ok {
		entry := cached.(storeCacheEntry)
		// 检查是否过期
		if time.Now().Before(entry.expireTime) {
			// 更新缓存命中统计
			storeCacheStats.mu.Lock()
			storeCacheStats.hits++
			storeCacheStats.mu.Unlock()
			return entry.store, true
		}
		// 过期，删除缓存
		storeCache.Delete(storeID)
	}

	// 更新缓存未命中统计
	storeCacheStats.mu.Lock()
	storeCacheStats.misses++
	storeCacheStats.mu.Unlock()

	return nil, false
}

// setStoreCache 设置店铺缓存（参考TEMU实现）
func (h *StoreInfoHandler) setStoreCache(storeID int64, store *management_api.StoreRespDTO) {
	entry := storeCacheEntry{
		store:      store,
		expireTime: time.Now().Add(storeCacheTTL),
	}
	storeCache.Store(storeID, entry)
}

// GetStoreCacheStats 获取缓存统计信息（用于监控）
func GetStoreCacheStats() (hits, misses int64) {
	storeCacheStats.mu.Lock()
	defer storeCacheStats.mu.Unlock()
	return storeCacheStats.hits, storeCacheStats.misses
}
