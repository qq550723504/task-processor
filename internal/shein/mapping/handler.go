// Package mapping 提供 SHEIN 平台商品映射功能
package mapping

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/sirupsen/logrus"
)

// MappingRepairHandler SKU映射关系修复处理器
type MappingRepairHandler struct {
	repairService MappingRepairService
	config        *MappingRepairConfig
	requestQueue  chan *MappingRepairRequest
	resultQueue   chan *MappingRepairResult
	workers       int
	stopCh        chan struct{}
	wg            sync.WaitGroup
	logger        *logrus.Entry
}

// NewMappingRepairHandler 创建SKU映射关系修复处理器
func NewMappingRepairHandler(
	repairService MappingRepairService,
	config *MappingRepairConfig,
	workers int,
) *MappingRepairHandler {
	if config == nil {
		config = DefaultMappingRepairConfig()
	}

	if workers <= 0 {
		workers = 3 // 默认3个工作协程
	}

	return &MappingRepairHandler{
		repairService: repairService,
		config:        config,
		requestQueue:  make(chan *MappingRepairRequest, config.BatchSize*2),
		resultQueue:   make(chan *MappingRepairResult, config.BatchSize*2),
		workers:       workers,
		stopCh:        make(chan struct{}),
		logger:        logrus.WithField("component", "MappingRepairHandler"),
	}
}

// Start 启动修复处理器
func (h *MappingRepairHandler) Start(ctx context.Context) error {
	h.logger.WithField("workers", h.workers).Info("启动SKU映射关系修复处理器")

	// 启动工作协程
	for i := 0; i < h.workers; i++ {
		h.wg.Add(1)
		go h.worker(ctx, i)
	}

	// 启动结果处理协程
	h.wg.Add(1)
	go h.resultProcessor(ctx)

	return nil
}

// Stop 停止修复处理器
func (h *MappingRepairHandler) Stop() error {
	h.logger.Info("停止SKU映射关系修复处理器")

	close(h.stopCh)
	h.wg.Wait()

	close(h.requestQueue)
	close(h.resultQueue)

	h.logger.Info("SKU映射关系修复处理器已停止")
	return nil
}

// SubmitRepairRequest 提交修复请求
func (h *MappingRepairHandler) SubmitRepairRequest(request *MappingRepairRequest) error {
	select {
	case h.requestQueue <- request:
		h.logger.WithFields(logrus.Fields{
			"sku_code": request.SkuCode,
			"store_id": request.StoreID,
			"priority": request.Priority,
		}).Debug("提交修复请求")
		return nil
	case <-time.After(5 * time.Second):
		return fmt.Errorf("提交修复请求超时: sku_code=%s", request.SkuCode)
	}
}

// SubmitBatchRepairRequests 批量提交修复请求
func (h *MappingRepairHandler) SubmitBatchRepairRequests(requests []*MappingRepairRequest) error {
	h.logger.WithField("count", len(requests)).Info("批量提交修复请求")

	for _, request := range requests {
		if err := h.SubmitRepairRequest(request); err != nil {
			h.logger.WithError(err).WithField("sku_code", request.SkuCode).Warn("提交修复请求失败")
			return err
		}
	}

	return nil
}

// GetRepairStats 获取修复统计信息
func (h *MappingRepairHandler) GetRepairStats() *MappingRepairStats {
	return h.repairService.GetRepairStats()
}

// worker 工作协程
func (h *MappingRepairHandler) worker(ctx context.Context, workerID int) {
	defer h.wg.Done()

	logger := h.logger.WithField("worker_id", workerID)
	logger.Info("启动修复工作协程")

	for {
		select {
		case <-ctx.Done():
			logger.Info("收到上下文取消信号，停止工作协程")
			return
		case <-h.stopCh:
			logger.Info("收到停止信号，停止工作协程")
			return
		case request := <-h.requestQueue:
			if request == nil {
				logger.Info("请求队列已关闭，停止工作协程")
				return
			}

			h.processRepairRequest(ctx, request, logger)
		}
	}
}

// processRepairRequest 处理修复请求
func (h *MappingRepairHandler) processRepairRequest(
	ctx context.Context,
	request *MappingRepairRequest,
	logger *logrus.Entry,
) {
	logger.WithFields(logrus.Fields{
		"sku_code": request.SkuCode,
		"store_id": request.StoreID,
		"reason":   request.Reason,
	}).Debug("开始处理修复请求")

	startTime := time.Now()

	// 检查是否需要延迟处理
	if request.RetryTime != nil && time.Now().Before(*request.RetryTime) {
		delay := time.Until(*request.RetryTime)
		logger.WithFields(logrus.Fields{
			"sku_code": request.SkuCode,
			"delay":    delay,
		}).Debug("延迟处理修复请求")

		select {
		case <-ctx.Done():
			return
		case <-time.After(delay):
			// 继续处理
		}
	}

	// 执行修复
	result, err := h.repairService.RepairMapping(ctx, request)
	if err != nil {
		logger.WithError(err).WithField("sku_code", request.SkuCode).Error("修复请求处理失败")
		result = &MappingRepairResult{
			SkuCode:    request.SkuCode,
			Success:    false,
			Error:      fmt.Sprintf("处理失败: %v", err),
			RepairTime: time.Now(),
		}
	}

	duration := time.Since(startTime)
	logger.WithFields(logrus.Fields{
		"sku_code": request.SkuCode,
		"success":  result.Success,
		"duration": duration,
	}).Info("修复请求处理完成")

	// 发送结果
	select {
	case h.resultQueue <- result:
		// 成功发送
	case <-ctx.Done():
		return
	case <-time.After(5 * time.Second):
		logger.WithField("sku_code", request.SkuCode).Warn("发送修复结果超时")
	}
}

// resultProcessor 结果处理协程
func (h *MappingRepairHandler) resultProcessor(ctx context.Context) {
	defer h.wg.Done()

	logger := h.logger.WithField("component", "ResultProcessor")
	logger.Info("启动结果处理协程")

	for {
		select {
		case <-ctx.Done():
			logger.Info("收到上下文取消信号，停止结果处理协程")
			return
		case <-h.stopCh:
			logger.Info("收到停止信号，停止结果处理协程")
			return
		case result := <-h.resultQueue:
			if result == nil {
				logger.Info("结果队列已关闭，停止结果处理协程")
				return
			}

			h.processRepairResult(result, logger)
		}
	}
}

// processRepairResult 处理修复结果
func (h *MappingRepairHandler) processRepairResult(result *MappingRepairResult, logger *logrus.Entry) {
	logger.WithFields(logrus.Fields{
		"sku_code": result.SkuCode,
		"success":  result.Success,
	}).Debug("处理修复结果")

	if result.Success {
		logger.WithField("sku_code", result.SkuCode).Info("SKU映射关系修复成功")
	} else {
		logger.WithFields(logrus.Fields{
			"sku_code": result.SkuCode,
			"error":    result.Error,
		}).Warn("SKU映射关系修复失败")
	}

	// 这里可以添加更多的结果处理逻辑，比如：
	// 1. 发送通知
	// 2. 更新数据库状态
	// 3. 记录审计日志
	// 4. 触发后续流程
}

// CreateRepairRequestFromError 从错误信息创建修复请求
func CreateRepairRequestFromError(
	skuCode string,
	storeID, tenantID int64,
	err error,
	spuCode, spuName string,
) *MappingRepairRequest {
	return &MappingRepairRequest{
		TenantID: tenantID,
		StoreID:  storeID,
		SkuCode:  skuCode,
		SpuCode:  spuCode,
		SpuName:  spuName,
		Reason:   fmt.Sprintf("查询映射关系失败: %v", err),
		Priority: 2, // 中等优先级
	}
}

// CreateHighPriorityRepairRequest 创建高优先级修复请求
func CreateHighPriorityRepairRequest(
	skuCode string,
	storeID, tenantID int64,
	reason string,
) *MappingRepairRequest {
	return &MappingRepairRequest{
		TenantID: tenantID,
		StoreID:  storeID,
		SkuCode:  skuCode,
		Reason:   reason,
		Priority: 1, // 高优先级
	}
}

// CreateDelayedRepairRequest 创建延迟修复请求
func CreateDelayedRepairRequest(
	skuCode string,
	storeID, tenantID int64,
	reason string,
	delay time.Duration,
) *MappingRepairRequest {
	retryTime := time.Now().Add(delay)
	return &MappingRepairRequest{
		TenantID:  tenantID,
		StoreID:   storeID,
		SkuCode:   skuCode,
		Reason:    reason,
		Priority:  3, // 低优先级
		RetryTime: &retryTime,
	}
}
