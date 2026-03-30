// Package browser 提供Amazon浏览器池管理功能
package browser

import (
	"context"
	"fmt"
	"sync"
	"task-processor/internal/core/config"
	"task-processor/internal/core/logger"
	sharedbrowser "task-processor/internal/crawler/shared/browser"

	"github.com/playwright-community/playwright-go"
)

// BrowserInstance 浏览器实例
type BrowserInstance struct {
	ID             int
	Manager        *BrowserManager
	Page           playwright.Page
	InUse          bool
	Closed         bool   // 已被超时路径关闭，不可再归还池
	CurrentZipcode string // 当前设置的邮编，用于避免重复设置
	CurrentProxy   string
	Mu             sync.Mutex
	riskState      instanceRiskState
}

// instanceRebuilder 定义实例重建行为，便于测试注入 mock
type instanceRebuilder interface {
	CreateInstance(id int) (*BrowserInstance, error)
	RecreateInstanceSync(old *BrowserInstance) *BrowserInstance
	RecreateInstanceAsync(old *BrowserInstance)
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
	instanceManager      instanceRebuilder
	healthChecker        *HealthChecker
	errorDetector        *ErrorDetector
	riskPolicy           *riskPolicy
	proxyPool            *sharedbrowser.ProxyPool
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
		logger.GetGlobalLogger("crawler/amazon").Info("浏览器池启用随机指纹生成")
	}

	// 初始化各个管理器
	bp.instanceManager = NewInstanceManager(bp)
	bp.healthChecker = NewHealthChecker(bp)
	bp.errorDetector = NewErrorDetector()
	bp.riskPolicy = newRiskPolicy(cfg, bp.errorDetector)
	bp.proxyPool = newProxyPool(cfg)

	return bp
}

// Initialize 初始化浏览器池
func (bp *BrowserPool) Initialize() error {
	poolSize := bp.poolConfig.Size

	if poolSize == 0 {
		poolSize = 1 // 默认值
	}

	logger.GetGlobalLogger("crawler/amazon").Infof("初始化浏览器池，大小: %d", poolSize)

	for i := 0; i < poolSize; i++ {
		instance, err := bp.instanceManager.CreateInstance(i)
		if err != nil {
			logger.GetGlobalLogger("crawler/amazon").Infof("创建浏览器实例 %d 失败: %v", i, err)
			// 清理已创建的实例
			bp.Shutdown()
			return fmt.Errorf("初始化浏览器池失败: %w", err)
		}

		bp.instances = append(bp.instances, instance)
		bp.available <- instance

		logger.GetGlobalLogger("crawler/amazon").Infof("浏览器实例 %d 创建成功", i)
	}

	logger.GetGlobalLogger("crawler/amazon").Infof("浏览器池初始化完成，共 %d 个实例", len(bp.instances))

	// 启动健康检查例程 - 使用长期运行的context
	if bp.poolConfig.HealthCheckEnabled {
		ctx := context.Background()
		bp.healthChecker.StartHealthCheckRoutine(ctx)
	}

	return nil
}

// Acquire 获取浏览器实例（阻塞直到有可用实例，超时由调用方通过 context 控制）
func (bp *BrowserPool) Acquire() (*BrowserInstance, error) {
	bp.Mu.Lock()
	if bp.closed {
		bp.Mu.Unlock()
		return nil, fmt.Errorf("浏览器池已关闭")
	}
	bp.Mu.Unlock()

	instance, ok := <-bp.available
	if !ok || instance == nil {
		return nil, fmt.Errorf("浏览器池已关闭")
	}
	instance.Mu.Lock()
	instance.InUse = true
	instance.Mu.Unlock()
	logger.GetGlobalLogger("crawler/amazon").Infof("获取浏览器实例 %d", instance.ID)
	return instance, nil
}

// Release 释放浏览器实例
func (bp *BrowserPool) Release(instance *BrowserInstance) {
	if instance == nil {
		return
	}

	instance.Mu.Lock()
	instance.InUse = false
	instance.Mu.Unlock()
	if bp.proxyPool != nil && instance.CurrentProxy != "" {
		bp.proxyPool.MarkSuccess(instance.CurrentProxy)
	}
	if bp.riskPolicy != nil {
		bp.riskPolicy.OnSuccess(instance)
	}

	bp.Mu.Lock()
	if bp.closed || bp.available == nil {
		bp.Mu.Unlock()
		logger.GetGlobalLogger("crawler/amazon").Infof("浏览器池已关闭，无法释放实例 %d", instance.ID)
		return
	}
	bp.Mu.Unlock()

	logger.GetGlobalLogger("crawler/amazon").Infof("释放浏览器实例 %d", instance.ID)

	// 使用 select 防止在关闭过程中阻塞
	select {
	case bp.available <- instance:
		// 成功释放
	default:
		// 通道已满或已关闭，忽略
		logger.GetGlobalLogger("crawler/amazon").Warnf("无法释放浏览器实例 %d，通道可能已满或已关闭", instance.ID)
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
	if bp.proxyPool != nil && instance.CurrentProxy != "" {
		bp.proxyPool.MarkFailure(instance.CurrentProxy)
	}

	// 检查池是否已关闭
	bp.Mu.Lock()
	if bp.closed {
		bp.Mu.Unlock()
		logger.GetGlobalLogger("crawler/amazon").Infof("浏览器池已关闭，无法释放实例 %d", instance.ID)
		return
	}
	bp.Mu.Unlock()

	// 检测是否为风控或严重错误
	if bp.ShouldRecreateAfterFailure(instance, err) {
		logger.GetGlobalLogger("crawler/amazon").Infof("检测到浏览器实例 %d 被风控或出现严重错误: %v", instance.ID, err)
		bp.instanceManager.RecreateInstanceAsync(instance)
		return
	}

	// 使用 select 防止在关闭过程中阻塞
	select {
	case bp.available <- instance:
		// 成功释放
	default:
		logger.GetGlobalLogger("crawler/amazon").Warnf("无法释放浏览器实例 %d，通道可能已满或已关闭", instance.ID)
	}
}

func (bp *BrowserPool) ShouldRecreateAfterFailure(instance *BrowserInstance, err error) bool {
	if err == nil {
		return false
	}
	if bp.riskPolicy != nil {
		return bp.riskPolicy.OnFailure(instance, err)
	}
	return bp.errorDetector != nil && bp.errorDetector.IsBlockedOrSeriousError(err)
}

func (bp *BrowserPool) ShouldSyncRecreateAfterFailure(instance *BrowserInstance, err error) bool {
	if err == nil {
		return false
	}
	if bp.riskPolicy != nil {
		return bp.riskPolicy.ShouldSyncRecreateAfterFailure(instance, err)
	}
	return bp.errorDetector != nil && bp.errorDetector.IsBlockedOrSeriousError(err)
}

// RecreateInstanceSync 同步重新创建浏览器实例（用于任务内重试）
func (bp *BrowserPool) RecreateInstanceSync(oldInstance *BrowserInstance) *BrowserInstance {
	bp.markProxyFailure(oldInstance)
	return bp.instanceManager.RecreateInstanceSync(oldInstance)
}

// RecreateInstanceAsync 异步重新创建浏览器实例（用于失败恢复，避免池永久缩容）
func (bp *BrowserPool) RecreateInstanceAsync(oldInstance *BrowserInstance) {
	bp.instanceManager.RecreateInstanceAsync(oldInstance)
}

// Shutdown 关闭浏览器池
func (bp *BrowserPool) Shutdown() {
	bp.shutdownOnce.Do(func() {
		logger.GetGlobalLogger("crawler/amazon").Info("关闭浏览器池...")

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

		logger.GetGlobalLogger("crawler/amazon").Info("浏览器池已关闭")
	})
}

// IsBlockedOrSeriousError 检测是否为风控或严重错误（公开方法）
func (bp *BrowserPool) IsBlockedOrSeriousError(err error) bool {
	return bp.errorDetector.IsBlockedOrSeriousError(err)
}

// GetInstancesSnapshot 加锁取快照后立即释放锁，返回实例列表副本。
// 调用方无需持有 bp.Mu，适合在锁外安全遍历实例。
func (bp *BrowserPool) GetInstancesSnapshot() []*BrowserInstance {
	bp.Mu.Lock()
	snapshot := make([]*BrowserInstance, len(bp.instances))
	copy(snapshot, bp.instances)
	bp.Mu.Unlock()
	return snapshot
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

func (bp *BrowserPool) AcquireProxy(instanceID int) string {
	if bp == nil || bp.proxyPool == nil {
		return ""
	}
	proxyServer := bp.proxyPool.Acquire()
	if proxyServer == "" {
		return ""
	}
	logProxyAssigned(instanceID, proxyServer, bp.config.Amazon.ProxyPool.Strategy)
	return proxyServer
}

func (bp *BrowserPool) markProxyFailure(instance *BrowserInstance) {
	if bp == nil || bp.proxyPool == nil || instance == nil {
		return
	}
	if instance.CurrentProxy == "" {
		return
	}
	bp.proxyPool.MarkFailure(instance.CurrentProxy)
	logProxyCooldown(instance.CurrentProxy, bp.config.Amazon.ProxyPool.FailureCooldownSeconds)
}

func (bp *BrowserPool) ProxyStats() map[string]any {
	if bp == nil || bp.proxyPool == nil {
		return nil
	}
	return bp.proxyPool.Snapshot()
}
