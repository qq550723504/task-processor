// Package shared 提供爬虫共享服务基础实现
package shared

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"task-processor/internal/core/config"
	"task-processor/internal/infra/redisclient"
	"task-processor/internal/infra/worker"

	"github.com/sirupsen/logrus"
)

const (
	defaultResultStorePrefix = "crawler:task-result"
	defaultResultStoreTTL    = 6 * time.Hour
)

type resultStore interface {
	Get(ctx context.Context, key string) (string, error)
	Set(ctx context.Context, key string, value string, ttl time.Duration) error
	Delete(ctx context.Context, key string) error
}

// BaseService 封装两个爬虫服务共同的结果存储与 worker 池逻辑。
// amazon.Service 和 alibaba1688.Service 通过嵌入此结构体消除重复代码。
type BaseService struct {
	workerPool        worker.WorkerPool
	results           sync.Map
	resultMu          sync.Mutex
	sharedResultStore resultStore
	resultStorePrefix string
	resultStoreTTL    time.Duration
}

// SetWorkerPool 设置 worker 池（由子类在构造时调用）
func (b *BaseService) SetWorkerPool(pool worker.WorkerPool) {
	b.workerPool = pool
}

// WorkerPool 返回 worker 池
func (b *BaseService) WorkerPool() worker.WorkerPool {
	return b.workerPool
}

// ConfigureSharedResultStore 配置共享任务结果存储。
func (b *BaseService) ConfigureSharedResultStore(store resultStore, prefix string, ttl time.Duration) {
	b.sharedResultStore = store
	if strings.TrimSpace(prefix) == "" {
		prefix = defaultResultStorePrefix
	}
	if ttl <= 0 {
		ttl = defaultResultStoreTTL
	}
	b.resultStorePrefix = prefix
	b.resultStoreTTL = ttl
}

// ConfigureRedisResultStore 使用 Redis 作为共享任务结果存储。
func (b *BaseService) ConfigureRedisResultStore(cfg *config.RedisConfig, logger *logrus.Logger, prefix string, ttl time.Duration) error {
	if cfg == nil {
		return nil
	}

	client, err := redisclient.New(cfg)
	if err != nil {
		return fmt.Errorf("create crawler result redis store: %w", err)
	}

	b.ConfigureSharedResultStore(client, prefix, ttl)
	if logger != nil {
		logger.Infof("已启用 crawler 异步任务共享结果存储: prefix=%s ttl=%s", b.resultStorePrefix, b.resultStoreTTL)
	}
	return nil
}

// Close 关闭共享资源。
func (b *BaseService) Close() error {
	if closer, ok := b.sharedResultStore.(interface{ Close() error }); ok {
		return closer.Close()
	}
	return nil
}

// StoreResult 存储初始结果。
// 当启用共享结果存储时，如果持久化失败，会返回错误以避免异步任务进入“已提交但跨 Pod 不可查询”的半成功状态。
func (b *BaseService) StoreResult(taskID string, result *CrawlerResult) error {
	b.results.Store(taskID, result)
	if err := b.persistSharedResult(taskID, result); err != nil {
		return err
	}
	return nil
}

// GetTask 获取任务结果
func (b *BaseService) GetTask(taskID string) (*CrawlerResult, error) {
	value, ok := b.results.Load(taskID)
	if ok {
		return value.(*CrawlerResult), nil
	}

	result, found, err := b.loadSharedResult(taskID)
	if err != nil {
		return nil, err
	}
	if !found {
		return nil, ErrTaskNotFound
	}

	b.results.Store(taskID, result)
	return result, nil
}

// DeleteTask 删除任务
func (b *BaseService) DeleteTask(taskID string) {
	b.results.Delete(taskID)
	if b.sharedResultStore != nil {
		_ = b.sharedResultStore.Delete(context.Background(), b.resultKey(taskID))
	}
}

// GetAllTasks 获取所有任务
func (b *BaseService) GetAllTasks() []*CrawlerResult {
	tasks := make([]*CrawlerResult, 0)
	b.results.Range(func(_, value any) bool {
		tasks = append(tasks, value.(*CrawlerResult))
		return true
	})
	return tasks
}

// UpdateResult 线程安全地更新任务结果。
func (b *BaseService) UpdateResult(taskID string, fn func(*CrawlerResult)) error {
	b.resultMu.Lock()
	defer b.resultMu.Unlock()

	result, found, err := b.loadResult(taskID)
	if err != nil || !found {
		return err
	}

	fn(result)
	b.results.Store(taskID, result)
	return b.persistSharedResult(taskID, result)
}

// GetStats 获取通用队列与指标统计
func (b *BaseService) GetStats() map[string]any {
	stats := map[string]any{
		"queue_size":      0,
		"queue_capacity":  0,
		"available_slots": 0,
		"usage_percent":   0.0,
	}
	if b.workerPool != nil {
		queueStats := b.workerPool.GetQueueStats()
		stats["queue_size"] = queueStats.QueueSize
		stats["queue_capacity"] = queueStats.BufferSize
		stats["available_slots"] = queueStats.AvailableSlots
		stats["usage_percent"] = queueStats.UsagePercent
	}

	statusCount := make(map[string]int)
	b.results.Range(func(_, value any) bool {
		statusCount[string(value.(*CrawlerResult).Status)]++
		return true
	})
	stats["status_count"] = statusCount

	if b.workerPool != nil {
		if metrics := b.workerPool.GetMetrics(); metrics != nil {
			snapshot := metrics.GetSnapshot()
			stats["total_submitted"] = snapshot.TotalSubmitted
			stats["total_processed"] = snapshot.TotalProcessed
			stats["total_succeeded"] = snapshot.TotalSucceeded
			stats["total_failed"] = snapshot.TotalFailed
			stats["total_panicked"] = snapshot.TotalPanicked
			stats["queue_full_count"] = snapshot.QueueFullCount
			stats["success_rate"] = snapshot.SuccessRate()
			stats["failure_rate"] = snapshot.FailureRate()
			stats["panic_rate"] = snapshot.PanicRate()
			stats["uptime"] = snapshot.Uptime.String()
		}
	}

	return stats
}

// IsReady 检查服务是否就绪
func (b *BaseService) IsReady() bool {
	return b.workerPool.AvailableSlots() > 0
}

// IsHealthy 检查服务是否健康
func (b *BaseService) IsHealthy() bool {
	return true
}

func (b *BaseService) loadResult(taskID string) (*CrawlerResult, bool, error) {
	if value, ok := b.results.Load(taskID); ok {
		return value.(*CrawlerResult), true, nil
	}
	return b.loadSharedResult(taskID)
}

func (b *BaseService) loadSharedResult(taskID string) (*CrawlerResult, bool, error) {
	if b.sharedResultStore == nil {
		return nil, false, nil
	}

	payload, err := b.sharedResultStore.Get(context.Background(), b.resultKey(taskID))
	if err != nil {
		if isSharedResultNotFound(err) {
			return nil, false, nil
		}
		return nil, false, err
	}

	var result CrawlerResult
	if err := json.Unmarshal([]byte(payload), &result); err != nil {
		return nil, false, err
	}
	return &result, true, nil
}

func (b *BaseService) persistSharedResult(taskID string, result *CrawlerResult) error {
	if b.sharedResultStore == nil || result == nil {
		return nil
	}

	payload, err := json.Marshal(result)
	if err != nil {
		return err
	}
	return b.sharedResultStore.Set(context.Background(), b.resultKey(taskID), string(payload), b.resultStoreTTL)
}

func (b *BaseService) resultKey(taskID string) string {
	prefix := b.resultStorePrefix
	if strings.TrimSpace(prefix) == "" {
		prefix = defaultResultStorePrefix
	}
	return fmt.Sprintf("%s:%s", prefix, taskID)
}

func isSharedResultNotFound(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, ErrTaskNotFound) {
		return true
	}
	return strings.Contains(err.Error(), "key not found:")
}
