// Package browser 提供浏览器健康检查功能
package browser

import (
	"context"
	"time"

	"github.com/sirupsen/logrus"
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

// HealthCheck 检查浏览器实例健康状态
func (hc *HealthChecker) HealthCheck(instance *BrowserInstance) bool {
	if instance == nil || instance.Manager == nil || instance.Page == nil {
		return false
	}

	// 尝试执行简单的JavaScript来检查页面是否响应
	_, err := instance.Page.Evaluate("() => document.readyState")
	if err != nil {
		logrus.Infof("浏览器实例 %d 健康检查失败: %v", instance.ID, err)
		return false
	}

	return true
}

// GetPoolStats 获取浏览器池统计信息
func (hc *HealthChecker) GetPoolStats() map[string]interface{} {
	hc.pool.Mu.Lock()
	defer hc.pool.Mu.Unlock()

	instances := hc.pool.GetInstances()
	available := hc.pool.GetAvailableChannel()

	stats := map[string]interface{}{
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
	logrus.Infof("📊 浏览器池状态: 总计=%d, 可用=%d, 使用中=%d, 健康=%d",
		stats["total_instances"], stats["available_instances"],
		stats["in_use_instances"], stats["healthy_instances"])
}

// StartHealthCheckRoutine 启动定期健康检查
func (hc *HealthChecker) StartHealthCheckRoutine(ctx context.Context) {
	go func() {
		defer func() {
			if r := recover(); r != nil {
				logrus.Errorf("浏览器池健康检查goroutine panic: %v", r)
			}
		}()

		ticker := time.NewTicker(5 * time.Minute) // 每5分钟检查一次
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				logrus.Info("浏览器池健康检查例程停止")
				return
			case <-ticker.C:
				hc.performHealthCheck()
			}
		}
	}()

	logrus.Info("🏥 浏览器池健康检查例程已启动")
}

// performHealthCheck 执行健康检查
func (hc *HealthChecker) performHealthCheck() {
	logrus.Info("🔍 开始浏览器池健康检查...")

	hc.LogPoolStats()

	unhealthyInstances := make([]*BrowserInstance, 0)

	// 检查所有实例的健康状态
	hc.pool.Mu.Lock()
	instances := hc.pool.GetInstances()
	for _, instance := range instances {
		instance.Mu.Lock()
		isInUse := instance.InUse
		instance.Mu.Unlock()

		// 只检查未在使用的实例
		if !isInUse && !hc.HealthCheck(instance) {
			unhealthyInstances = append(unhealthyInstances, instance)
		}
	}
	hc.pool.Mu.Unlock()

	// 重建不健康的实例
	for _, instance := range unhealthyInstances {
		logrus.Infof("⚠️ 发现不健康的浏览器实例 %d，准备重建", instance.ID)
		hc.pool.instanceManager.RecreateInstanceAsync(instance)
	}

	if len(unhealthyInstances) == 0 {
		logrus.Info("✅ 所有浏览器实例健康状态良好")
	} else {
		logrus.Infof("🔧 发现 %d 个不健康实例，已启动重建", len(unhealthyInstances))
	}
}

// CheckInstanceHealth 检查单个实例健康状态
func (hc *HealthChecker) CheckInstanceHealth(instance *BrowserInstance) map[string]interface{} {
	if instance == nil {
		return map[string]interface{}{
			"healthy": false,
			"error":   "instance is nil",
		}
	}

	result := map[string]interface{}{
		"instance_id": instance.ID,
		"healthy":     false,
		"checks":      make(map[string]bool),
	}

	checks := result["checks"].(map[string]bool)

	// 检查管理器
	checks["manager_exists"] = instance.Manager != nil

	// 检查页面
	checks["page_exists"] = instance.Page != nil

	// 检查页面响应
	if instance.Page != nil {
		_, err := instance.Page.Evaluate("() => document.readyState")
		checks["page_responsive"] = err == nil
		if err != nil {
			result["page_error"] = err.Error()
		}
	} else {
		checks["page_responsive"] = false
	}

	// 综合判断健康状态
	result["healthy"] = checks["manager_exists"] && checks["page_exists"] && checks["page_responsive"]

	return result
}

// GetDetailedStats 获取详细统计信息
func (hc *HealthChecker) GetDetailedStats() map[string]interface{} {
	hc.pool.Mu.Lock()
	defer hc.pool.Mu.Unlock()

	instances := hc.pool.GetInstances()
	available := hc.pool.GetAvailableChannel()

	stats := map[string]interface{}{
		"total_instances":     len(instances),
		"available_instances": len(available),
		"instance_details":    make([]map[string]interface{}, 0),
	}

	var inUseCount, healthyCount int
	instanceDetails := stats["instance_details"].([]map[string]interface{})

	for _, instance := range instances {
		instance.Mu.Lock()
		isInUse := instance.InUse
		instance.Mu.Unlock()

		if isInUse {
			inUseCount++
		}

		healthCheck := hc.CheckInstanceHealth(instance)
		if healthCheck["healthy"].(bool) {
			healthyCount++
		}

		instanceDetails = append(instanceDetails, healthCheck)
	}

	stats["in_use_instances"] = inUseCount
	stats["healthy_instances"] = healthyCount
	stats["instance_details"] = instanceDetails

	return stats
}

// MonitorPool 监控浏览器池状态
func (hc *HealthChecker) MonitorPool(ctx context.Context, interval time.Duration) {
	go func() {
		defer func() {
			if r := recover(); r != nil {
				logrus.Errorf("浏览器池监控goroutine panic: %v", r)
			}
		}()

		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				logrus.Info("浏览器池监控停止")
				return
			case <-ticker.C:
				stats := hc.GetDetailedStats()
				logrus.Infof("🔍 浏览器池监控: %+v", stats)
			}
		}
	}()

	logrus.Infof("📊 浏览器池监控已启动，间隔: %v", interval)
}

// WaitForHealthyPool 等待浏览器池达到健康状态
func (hc *HealthChecker) WaitForHealthyPool(timeout time.Duration) bool {
	deadline := time.Now().Add(timeout)

	for time.Now().Before(deadline) {
		stats := hc.GetPoolStats()
		totalInstances := stats["total_instances"].(int)
		healthyInstances := stats["healthy_instances"].(int)

		if totalInstances > 0 && healthyInstances == totalInstances {
			logrus.Info("✅ 浏览器池已达到健康状态")
			return true
		}

		logrus.Infof("⏳ 等待浏览器池健康状态: %d/%d", healthyInstances, totalInstances)
		time.Sleep(2 * time.Second)
	}

	logrus.Warn("⚠️ 等待浏览器池健康状态超时")
	return false
}
