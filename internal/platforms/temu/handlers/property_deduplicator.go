// Package handlers 提供属性去重功能
package handlers

import (
	"fmt"
	"task-processor/internal/platforms/temu/api/models"

	"github.com/sirupsen/logrus"
)

// PropertyDeduplicator 属性去重器
type PropertyDeduplicator struct {
	logger *logrus.Entry
}

// NewPropertyDeduplicator 创建新的属性去重器
func NewPropertyDeduplicator(logger *logrus.Entry) *PropertyDeduplicator {
	return &PropertyDeduplicator{
		logger: logger,
	}
}

// DeduplicateProperties 去除重复的属性
// 去重规则：相同的 pid + ref_pid + template_pid 组合视为重复
// 保留最后一个（最新的）属性值
func (d *PropertyDeduplicator) DeduplicateProperties(properties []models.PropertyItem) []models.PropertyItem {
	if len(properties) <= 1 {
		return properties
	}

	d.logger.Infof("🔄 开始属性去重，原始属性数量: %d", len(properties))

	// 使用map记录每个属性的唯一标识
	// key: "pid_refpid_templatepid"
	propertyMap := make(map[string]models.PropertyItem)
	duplicateCount := 0

	for _, prop := range properties {
		// 构建唯一标识
		key := d.buildPropertyKey(prop)

		// 检查是否已存在
		if existingProp, exists := propertyMap[key]; exists {
			d.logger.Warnf("⚠️ 发现重复属性: PID=%d, RefPID=%d, TemplatePID=%d",
				prop.Pid, prop.RefPid, prop.TemplatePid)
			d.logger.Debugf("   原值: Value='%s', VID=%d", existingProp.Value, existingProp.Vid)
			d.logger.Debugf("   新值: Value='%s', VID=%d", prop.Value, prop.Vid)
			duplicateCount++
		}

		// 保留最新的属性值（覆盖旧值）
		propertyMap[key] = prop
	}

	// 转换回切片
	result := make([]models.PropertyItem, 0, len(propertyMap))
	for _, prop := range propertyMap {
		result = append(result, prop)
	}

	if duplicateCount > 0 {
		d.logger.Warnf("🔄 属性去重完成，移除了 %d 个重复项，最终属性数量: %d",
			duplicateCount, len(result))
	} else {
		d.logger.Info("✅ 未发现重复属性")
	}

	return result
}

// buildPropertyKey 构建属性的唯一标识
func (d *PropertyDeduplicator) buildPropertyKey(prop models.PropertyItem) string {
	// 使用 pid + ref_pid + template_pid 作为唯一标识
	// 这三个字段的组合应该能唯一标识一个属性
	return fmt.Sprintf("%d_%d_%d", prop.Pid, prop.RefPid, prop.TemplatePid)
}

// DeduplicateByPidOnly 仅根据PID去重（更宽松的去重策略）
// 当同一个PID有多个值时，保留最后一个
func (d *PropertyDeduplicator) DeduplicateByPidOnly(properties []models.PropertyItem) []models.PropertyItem {
	if len(properties) <= 1 {
		return properties
	}

	d.logger.Infof("🔄 开始按PID去重，原始属性数量: %d", len(properties))

	// 使用map记录每个PID的最新属性
	propertyMap := make(map[int]models.PropertyItem)
	duplicateCount := 0

	for _, prop := range properties {
		// 检查是否已存在相同PID
		if existingProp, exists := propertyMap[prop.Pid]; exists {
			d.logger.Warnf("⚠️ 发现相同PID的重复属性: PID=%d", prop.Pid)
			d.logger.Debugf("   原值: RefPID=%d, Value='%s', VID=%d",
				existingProp.RefPid, existingProp.Value, existingProp.Vid)
			d.logger.Debugf("   新值: RefPID=%d, Value='%s', VID=%d",
				prop.RefPid, prop.Value, prop.Vid)
			duplicateCount++
		}

		// 保留最新的属性值
		propertyMap[prop.Pid] = prop
	}

	// 转换回切片
	result := make([]models.PropertyItem, 0, len(propertyMap))
	for _, prop := range propertyMap {
		result = append(result, prop)
	}

	if duplicateCount > 0 {
		d.logger.Warnf("🔄 PID去重完成，移除了 %d 个重复项，最终属性数量: %d",
			duplicateCount, len(result))
	} else {
		d.logger.Info("✅ 未发现相同PID的重复属性")
	}

	return result
}
