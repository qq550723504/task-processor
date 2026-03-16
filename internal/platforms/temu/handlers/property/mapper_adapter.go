// Package property 提供属性映射器，直接使用新的管道架构
package property

import (
	"fmt"

	models "task-processor/internal/platforms/temu/api/product"
	temucontext "task-processor/internal/platforms/temu/context"

	"github.com/sirupsen/logrus"
)

// PropertyMapper 属性映射器，使用新的管道架构
type PropertyMapper struct {
	orchestrator *PropertyMappingOrchestrator
	logger       *logrus.Entry
}

// NewPropertyMapper 创建属性映射器
func NewPropertyMapper(logger *logrus.Entry) *PropertyMapper {
	return &PropertyMapper{
		orchestrator: NewPropertyMappingOrchestrator(logger),
		logger:       logger,
	}
}

// BuildGoodsProperties 构建商品属性
func (m *PropertyMapper) BuildGoodsProperties(temuCtx *temucontext.TemuTaskContext, ext *models.ExtensionInfo) error {
	m.logger.Info("🚀 开始属性映射处理")

	// 使用新的编排器处理
	if err := m.orchestrator.ProcessProperties(temuCtx, ext); err != nil {
		return fmt.Errorf("属性处理失败: %w", err)
	}

	m.logger.Info("✅ 属性映射处理完成")
	return nil
}

// GetOrchestrator 获取编排器（用于配置和监控）
func (m *PropertyMapper) GetOrchestrator() *PropertyMappingOrchestrator {
	return m.orchestrator
}

// SetConfig 配置处理参数
func (m *PropertyMapper) SetConfig(config *ProcessingConfig) {
	m.orchestrator.SetConfig(config)
	m.logger.Debug("⚙️ 更新处理配置")
}

// AddCustomStage 添加自定义处理阶段
func (m *PropertyMapper) AddCustomStage(stage PropertyStage) {
	m.orchestrator.AddCustomStage(stage)
	m.logger.Debugf("➕ 添加自定义阶段: %s", stage.GetName())
}

// ValidateConfiguration 验证配置
func (m *PropertyMapper) ValidateConfiguration() error {
	return m.orchestrator.ValidateConfiguration()
}

// GetStats 获取统计信息
func (m *PropertyMapper) GetStats() map[string]any {
	return m.orchestrator.GetProcessingStats()
}

// Name 返回处理器名称
func (m *PropertyMapper) Name() string {
	return "属性映射器"
}

// HandleTemu 处理任务
func (m *PropertyMapper) HandleTemu(temuCtx *temucontext.TemuTaskContext) error {
	// 检查TEMU产品信息
	if temuCtx.TemuProduct == nil {
		return fmt.Errorf("TEMU产品信息为空")
	}

	// 构建商品属性
	return m.BuildGoodsProperties(temuCtx, &temuCtx.TemuProduct.GoodsExtensionInfo)
}
