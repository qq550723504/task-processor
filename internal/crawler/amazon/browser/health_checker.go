// Package browser 提供浏览器健康检查功能
package browser

import (
	"context"
	"task-processor/internal/core/logger"
	"time"
)

// HealthChecker 健康检查器
type HealthChecker struct {
	pool *BrowserPool
}

// NewHealthChecker 创建健康检查器
func NewHealthChecker(pool *BrowserPool) *HealthChecker {
	return &HealthChecker{
		pool: pool,
	}
}

// HealthCheck 检查浏览器实例健康状态。
// Page.Evaluate 在 WebSocket 断连时可能永久 hang，使用 5s 超时保护。
func (hc *HealthChecker) HealthCheck(instance *BrowserInstance) bool {
	if instance == nil || instance.Manager == nil || instance.Page == nil {
		return false
	}

	type evalResult struct{ err error }
	ch := make(chan evalResult, 1)
	go func() {
		_, err := instance.Page.Evaluate("() => document.readyState")
		ch <- evalResult{err}
	}()

	select {
	case res := <-ch:
		if res.err != nil {
			logger.GetGlobalLogger("crawler/amazon").Infof("浏览器实例 %d 健康检查失败: %v", instance.ID, res.err)
			return false
		}
		return true
	case <-time.After(5 * time.Second):
		logger.GetGlobalLogger("crawler/amazon").Warnf("浏览器实例 %d 健康检查超时(5s)", instance.ID)
		return false
	}
}

// GetPoolStats 获取浏览器池统计信息。
// 先取快照（锁内），再在锁外做 HealthCheck，避免持锁期间调用 Playwright 操作导致死锁。
func (hc *HealthChecker) GetPoolStats() map[string]any {
	instances := hc.pool.GetInstancesSnapshot()
	available := hc.pool.GetAvailableChannel()

	stats := map[string]any{
		"total_instances":     len(instances),
		"available_instances": len(available),
		"in_use_instances":    0,
		"healthy_instances":   0,
	}

	inUseCount := 0
	healthyCount := 0

	for _, instance := range instances {
		instance.Mu.Lock()
		if instance.InUse {
			inUseCount++
		}
		instance.Mu.Unlock()

		if hc.HealthCheck(instance) {
			healthyCount++
		}
	}

	stats["in_use_instances"] = inUseCount
	stats["healthy_instances"] = healthyCount

	return stats
}

// LogPoolStats 记录浏览器池统计信息
func (hc *HealthChecker) LogPoolStats() {
	stats := hc.GetPoolStats()
	logger.GetGlobalLogger("crawler/amazon").Infof("📊 浏览器池状态: 总计=%d, 可用=%d, 使用中=%d, 健康=%d",
		stats["total_instances"], stats["available_instances"],
		stats["in_use_instances"], stats["healthy_instances"])
}

// StartHealthCheckRoutine 启动定期健康检查
func (hc *HealthChecker) StartHealthCheckRoutine(ctx context.Context) {
	go func() {
		defer func() {
			if r := recover(); r != nil {
				logger.GetGlobalLogger("crawler/amazon").Errorf("浏览器池健康检查goroutine panic: %v", r)
			}
		}()

		ticker := time.NewTicker(5 * time.Minute) // 每5分钟检查一次
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				logger.GetGlobalLogger("crawler/amazon").Info("浏览器池健康检查例程停止")
				return
			case <-ticker.C:
				hc.performHealthCheck()
			}
		}
	}()

	logger.GetGlobalLogger("crawler/amazon").Info("🏥 浏览器池健康检查例程已启动")
}

// performHealthCheck 执行健康检查。
// 先取快照（锁内），再在锁外做 HealthCheck，避免持锁期间调用 Playwright 操作导致死锁。
func (hc *HealthChecker) performHealthCheck() {
	logger.GetGlobalLogger("crawler/amazon").Info("🔍 开始浏览器池健康检查...")

	hc.LogPoolStats()

	unhealthyInstances := make([]*BrowserInstance, 0)

	// 取快照后立即释放锁，再在锁外做 HealthCheck
	instances := hc.pool.GetInstancesSnapshot()
	for _, instance := range instances {
		instance.Mu.Lock()
		isInUse := instance.InUse
		instance.Mu.Unlock()

		// 只检查未在使用的实例
		if !isInUse && !hc.HealthCheck(instance) {
			unhealthyInstances = append(unhealthyInstances, instance)
		}
	}

	// 重建不健康的实例
	for _, instance := range unhealthyInstances {
		logger.GetGlobalLogger("crawler/amazon").Infof("⚠️ 发现不健康的浏览器实例 %d，准备重建", instance.ID)

		// 必须先从 available channel 中取出该实例，否则池里会有僵尸实例占位，
		// 同时异步重建的新实例也无法放回（channel 已满），导致池永久缩容。
		// 使用非阻塞 select 尝试取出，取不到说明实例已被其他 goroutine 取走，跳过。
		drained := false
		availCh := hc.pool.GetAvailableChannel()
		for {
			select {
			case candidate, ok := <-availCh:
				if !ok {
					// channel 已关闭，池正在关闭，直接返回
					return
				}
				if candidate.ID == instance.ID {
					// 成功取出目标实例，标记后重建
					candidate.Mu.Lock()
					candidate.InUse = true
					candidate.Mu.Unlock()
					drained = true
				} else {
					// 取出的是其他实例，放回去
					select {
					case availCh <- candidate:
					default:
					}
				}
			default:
				// channel 暂时为空，停止尝试
				goto drainDone
			}
			if drained {
				break
			}
		}
	drainDone:
		if drained {
			hc.pool.instanceManager.RecreateInstanceAsync(instance)
		} else {
			logger.GetGlobalLogger("crawler/amazon").Infof("浏览器实例 %d 已被取走，跳过重建", instance.ID)
		}
	}

	if len(unhealthyInstances) == 0 {
		logger.GetGlobalLogger("crawler/amazon").Info("✅ 所有浏览器实例健康状态良好")
	} else {
		logger.GetGlobalLogger("crawler/amazon").Infof("🔧 发现 %d 个不健康实例，已启动重建", len(unhealthyInstances))
	}
}
