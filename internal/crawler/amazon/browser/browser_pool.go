// Package browser 提供Amazon浏览器池管理功能
package browser

import (
	"context"
	"fmt"
	"sync"
	"task-processor/internal/core/config"
	sharedbrowser "task-processor/internal/crawler/shared/browser"
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
	config               *config.Config
	poolConfig           *BrowserPoolConfig
	instances            []*BrowserInstance
	available            chan *BrowserInstance
	Mu                   sync.Mutex
	fingerprintGen       *sharedbrowser.FingerprintGenerator
	useRandomFingerprint bool
	instanceManager      *InstanceManager
	healthChecker        *HealthChecker
	errorDetector        *ErrorDetector
	shutdownOnce         sync.Once // 确保只关闭一次
	closed               bool      // 标记是否已关闭
}

// BrowserPoolConfig 浏览器池配置
type BrowserPoolConfig struct {
	Size                 int    // 池大小
	MaxRetries           int    // 最大重试次数
	HealthCheckEnabled   bool   // 是否启用健康检查
	UseRandomFingerprint bool   // 是否使用随机指纹
	FingerprintStrategy  string // "random", "stable", "preset"
	PresetName           string // 预设配置名称
}

// DefaultBrowserPoolConfig 默认浏览器池配置
func DefaultBrowserPoolConfig() *BrowserPoolConfig {
	return &BrowserPoolConfig{
		Size:                 1,
		UseRandomFingerprint: true, // 默认启用随机指纹
		FingerprintStrategy:  "random",
		PresetName:           "windows_high_end",
	}
}

// NewBrowserPool 创建浏览器池
func NewBrowserPool(cfg *config.Config, poolConfig *BrowserPoolConfig) *BrowserPool {

	if poolConfig.Size == 0 {
		poolConfig.Size = 1 // 默认值
	}

	bp := &BrowserPool{
		config:               cfg,
		poolConfig:           poolConfig,
		instances:            make([]*BrowserInstance, 0, poolConfig.Size),
		available:            make(chan *BrowserInstance, poolConfig.Size),
		useRandomFingerprint: poolConfig.UseRandomFingerprint,
	}

	// 如果启用随机指纹，初始化指纹生成器
	if bp.useRandomFingerprint {
		bp.fingerprintGen = sharedbrowser.NewFingerprintGenerator()
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
	poolSize := bp.poolConfig.Size

	if poolSize == 0 {
		poolSize = 1 // 默认值
	}

	logrus.Infof("初始化浏览器池，大小: %d", poolSize)

	for i := 0; i < poolSize; i++ {
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
	if bp.poolConfig.HealthCheckEnabled {
		ctx := context.Background()
		bp.healthChecker.StartHealthCheckRoutine(ctx)
	}

	return nil
}

// Acquire 获取浏览器实例
func (bp *BrowserPool) Acquire() (*BrowserInstance, error) {
	bp.Mu.Lock()
	if bp.closed {
		bp.Mu.Unlock()
		return nil, fmt.Errorf("浏览器池已关闭")
	}
	bp.Mu.Unlock()

	select {
	case instance := <-bp.available:
		if instance == nil {
			return nil, fmt.Errorf("浏览器池已关闭")
		}
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
	if instance == nil {
		return
	}

	instance.Mu.Lock()
	instance.InUse = false
	instance.Mu.Unlock()

	bp.Mu.Lock()
	if bp.closed || bp.available == nil {
		bp.Mu.Unlock()
		logrus.Infof("浏览器池已关闭，无法释放实例 %d", instance.ID)
		return
	}
	bp.Mu.Unlock()

	logrus.Infof("释放浏览器实例 %d", instance.ID)

	// 使用 select 防止在关闭过程中阻塞
	select {
	case bp.available <- instance:
		// 成功释放
	default:
		// 通道已满或已关闭，忽略
		logrus.Warnf("无法释放浏览器实例 %d，通道可能已满或已关闭", instance.ID)
	}
}

// ReleaseWithError 释放浏览器实例（带错误检测）
func (bp *BrowserPool) ReleaseWithError(instance *BrowserInstance, err error) {
	if instance == nil {
		return
	}

	instance.Mu.Lock()
	instance.InUse = false
	instance.Mu.Unlock()

	// 检查池是否已关闭
	bp.Mu.Lock()
	if bp.closed {
		bp.Mu.Unlock()
		logrus.Infof("浏览器池已关闭，无法释放实例 %d", instance.ID)
		return
	}
	bp.Mu.Unlock()

	// 检测是否为风控或严重错误
	if bp.errorDetector.IsBlockedOrSeriousError(err) {
		logrus.Infof("检测到浏览器实例 %d 被风控或出现严重错误: %v", instance.ID, err)
		bp.instanceManager.RecreateInstanceAsync(instance)
		return
	}

	logrus.Infof("释放浏览器实例 %d", instance.ID)

	// 使用 select 防止在关闭过程中阻塞
	select {
	case bp.available <- instance:
		// 成功释放
	default:
		// 通道已满或已关闭，忽略
		logrus.Warnf("无法释放浏览器实例 %d，通道可能已满或已关闭", instance.ID)
	}
}

// RecreateInstanceSync 同步重新创建浏览器实例（用于任务内重试）
func (bp *BrowserPool) RecreateInstanceSync(oldInstance *BrowserInstance) *BrowserInstance {
	return bp.instanceManager.RecreateInstanceSync(oldInstance)
}

// Shutdown 关闭浏览器池
func (bp *BrowserPool) Shutdown() {
	bp.shutdownOnce.Do(func() {
		logrus.Info("关闭浏览器池...")

		bp.Mu.Lock()
		defer bp.Mu.Unlock()

		// 标记为已关闭
		bp.closed = true

		// 关闭所有浏览器实例
		for _, instance := range bp.instances {
			if instance.Manager != nil {
				instance.Manager.Close()
			}
		}

		// 清空实例列表
		bp.instances = nil

		// 关闭可用实例通道
		if bp.available != nil {
			close(bp.available)
			bp.available = nil
		}

		logrus.Info("浏览器池已关闭")
	})
}

// GetPoolStats 获取浏览器池统计信息
func (bp *BrowserPool) GetPoolStats() map[string]any {
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
func (bp *BrowserPool) GetConfig() *config.Config {
	return bp.config
}

// GetFingerprintGenerator 获取指纹生成器（用于内部管理器访问）
func (bp *BrowserPool) GetFingerprintGenerator() *sharedbrowser.FingerprintGenerator {
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
