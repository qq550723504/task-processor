package product

import (
	"fmt"

	"task-processor/internal/infra/clients/management/api"
	"task-processor/internal/pipeline"
	"task-processor/internal/state"
	temucontext "task-processor/internal/temu/context"

	"task-processor/internal/core/logger"

	"github.com/sirupsen/logrus"
)

// SavePublishResultHandler 保存发品成功后返回信息处理器（参考SHEIN实现）
type SavePublishResultHandler struct {
	mappingClient api.ProductImportMappingAPI
	memoryManager *state.MemoryManager
	logger        *logrus.Entry
}

// NewSavePublishResultHandler 创建新的保存发品成功后返回信息处理器
func NewSavePublishResultHandler(mappingClient api.ProductImportMappingAPI, memoryManager *state.MemoryManager) *SavePublishResultHandler {
	return &SavePublishResultHandler{
		mappingClient: mappingClient,
		memoryManager: memoryManager,
		logger:        logger.GetGlobalLogger("SavePublishResultHandler"),
	}
}

// Name 返回处理器名称
func (h *SavePublishResultHandler) Name() string {
	return "保存发品成功后返回的信息"
}

// Handle 执行保存发品成功后返回信息处理（兼容pipeline.Handler接口）
func (h *SavePublishResultHandler) Handle(ctx pipeline.TaskContext) error {
	// 类型断言为强类型上下文
	temuCtx, ok := ctx.(*temucontext.TemuTaskContext)
	if !ok {
		return fmt.Errorf("上下文类型错误，期望TemuTaskContext")
	}
	return h.HandleTemu(temuCtx)
}

// HandleTemu 执行保存发品成功后返回信息处理（强类型上下文）
func (h *SavePublishResultHandler) HandleTemu(temuCtx *temucontext.TemuTaskContext) error {
	h.logger.Info("开始保存发品成功后的信息")

	input, err := buildSavePublishResultInput(temuCtx)
	if err != nil {
		h.logger.Warn("TEMU提交响应数据为空，跳过保存")
		return nil
	}

	if input.Product == nil {
		h.logger.Warn("产品数据不存在，跳过发布结果保存")
		return nil
	}

	// 记录响应数据到日志
	if err := h.logSubmitResponseWithInput(input); err != nil {
		h.logger.Warnf("记录响应数据失败: %v", err)
		// 不阻断流程，继续执行
	}

	// 创建产品导入映射关系
	if err := h.createProductImportMappingWithInput(input); err != nil {
		h.logger.Warnf("创建产品导入映射关系失败: %v", err)
		// 不阻断流程，继续执行
	}

	// 记录每日上架成功数量并检查限额
	h.recordDailyListingCountWithInput(input)

	h.logger.Info("发品成功后返回信息保存完成")
	return nil
}
