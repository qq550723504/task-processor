// Package property 提供属性去重功能
package property

import (
	"fmt"

	models "task-processor/internal/temu/api/product"

	"github.com/sirupsen/logrus"
)

// PropertyDeduplicator 属性去重器
type PropertyDeduplicator struct {
	logger *logrus.Entry
}

// NewPropertyDeduplicator 创建属性去重器
func NewPropertyDeduplicator(logger *logrus.Entry) *PropertyDeduplicator {
	return &PropertyDeduplicator{
		logger: logger,
	}
}

// DeduplicateByPidOnly 按PID去重属性
func (d *PropertyDeduplicator) DeduplicateByPidOnly(properties []models.PropertyItem) []models.PropertyItem {
	if len(properties) <= 1 {
		return properties
	}

	seen := make(map[int]bool)
	var result []models.PropertyItem

	for _, prop := range properties {
		key := prop.Pid
		if !seen[key] {
			seen[key] = true
			result = append(result, prop)
		} else {
			d.logger.Debugf("🔄 去重属性: PID=%d", prop.Pid)
		}
	}

	removedCount := len(properties) - len(result)
	if removedCount > 0 {
		d.logger.Infof("🔄 去重完成，移除重复属性: %d", removedCount)
	}

	return result
}

// generatePropertyKey 生成属性键
func generatePropertyKey(pid, refPid int) string {
	return fmt.Sprintf("%d_%d", pid, refPid)
}
