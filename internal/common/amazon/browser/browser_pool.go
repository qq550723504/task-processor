// Package browser 提供Amazon浏览器池管理功能
package browser

import (
	"context"
	"fmt"
	"sync"
	"task-processor/internal/core/config"
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
	instanceManager      *InstanceManager
	healthChecker        *HealthChecker
	errorDetector        *ErrorDetector
}

// BrowserPoolConfig 浏览器池配置
type BrowserPoolConfig struct {
	PoolSize             int
	UseRandomFingerprint bool
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

	// 初始化各个管理器
	bp.instanceManager = NewInstanceManager(bp)
	bp.healthChecker = NewHealthChecker(bp)
	bp.errorDetector = NewErrorDetector()

	return bp
}

// Initialize 初始化浏览器池
func (bp *BrowserPool) Initialize() error {
	logrus.Infof("初始化浏览器池，大小: %d", bp.poolConfig.PoolSize)

	for i := 0; i < bp.poolConfig.PoolSize; i++ {
		instance, err := bp.instanceManager.CreateInstance(i)
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

	// 启动健康检查例程 - 使用长期运行的context
	ctx := context.Background()
	bp.healthChecker.StartHealthCheckRoutine(ctx)

	return nil
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
	if bp.errorDetector.IsBlockedOrSeriousError(err) {
		logrus.Infof("检测到浏览器实例 %d 被风控或出现严重错误: %v", instance.ID, err)
		bp.instanceManager.RecreateInstanceAsync(instance)
		return
	}

	logrus.Infof("释放浏览器实例 %d", instance.ID)
	bp.available <- instance
}

// RecreateInstanceSync 同步重新创建浏览器实例（用于任务内重试）
func (bp *BrowserPool) RecreateInstanceSync(oldInstance *BrowserInstance) *BrowserInstance {
	return bp.instanceManager.RecreateInstanceSync(oldInstance)
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

// GetPoolStats 获取浏览器池统计信息
func (bp *BrowserPool) GetPoolStats() map[string]interface{} {
	return bp.healthChecker.GetPoolStats()
}

// LogPoolStats 记录浏览器池统计信息
func (bp *BrowserPool) LogPoolStats() {
	bp.healthChecker.LogPoolStats()
}

// IsBlockedOrSeriousError 检测是否为风控或严重错误（公开方法）
func (bp *BrowserPool) IsBlockedOrSeriousError(err error) bool {
	return bp.errorDetector.IsBlockedOrSeriousError(err)
}

// GetInstances 获取所有实例（用于内部管理器访问）
func (bp *BrowserPool) GetInstances() []*BrowserInstance {
	bp.Mu.Lock()
	defer bp.Mu.Unlock()
	return bp.instances
}

// GetAvailableChannel 获取可用实例通道（用于内部管理器访问）
func (bp *BrowserPool) GetAvailableChannel() chan *BrowserInstance {
	return bp.available
}

// GetConfig 获取配置（用于内部管理器访问）
func (bp *BrowserPool) GetConfig() *config.AmazonConfig {
	return bp.config
}

// GetFingerprintGenerator 获取指纹生成器（用于内部管理器访问）
func (bp *BrowserPool) GetFingerprintGenerator() *FingerprintGenerator {
	return bp.fingerprintGen
}

// UseRandomFingerprint 是否使用随机指纹（用于内部管理器访问）
func (bp *BrowserPool) UseRandomFingerprint() bool {
	return bp.useRandomFingerprint
}

// UpdateInstance 更新实例（用于内部管理器访问）
func (bp *BrowserPool) UpdateInstance(oldInstance, newInstance *BrowserInstance) {
	bp.Mu.Lock()
	defer bp.Mu.Unlock()

	for i, inst := range bp.instances {
		if inst.ID == oldInstance.ID {
			bp.instances[i] = newInstance
			break
		}
	}
}
