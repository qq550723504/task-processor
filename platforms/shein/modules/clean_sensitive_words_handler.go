package modules

import (
	"fmt"

	"github.com/sirupsen/logrus"
)

// CleanSensitiveWordsHandler 清理敏感词处理器
type CleanSensitiveWordsHandler struct {
	// 敏感词服务
	sensitiveWordService *SensitiveWordService
}

// NewCleanSensitiveWordsHandler 创建新的清理敏感词处理器
func NewCleanSensitiveWordsHandler() *CleanSensitiveWordsHandler {
	return &CleanSensitiveWordsHandler{
		sensitiveWordService: NewSensitiveWordService(),
	}
}

// Name 返回处理器名称
func (h *CleanSensitiveWordsHandler) Name() string {
	return "清理敏感词"
}

// Handle 执行清理敏感词处理
func (h *CleanSensitiveWordsHandler) Handle(ctx *TaskContext) error {
	logrus.Info("开始执行敏感词清理处理...")

	// 检查是否已获取产品数据
	if ctx.ProductData == nil {
		return fmt.Errorf("产品数据未获取，请先执行获取产品数据步骤")
	}

	// 使用简化的敏感词服务处理产品数据
	if err := h.sensitiveWordService.ProcessProductData(ctx); err != nil {
		return fmt.Errorf("敏感词处理失败: %v", err)
	}

	logrus.Info("敏感词清理处理完成")
	return nil
}
