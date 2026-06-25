package store

import (
	"fmt"
	"sync"
	"task-processor/internal/core/logger"
	"task-processor/internal/pipeline"
	api "task-processor/internal/ports/managementapi"
	temucontext "task-processor/internal/temu/context"
	"time"

	"github.com/sirupsen/logrus"
)

// storeCacheEntry 店铺缓存条目
type storeCacheEntry struct {
	store      *api.StoreRespDTO
	expireTime time.Time
}

// 全局店铺信息缓存（5分钟过期）
var (
	storeCache      sync.Map // key: storeID (int64), value: storeCacheEntry
	storeCacheTTL   = 5 * time.Minute
	storeCacheStats struct {
		hits   int64
		misses int64
		mu     sync.Mutex
	}
)

// StoreInfoHandler 店铺信息处理器
type StoreInfoHandler struct {
	logger      *logrus.Entry
	storeClient interface {
		GetStore(id int64) (*api.StoreRespDTO, error)
	}
}

// NewStoreInfoHandler 创建新的店铺信息处理器
func NewStoreInfoHandler(storeClient interface {
	GetStore(id int64) (*api.StoreRespDTO, error)
}) *StoreInfoHandler {
	return &StoreInfoHandler{
		logger:      logger.GetGlobalLogger("temu.handlers.store_info").WithField("handler", "StoreInfoHandler"),
		storeClient: storeClient,
	}
}

// Name 返回处理器名称
func (h *StoreInfoHandler) Name() string {
	return "店铺信息处理器"
}

// HandleTemu 处理TEMU任务（实现TemuHandler接口）
func (h *StoreInfoHandler) HandleTemu(temuCtx *temucontext.TemuTaskContext) error {
	h.logger.Info("开始获取店铺信息")
	task := temuCtx.GetTask()
	if task == nil {
		return fmt.Errorf("任务信息为空")
	}

	if task.StoreID == 0 {
		return fmt.Errorf("店铺ID为空")
	}

	// 尝试从缓存获取
	storeInfo, fromCache := getStoreWithCache(task.StoreID)
	if fromCache {
		h.logger.WithFields(map[string]any{
			"store_id":   storeInfo.ID,
			"store_name": storeInfo.Name,
		}).Info("✅ 从缓存获取店铺信息")
		// 直接赋值到强类型字段
		temuCtx.StoreInfo = storeInfo
		return nil
	}

	// 缓存未命中，从API获取
	storeInfo, err := h.storeClient.GetStore(task.StoreID)
	if err != nil {
		h.logger.WithError(err).Error("获取店铺信息失败")
		return fmt.Errorf("获取店铺信息失败: %w", err)
	}

	if storeInfo == nil {
		return fmt.Errorf("店铺信息为空")
	}

	// 缓存店铺信息
	setStoreCache(task.StoreID, storeInfo)

	// 直接赋值到强类型字段
	temuCtx.StoreInfo = storeInfo

	h.logger.WithFields(map[string]any{
		"store_id":   storeInfo.ID,
		"store_name": storeInfo.Name,
		"price_type": storeInfo.PriceType,
	}).Info("✅ 获取店铺信息成功")

	return nil
}

// getStoreWithCache 从缓存获取店铺信息
func getStoreWithCache(storeID int64) (*api.StoreRespDTO, bool) {
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

// setStoreCache 设置店铺信息到缓存
func setStoreCache(storeID int64, store *api.StoreRespDTO) {
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

// Handle 兼容原有的Handler接口（用于pipeline.AddHandler）
func (h *StoreInfoHandler) Handle(ctx pipeline.TaskContext) error {
	// 尝试类型断言为TemuTaskContext
	if temuCtx, ok := ctx.(*temucontext.TemuTaskContext); ok {
		return h.HandleTemu(temuCtx)
	}
	// 如果不是TemuTaskContext，返回错误
	return fmt.Errorf("上下文类型错误，期望*temucontext.TemuTaskContext，实际类型: %T", ctx)
}
