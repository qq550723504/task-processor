package handlers

import (
	"fmt"
	"sync"
	"task-processor/common/management/api"
	"task-processor/common/pipeline"
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
		logger:      logrus.WithField("handler", "StoreInfoHandler"),
		storeClient: storeClient,
	}
}

// Name 返回处理器名称
func (h *StoreInfoHandler) Name() string {
	return "店铺信息处理器"
}

// Handle 处理任务
func (h *StoreInfoHandler) Handle(ctx *pipeline.TaskContext) error {
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
		ctx.StoreInfo = storeInfo
		return nil
	}

	// 缓存未命中，从API获取
	storeInfo, err := h.storeClient.GetStore(ctx.Task.StoreID)
	if err != nil {
		h.logger.Errorf("获取店铺信息失败: %v", err)
		return fmt.Errorf("获取店铺信息失败: %w", err)
	}

	if storeInfo == nil {
		return fmt.Errorf("店铺信息为空")
	}

	// 存入缓存
	h.setStoreCache(ctx.Task.StoreID, storeInfo)

	// 将店铺信息存储到上下文中
	ctx.StoreInfo = storeInfo

	h.logger.Infof("成功获取店铺信息（已缓存）: StoreID=%d, StoreName=%s",
		storeInfo.ID, storeInfo.Name)
	return nil
}

// getStoreWithCache 从缓存获取店铺信息
func (h *StoreInfoHandler) getStoreWithCache(storeID int64) (*api.StoreRespDTO, bool) {
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

// setStoreCache 设置店铺缓存
func (h *StoreInfoHandler) setStoreCache(storeID int64, store *api.StoreRespDTO) {
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
