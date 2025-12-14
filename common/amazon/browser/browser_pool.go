package browser

import (
	"fmt"
	"strings"
	"sync"
	"task-processor/common/amazon/model"
	"task-processor/common/config"
	"time"

	"github.com/playwright-community/playwright-go"
	"github.com/sirupsen/logrus"
)

// BrowserInstance 浏览器实例
type BrowserInstance struct {
	ID             int
	Manager        *BrowserManager
	Page           playwright.Page
	InUse          bool
	CurrentZipcode string // 当前设置的邮编，用于避免重复设置
	Mu             sync.Mutex
}

// BrowserPool 浏览器池
type BrowserPool struct {
	config               *config.AmazonConfig
	poolConfig           *BrowserPoolConfig
	instances            []*BrowserInstance
	available            chan *BrowserInstance
	Mu                   sync.Mutex
	fingerprintGen       *FingerprintGenerator
	useRandomFingerprint bool
}

// BrowserPoolConfig 浏览器池配置
type BrowserPoolConfig struct {
	PoolSize             int
	UseRandomFingerprint bool
}

// FingerprintGenerator 指纹生成器（简化版）
type FingerprintGenerator struct{}

// NewFingerprintGenerator 创建指纹生成器
func NewFingerprintGenerator() *FingerprintGenerator {
	return &FingerprintGenerator{}
}

// GenerateFingerprint 生成指纹
func (fg *FingerprintGenerator) GenerateFingerprint(userID string) *FingerprintConfig {
	return &FingerprintConfig{
		Enable: true,
		GPU: map[string]interface{}{
			"description": "NVIDIA GeForce GTX 1060",
		},
	}
}

// DefaultBrowserPoolConfig 默认浏览器池配置
func DefaultBrowserPoolConfig() *BrowserPoolConfig {
	return &BrowserPoolConfig{
		PoolSize:             3,
		UseRandomFingerprint: true, // 默认启用随机指纹
	}
}

// NewBrowserPool 创建浏览器池
func NewBrowserPool(cfg *config.AmazonConfig, poolConfig *BrowserPoolConfig) *BrowserPool {
	bp := &BrowserPool{
		config:               cfg,
		poolConfig:           poolConfig,
		instances:            make([]*BrowserInstance, 0, poolConfig.PoolSize),
		available:            make(chan *BrowserInstance, poolConfig.PoolSize),
		useRandomFingerprint: poolConfig.UseRandomFingerprint,
	}

	// 如果启用随机指纹，初始化指纹生成器
	if bp.useRandomFingerprint {
		bp.fingerprintGen = NewFingerprintGenerator()
		logrus.Info("浏览器池启用随机指纹生成")
	}

	return bp
}

// Initialize 初始化浏览器池
func (bp *BrowserPool) Initialize() error {
	logrus.Infof("初始化浏览器池，大小: %d", bp.poolConfig.PoolSize)

	for i := 0; i < bp.poolConfig.PoolSize; i++ {
		instance, err := bp.createInstance(i)
		if err != nil {
			logrus.Infof("创建浏览器实例 %d 失败: %v", i, err)
			// 清理已创建的实例
			bp.Shutdown()
			return fmt.Errorf("初始化浏览器池失败: %w", err)
		}

		bp.instances = append(bp.instances, instance)
		bp.available <- instance

		logrus.Infof("浏览器实例 %d 创建成功", i)
	}

	logrus.Infof("浏览器池初始化完成，共 %d 个实例", len(bp.instances))

	// 启动健康检查例程
	bp.StartHealthCheckRoutine()

	return nil
}

// createInstance 创建浏览器实例
func (bp *BrowserPool) createInstance(id int) (*BrowserInstance, error) {
	manager := NewBrowserManager(bp.config)

	// 如果启用随机指纹，为每个实例生成唯一指纹
	if bp.useRandomFingerprint && bp.fingerprintGen != nil {
		userID := fmt.Sprintf("instance_%d_%d", id, time.Now().UnixNano())
		randomFingerprint := bp.fingerprintGen.GenerateFingerprint(userID)
		manager.SetFingerprint(randomFingerprint)
		logrus.Infof("为浏览器实例 %d 生成随机指纹", id)
	}

	if err := manager.Install(); err != nil {
		return nil, fmt.Errorf("初始化playwright失败: %w", err)
	}

	if err := manager.Launch(); err != nil {
		return nil, fmt.Errorf("启动浏览器失败: %w", err)
	}

	page, err := manager.NewPage()
	if err != nil {
		manager.Close()
		return nil, fmt.Errorf("创建页面失败: %w", err)
	}

	return &BrowserInstance{
		ID:      id,
		Manager: manager,
		Page:    page,
		InUse:   false,
	}, nil
}

// Acquire 获取浏览器实例
func (bp *BrowserPool) Acquire() (*BrowserInstance, error) {
	select {
	case instance := <-bp.available:
		instance.Mu.Lock()
		instance.InUse = true
		instance.Mu.Unlock()
		logrus.Infof("获取浏览器实例 %d", instance.ID)
		return instance, nil
	case <-time.After(30 * time.Second):
		return nil, fmt.Errorf("获取浏览器实例超时")
	}
}

// Release 释放浏览器实例
func (bp *BrowserPool) Release(instance *BrowserInstance) {
	instance.Mu.Lock()
	instance.InUse = false
	instance.Mu.Unlock()

	logrus.Infof("释放浏览器实例 %d", instance.ID)
	bp.available <- instance
}

// ReleaseWithError 释放浏览器实例（带错误检测）
func (bp *BrowserPool) ReleaseWithError(instance *BrowserInstance, err error) {
	instance.Mu.Lock()
	instance.InUse = false
	instance.Mu.Unlock()

	// 检测是否为风控或严重错误
	if bp.IsBlockedOrSeriousError(err) {
		logrus.Infof("检测到浏览器实例 %d 被风控或出现严重错误: %v", instance.ID, err)
		bp.recreateInstance(instance)
		return
	}

	logrus.Infof("释放浏览器实例 %d", instance.ID)
	bp.available <- instance
}

// IsBlockedOrSeriousError 检测是否为风控或严重错误
func (bp *BrowserPool) IsBlockedOrSeriousError(err error) bool {
	if err == nil {
		return false
	}

	// 检查是否为产品不存在错误（不应触发浏览器重建）
	if _, ok := err.(*model.ProductNotFoundError); ok {
		return false
	}

	errorStr := err.Error()

	// 如果错误信息包含"产品页面不存在"或"产品页面缺少必要元素"，不触发重建
	if strings.Contains(errorStr, "产品页面不存在") || strings.Contains(errorStr, "产品页面缺少必要元素") {
		return false
	}

	// 检测常见的风控和严重错误模式
	blockPatterns := []string{
		"SIGN_IN_REQUIRED", // 需要登录才能更新位置
		"timeout", "Timeout", "TIMEOUT",
		"blocked", "Blocked", "BLOCKED",
		"captcha", "CAPTCHA", "Captcha",
		"robot", "Robot", "ROBOT",
		"access denied", "Access Denied", "ACCESS DENIED",
		"forbidden", "Forbidden", "FORBIDDEN",
		"503", "502", "504", // 服务器错误
		"connection refused", "Connection refused",
		"network error", "Network error",
		"page crashed", "Page crashed",
		"browser disconnected", "Browser disconnected",
		"context closed", "Context closed",
		"navigation failed", "Navigation failed",
		"Timeout 30000ms exceeded", // 特定的超时错误
	}

	for _, pattern := range blockPatterns {
		if strings.Contains(errorStr, pattern) {
			return true
		}
	}

	return false
}

// RecreateInstanceSync 同步重新创建浏览器实例（用于任务内重试）
func (bp *BrowserPool) RecreateInstanceSync(oldInstance *BrowserInstance) *BrowserInstance {
	logrus.Infof("开始同步重新创建浏览器实例 %d", oldInstance.ID)

	// 先关闭旧实例
	if oldInstance.Manager != nil {
		oldInstance.Manager.Close()
		logrus.Infof("已关闭出现问题的浏览器实例 %d", oldInstance.ID)
	}

	// 等待短暂时间，让资源释放
	time.Sleep(2 * time.Second)

	// 创建新实例
	newInstance, err := bp.createInstance(oldInstance.ID)
	if err != nil {
		logrus.Errorf("同步重新创建浏览器实例 %d 失败: %v", oldInstance.ID, err)

		// 如果创建失败，等待更长时间后再次尝试
		logrus.Infof("等待5秒后进行第二次创建尝试...")
		time.Sleep(5 * time.Second)

		newInstance, err = bp.createInstance(oldInstance.ID)
		if err != nil {
			logrus.Errorf("第二次同步重新创建浏览器实例 %d 失败: %v", oldInstance.ID, err)
			// 返回nil，让调用方处理
			return nil
		}
	}

	// 更新实例列表
	bp.Mu.Lock()
	for i, inst := range bp.instances {
		if inst.ID == oldInstance.ID {
			bp.instances[i] = newInstance
			break
		}
	}
	bp.Mu.Unlock()

	logrus.Infof("✅ 成功同步重新创建浏览器实例 %d", oldInstance.ID)
	return newInstance
}

// recreateInstance 异步重新创建浏览器实例（用于后台健康检查）
func (bp *BrowserPool) recreateInstance(oldInstance *BrowserInstance) {
	logrus.Infof("开始异步重新创建浏览器实例 %d", oldInstance.ID)

	// 异步重新创建实例，避免阻塞
	go func() {
		// 先关闭旧实例
		if oldInstance.Manager != nil {
			oldInstance.Manager.Close()
			logrus.Infof("已关闭被风控的浏览器实例 %d", oldInstance.ID)
		}

		// 等待一段时间再重新创建，避免立即重试
		time.Sleep(5 * time.Second)

		// 创建新实例
		newInstance, err := bp.createInstance(oldInstance.ID)
		if err != nil {
			logrus.Infof("重新创建浏览器实例 %d 失败: %v", oldInstance.ID, err)

			// 如果创建失败，等待更长时间后再次尝试
			time.Sleep(30 * time.Second)
			newInstance, err = bp.createInstance(oldInstance.ID)
			if err != nil {
				logrus.Infof("第二次重新创建浏览器实例 %d 失败: %v", oldInstance.ID, err)
				// 如果还是失败，可以考虑通知管理员或采取其他措施
				return
			}
		}

		// 更新实例列表
		bp.Mu.Lock()
		for i, inst := range bp.instances {
			if inst.ID == oldInstance.ID {
				bp.instances[i] = newInstance
				break
			}
		}
		bp.Mu.Unlock()

		// 将新实例放回可用池
		bp.available <- newInstance
		logrus.Infof("✅ 成功异步重新创建浏览器实例 %d", oldInstance.ID)
	}()
}

// Shutdown 关闭浏览器池
func (bp *BrowserPool) Shutdown() {
	logrus.Info("关闭浏览器池...")

	bp.Mu.Lock()
	defer bp.Mu.Unlock()

	for _, instance := range bp.instances {
		if instance.Manager != nil {
			instance.Manager.Close()
		}
	}

	bp.instances = nil
	close(bp.available)

	logrus.Info("浏览器池已关闭")
}

// HealthCheck 检查浏览器实例健康状态
func (bp *BrowserPool) HealthCheck(instance *BrowserInstance) bool {
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
func (bp *BrowserPool) GetPoolStats() map[string]interface{} {
	bp.Mu.Lock()
	defer bp.Mu.Unlock()

	stats := map[string]interface{}{
		"total_instances":     len(bp.instances),
		"available_instances": len(bp.available),
		"in_use_instances":    0,
		"healthy_instances":   0,
	}

	inUseCount := 0
	healthyCount := 0

	for _, instance := range bp.instances {
		instance.Mu.Lock()
		if instance.InUse {
			inUseCount++
		}
		instance.Mu.Unlock()

		if bp.HealthCheck(instance) {
			healthyCount++
		}
	}

	stats["in_use_instances"] = inUseCount
	stats["healthy_instances"] = healthyCount

	return stats
}

// LogPoolStats 记录浏览器池统计信息
func (bp *BrowserPool) LogPoolStats() {
	stats := bp.GetPoolStats()
	logrus.Infof("📊 浏览器池状态: 总计=%d, 可用=%d, 使用中=%d, 健康=%d",
		stats["total_instances"], stats["available_instances"],
		stats["in_use_instances"], stats["healthy_instances"])
}

// StartHealthCheckRoutine 启动定期健康检查
func (bp *BrowserPool) StartHealthCheckRoutine() {
	go func() {
		ticker := time.NewTicker(5 * time.Minute) // 每5分钟检查一次
		defer ticker.Stop()

		for range ticker.C {
			bp.performHealthCheck()
		}
	}()

	logrus.Info("🏥 浏览器池健康检查例程已启动")
}

// performHealthCheck 执行健康检查
func (bp *BrowserPool) performHealthCheck() {
	logrus.Info("🔍 开始浏览器池健康检查...")

	bp.LogPoolStats()

	unhealthyInstances := make([]*BrowserInstance, 0)

	// 检查所有实例的健康状态
	bp.Mu.Lock()
	for _, instance := range bp.instances {
		instance.Mu.Lock()
		isInUse := instance.InUse
		instance.Mu.Unlock()

		// 只检查未在使用的实例
		if !isInUse && !bp.HealthCheck(instance) {
			unhealthyInstances = append(unhealthyInstances, instance)
		}
	}
	bp.Mu.Unlock()

	// 重建不健康的实例
	for _, instance := range unhealthyInstances {
		logrus.Infof("⚠️ 发现不健康的浏览器实例 %d，准备重建", instance.ID)
		bp.recreateInstance(instance)
	}

	if len(unhealthyInstances) == 0 {
		logrus.Info("✅ 所有浏览器实例健康状态良好")
	} else {
		logrus.Infof("🔧 发现 %d 个不健康实例，已启动重建", len(unhealthyInstances))
	}
}
