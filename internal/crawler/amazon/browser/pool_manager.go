// Package browser 提供增强的浏览器池管理功能
package browser

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"task-processor/internal/model"
	"time"

	"github.com/sirupsen/logrus"
)

// ProductProcessor 产品处理器接口
type ProductProcessor interface {
	ProcessWithInstance(instance *BrowserInstance, url string, zipcode string) (*model.Product, error)
}

// PoolManager 增强的浏览器池管理器
type PoolManager struct {
	pool       *BrowserPool
	logger     *logrus.Entry
	mu         sync.RWMutex
	isShutdown bool
}

// NewEnhancedPoolManager 创建增强的浏览器池管理器
func NewPoolManager(pool *BrowserPool) *PoolManager {
	return &PoolManager{
		pool:   pool,
		logger: logrus.WithField("component", "EnhancedPoolManager"),
	}
}

// ProcessWithTimeout 带超时处理产品
func (pm *PoolManager) ProcessWithTimeout(ctx context.Context, url, zipcode string, timeout time.Duration, processor ProductProcessor) (*model.Product, error) {
	if pm.isShutdown {
		return nil, fmt.Errorf("浏览器池管理器已关闭")
	}

	// 创建超时上下文
	timeoutCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	// 使用channel来处理结果
	resultChan := make(chan *ProcessResult, 1)

	go func() {
		defer func() {
			if r := recover(); r != nil {
				pm.logger.Errorf("处理产品时发生panic: %v", r)
				resultChan <- &ProcessResult{
					Product: nil,
					Error:   fmt.Errorf("处理产品时发生panic: %v", r),
				}
			}
		}()

		result := pm.processProduct(timeoutCtx, url, zipcode, processor)
		resultChan <- result
	}()

	select {
	case result := <-resultChan:
		return result.Product, result.Error
	case <-timeoutCtx.Done():
		pm.logger.Errorf("处理产品超时: URL=%s, Timeout=%v", url, timeout)
		return nil, fmt.Errorf("处理产品超时: %v", timeout)
	}
}

// processProduct 处理产品的内部方法
func (pm *PoolManager) processProduct(ctx context.Context, url, zipcode string, processor ProductProcessor) *ProcessResult {
	// 获取浏览器实例
	instance, err := pm.acquireInstanceWithTimeout(ctx, 30*time.Second)
	if err != nil {
		return &ProcessResult{
			Product: nil,
			Error:   fmt.Errorf("获取浏览器实例失败: %w", err),
		}
	}

	// 确保释放实例
	defer func() {
		pm.releaseInstanceSafely(instance, err)
	}()

	// 处理产品
	product, err := processor.ProcessWithInstance(instance, url, zipcode)
	return &ProcessResult{
		Product: product,
		Error:   err,
	}
}

// acquireInstanceWithTimeout 带超时获取浏览器实例
func (pm *PoolManager) acquireInstanceWithTimeout(ctx context.Context, timeout time.Duration) (*BrowserInstance, error) {
	pm.mu.RLock()
	if pm.isShutdown {
		pm.mu.RUnlock()
		return nil, fmt.Errorf("浏览器池管理器已关闭")
	}
	pm.mu.RUnlock()

	// 创建超时上下文
	timeoutCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	// 使用channel来处理获取结果
	resultChan := make(chan *BrowserInstance, 1)
	errorChan := make(chan error, 1)

	go func() {
		instance, err := pm.pool.Acquire()
		if err != nil {
			errorChan <- err
			return
		}
		resultChan <- instance
	}()

	select {
	case instance := <-resultChan:
		pm.logger.Infof("成功获取浏览器实例: %d", instance.ID)
		return instance, nil
	case err := <-errorChan:
		return nil, fmt.Errorf("获取浏览器实例失败: %w", err)
	case <-timeoutCtx.Done():
		pm.logger.Error("获取浏览器实例超时")
		return nil, fmt.Errorf("获取浏览器实例超时: %v", timeout)
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

	// 检查是否为404或产品不存在错误
	if err != nil && pm.isProductNotFoundError(err) {
		// 404错误不需要重建浏览器实例，直接释放回池
		instance.Mu.Lock()
		instance.InUse = false
		instance.Mu.Unlock()

		pm.logger.Infof("404错误，直接释放浏览器实例到池: %d", instance.ID)
		select {
		case pm.pool.available <- instance:
			pm.logger.Infof("成功释放浏览器实例: %d (404错误)", instance.ID)
		default:
			pm.logger.Warnf("浏览器池已满，无法释放实例: %d", instance.ID)
		}
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
