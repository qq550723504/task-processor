// Package handlers 提供属性最终化阶段，负责去重和最终保障
package handlers

import (
	"fmt"

	"github.com/sirupsen/logrus"
)

// PropertyFinalizationStage 属性最终化阶段
type PropertyFinalizationStage struct {
	*BasePropertyStage
	deduplicator *PropertyDeduplicator
	logger       *logrus.Entry
}

// NewPropertyFinalizationStage 创建属性最终化阶段
func NewPropertyFinalizationStage(logger *logrus.Entry) *PropertyFinalizationStage {
	return &PropertyFinalizationStage{
		BasePropertyStage: NewBasePropertyStage("属性最终化阶段", 400),
		logger:            logger,
	}
}

// Process 处理属性最终化
func (s *PropertyFinalizationStage) Process(ctx *PropertyContext) error {
	s.logger.Info("🏁 开始属性最终化处理")

	originalCount := len(ctx.CurrentProperties)

	// 初始化组件
	if s.deduplicator == nil {
		s.deduplicator = NewPropertyDeduplicator(s.logger)
	}

	// 执行属性去重
	ctx.CurrentProperties = s.deduplicator.DeduplicateByPidOnly(ctx.CurrentProperties)

	// 检查必填属性完整性
	missingCount := s.checkMissingRequiredProperties(ctx)
	if missingCount > 0 {
		s.logger.Warnf("⚠️ 发现缺失的必填属性: %d 个", missingCount)
		if ctx.Config.EnableStrictMode {
			return fmt.Errorf("严格模式下不允许缺失必填属性，缺失数量: %d", missingCount)
		}
	}

	finalCount := len(ctx.CurrentProperties)

	// 更新统计
	ctx.Statistics.ProcessedCount += originalCount

	s.logger.Infof("🏁 属性最终化完成，属性数量: %d -> %d", originalCount, finalCount)
	return nil
}

// checkMissingRequiredProperties 检查缺失的必填属性
func (s *PropertyFinalizationStage) checkMissingRequiredProperties(ctx *PropertyContext) int {
	// 创建已有属性的PID集合
	existingPIDs := make(map[int]bool)
	for _, prop := range ctx.CurrentProperties {
		existingPIDs[prop.Pid] = true
	}

	// 统计缺失的必填属性
	missingCount := 0
	for _, templateProp := range ctx.TemplateProperties {
		if templateProp.Required && !existingPIDs[templateProp.PID] {
			missingCount++
			s.logger.Debugf("⚠️ 缺失必填属性: %s (PID=%d)", templateProp.Name, templateProp.PID)
		}
	}

	return missingCount
}

// IsEnabled 检查阶段是否启用
func (s *PropertyFinalizationStage) IsEnabled(ctx *PropertyContext) bool {
	return s.BasePropertyStage.IsEnabled(ctx)
}

// SetDeduplicator 设置去重器
func (s *PropertyFinalizationStage) SetDeduplicator(deduplicator *PropertyDeduplicator) {
	s.deduplicator = deduplicator
}
