// Package browser 提供增强的浏览器池管理功能
package browser

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"task-processor/internal/model"
	"time"

	"task-processor/internal/core/logger"

	"github.com/sirupsen/logrus"
)

// ProductProcessor 产品处理器接口
type ProductProcessor interface {
	ProcessWithInstance(ctx context.Context, instance *BrowserInstance, url string, zipcode string) (*model.Product, error)
}

// poolBehavior 定义 PoolManager 依赖的浏览器池行为，便于测试注入 mock
type poolBehavior interface {
	IsBlockedOrSeriousError(err error) bool
	RecreateInstanceSync(old *BrowserInstance) *BrowserInstance
	RecreateInstanceAsync(old *BrowserInstance)
	Release(instance *BrowserInstance)
	ReleaseWithError(instance *BrowserInstance, err error)
	GetAvailableChannel() chan *BrowserInstance
}

// PoolManager 增强的浏览器池管理器
type PoolManager struct {
	pool       poolBehavior
	logger     *logrus.Entry
	mu         sync.RWMutex
	isShutdown bool
}

// NewEnhancedPoolManager 创建增强的浏览器池管理器
func NewPoolManager(pool *BrowserPool) *PoolManager {
	return &PoolManager{
		pool:   pool,
		logger: logger.GetGlobalLogger("EnhancedPoolManager"),
	}
}

// ProcessWithTimeout 带超时处理产品
func (pm *PoolManager) ProcessWithTimeout(ctx context.Context, url, zipcode string, timeout time.Duration, processor ProductProcessor) (*model.Product, error) {
	if pm.isShutdown {
		return nil, fmt.Errorf("浏览器池管理器已关闭")
	}

	startAt := time.Now()
	timeoutCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	// instanceChan 用于将获取到的实例传递给超时处理逻辑
	// buffered(1) 确保 goroutine 写入后不阻塞
	resultChan := make(chan *ProcessResult, 1)
	instanceChan := make(chan *BrowserInstance, 1)

	go func() {
		defer func() {
			if r := recover(); r != nil {
				pm.logger.Errorf("处理产品时发生panic: %v", r)
				resultChan <- &ProcessResult{Error: fmt.Errorf("处理产品时发生panic: %v", r)}
			}
		}()
		// 传入 timeoutCtx：ctx 取消时浏览器操作会尽快退出，
		// processProduct 内部负责释放实例，goroutine 结束后实例回到池中
		resultChan <- pm.processProduct(timeoutCtx, url, zipcode, processor, instanceChan)
	}()

	select {
	case result := <-resultChan:
		elapsed := time.Since(startAt)
		if result.Error != nil {
			pm.logger.Warnf("处理完成(有错误): URL=%s, 耗时=%.1fs, Error=%v", url, elapsed.Seconds(), result.Error)
		} else {
			pm.logger.Infof("处理成功: URL=%s, 耗时=%.1fs", url, elapsed.Seconds())
		}
		return result.Product, result.Error
	case <-timeoutCtx.Done():
		elapsed := time.Since(startAt)
		pm.logger.Errorf("处理产品超时: URL=%s, Timeout=%v, 实际耗时=%.1fs", url, timeout, elapsed.Seconds())
		select {
		case instance := <-instanceChan:
			if instance != nil && instance.Manager != nil {
				pm.logger.Warnf("超时后强制关闭浏览器实例 %d 并异步重建", instance.ID)
				instance.Mu.Lock()
				instance.Closed = true
				instance.Mu.Unlock()
				pm.pool.RecreateInstanceAsync(instance)
			}
		default:
			pm.logger.Errorf("超时时 goroutine 仍在等待浏览器实例，池可能已耗尽！URL=%s", url)
		}
		return nil, fmt.Errorf("处理产品超时: %v", timeout)
	}
}

// processProduct 处理产品的内部方法，遇到浏览器严重错误时同步重建实例并重试一次。
// instanceChan 用于将获取到的实例传递给超时处理逻辑，以便超时时主动关闭实例。
func (pm *PoolManager) processProduct(ctx context.Context, url, zipcode string, processor ProductProcessor, instanceChan chan<- *BrowserInstance) *ProcessResult {
	// 获取浏览器实例：等待时间跟随父 ctx，不单独设30s硬超时。
	// 若父 ctx 还有3分钟，就最多等3分钟，避免池满时30s就报超时失败。
	acquireStart := time.Now()
	instance, err := pm.acquireInstanceWithTimeout(ctx, 0)
	acquireElapsed := time.Since(acquireStart)
	if err != nil {
		pm.logger.Errorf("获取浏览器实例失败: 等待耗时=%.1fs, Error=%v", acquireElapsed.Seconds(), err)
		return &ProcessResult{
			Product: nil,
			Error:   fmt.Errorf("获取浏览器实例失败: %w", err),
		}
	}
	pm.logger.Infof("获取浏览器实例成功: ID=%d, 等待耗时=%.1fs", instance.ID, acquireElapsed.Seconds())

	// 将实例发送给超时处理逻辑（non-blocking，超时方可能已经不再监听）
	select {
	case instanceChan <- instance:
	default:
	}

	// 处理产品
	processStart := time.Now()
	product, processErr := processor.ProcessWithInstance(ctx, instance, url, zipcode)
	processElapsed := time.Since(processStart)
	if processErr != nil {
		pm.logger.Warnf("处理产品完成(有错误): ID=%d, 处理耗时=%.1fs, Error=%v", instance.ID, processElapsed.Seconds(), processErr)
	} else {
		pm.logger.Infof("处理产品成功: ID=%d, 处理耗时=%.1fs", instance.ID, processElapsed.Seconds())
	}

	// 检查实例是否已被超时路径关闭
	instance.Mu.Lock()
	isClosed := instance.Closed
	instance.Mu.Unlock()
	if isClosed {
		pm.logger.Warnf("实例 %d 已由超时路径关闭，goroutine 直接退出", instance.ID)
		return &ProcessResult{Product: product, Error: processErr}
	}

	// 检测到 WebSocket 断连等严重错误时，同步重建实例并重试一次
	if processErr != nil && pm.pool.IsBlockedOrSeriousError(processErr) {
		pm.logger.Warnf("浏览器实例 %d 出现严重错误，同步重建后重试: %v", instance.ID, processErr)

		newInstance := pm.pool.RecreateInstanceSync(instance)
		if newInstance == nil {
			pm.logger.Errorf("同步重建浏览器实例失败，异步补充实例后返回错误: %s", url)
			// 重建失败：旧实例已被关闭，必须异步补充一个新实例，否则池永久缩容
			pm.pool.RecreateInstanceAsync(instance)
			return &ProcessResult{Error: fmt.Errorf("重建浏览器实例失败: %w", processErr)}
		}

		// 用新实例重试，重试后无论结果如何都直接释放（不再触发二次重建）
		product, processErr = processor.ProcessWithInstance(ctx, newInstance, url, zipcode)
		pm.pool.Release(newInstance)
		return &ProcessResult{Product: product, Error: processErr}
	}

	// 正常路径：释放实例
	pm.releaseInstanceSafely(instance, processErr)
	return &ProcessResult{Product: product, Error: processErr}
}

// acquireInstanceWithTimeout 带超时获取浏览器实例。
// acquireTimeout=0 时直接使用父 ctx 的 deadline，适合"等到任务超时为止"的场景。
// acquireTimeout>0 时使用独立超时，不受父 ctx 影响（用于需要精确控制等待时长的场景）。
func (pm *PoolManager) acquireInstanceWithTimeout(ctx context.Context, acquireTimeout time.Duration) (*BrowserInstance, error) {
	pm.mu.RLock()
	if pm.isShutdown {
		pm.mu.RUnlock()
		return nil, fmt.Errorf("浏览器池管理器已关闭")
	}
	pm.mu.RUnlock()

	// 先检查父 ctx 是否已取消，避免无意义的获取
	select {
	case <-ctx.Done():
		return nil, fmt.Errorf("context 已取消，跳过获取浏览器实例: %w", ctx.Err())
	default:
	}

	// acquireTimeout=0：直接用父 ctx，等到任务整体超时为止
	// acquireTimeout>0：使用独立超时，不受父 ctx 影响
	var acquireCtx context.Context
	var cancel context.CancelFunc
	if acquireTimeout > 0 {
		acquireCtx, cancel = context.WithTimeout(context.Background(), acquireTimeout)
		defer cancel()
	} else {
		acquireCtx = ctx
	}

	select {
	case instance, ok := <-pm.pool.GetAvailableChannel():
		if !ok || instance == nil {
			return nil, fmt.Errorf("浏览器池已关闭")
		}
		// 拿到实例后再检查父 ctx，若已取消则立即归还
		if ctx.Err() != nil {
			pm.pool.Release(instance)
			return nil, fmt.Errorf("context 已取消，跳过获取浏览器实例: %w", ctx.Err())
		}
		instance.Mu.Lock()
		instance.InUse = true
		instance.Mu.Unlock()
		pm.logger.Infof("成功获取浏览器实例: %d", instance.ID)
		return instance, nil
	case <-acquireCtx.Done():
		if acquireTimeout > 0 {
			pm.logger.Errorf("获取浏览器实例超时 (%v)", acquireTimeout)
			return nil, fmt.Errorf("获取浏览器实例超时: %v", acquireTimeout)
		}
		// acquireTimeout=0 时 acquireCtx == ctx，统一走下面的 ctx.Done 分支
		return nil, fmt.Errorf("获取浏览器实例失败: %w", ctx.Err())
	case <-ctx.Done():
		return nil, fmt.Errorf("context 已取消，跳过获取浏览器实例: %w", ctx.Err())
	}
}

// releaseInstanceSafely 安全释放浏览器实例
func (pm *PoolManager) releaseInstanceSafely(instance *BrowserInstance, err error) {
	if instance == nil {
		pm.logger.Warn("尝试释放空的浏览器实例")
		return
	}

	defer func() {
		if r := recover(); r != nil {
			pm.logger.Errorf("释放浏览器实例时发生panic: %v", r)
		}
	}()

	pm.mu.RLock()
	if pm.isShutdown {
		pm.mu.RUnlock()
		pm.logger.Warnf("池管理器已关闭，跳过释放实例: %d", instance.ID)
		return
	}
	pm.mu.RUnlock()

	// 实例已被超时处理关闭（Closed=true），
	// RecreateInstanceAsync 已在超时路径中触发，此处直接丢弃，不重复操作。
	instance.Mu.Lock()
	isClosed := instance.Closed
	instance.Mu.Unlock()
	if isClosed {
		pm.logger.Warnf("实例 %d 已由超时路径关闭，跳过释放", instance.ID)
		return
	}

	// 检查是否为404或产品不存在错误，直接归还池，不触发重建
	if err != nil && pm.isProductNotFoundError(err) {
		pm.pool.Release(instance)
		return
	}

	// 使用带错误检测的释放方法
	pm.pool.ReleaseWithError(instance, err)
	pm.logger.Infof("成功释放浏览器实例: %d", instance.ID)
}

// isProductNotFoundError 检查是否为产品不存在错误
func (pm *PoolManager) isProductNotFoundError(err error) bool {
	if err == nil {
		return false
	}

	errorStr := err.Error()
	notFoundPatterns := []string{
		"产品页面不存在",
		"产品页面缺少必要元素",
		"页面不存在(404)",
		"页面不存在",
		"页面未准备就绪: 页面不存在",
		"不是有效的产品页面",
		"product not found", "Product not found",
		"404",
	}

	for _, pattern := range notFoundPatterns {
		if strings.Contains(errorStr, pattern) {
			return true
		}
	}

	return false
}

// Shutdown 关闭池管理器
func (pm *PoolManager) Shutdown() {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	if pm.isShutdown {
		return
	}

	pm.logger.Info("开始关闭增强浏览器池管理器")
	pm.isShutdown = true
	pm.logger.Info("增强浏览器池管理器已关闭")
}

// ProcessResult 处理结果
type ProcessResult struct {
	Product *model.Product
	Error   error
}
