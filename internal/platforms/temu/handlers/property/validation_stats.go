// Package property 提供TEMU平台的属性验证统计功能
package property

import (
	"fmt"
	"time"

	"github.com/sirupsen/logrus"
)

// PropertyValidationStats 属性验证统计信息
type PropertyValidationStats struct {
	// 验证开始时间
	StartTime time.Time

	// 验证结束时间
	EndTime time.Time

	// 总属性数量
	TotalProperties int

	// 有效属性数量
	ValidProperties int

	// 修复的属性数量
	FixedProperties int

	// 跳过的属性数量
	SkippedProperties int

	// 按属性类型分类的统计
	TypeStats map[string]PropertyTypeStats

	// 修复详情
	FixDetails []PropertyFixDetail
}

// PropertyTypeStats 按属性类型的统计信息
type PropertyTypeStats struct {
	// 该类型的总数量
	Total int

	// 该类型的有效数量
	Valid int

	// 该类型的修复数量
	Fixed int

	// 该类型的跳过数量
	Skipped int
}

// PropertyFixDetail 属性修复详情
type PropertyFixDetail struct {
	// 属性PID
	PID int

	// 属性名称
	PropertyName string

	// 原始值
	OriginalValue string

	// 原始VID
	OriginalVID int

	// 修复后的值
	FixedValue string

	// 修复后的VID
	FixedVID int

	// 修复原因
	FixReason string

	// 修复方法
	FixMethod string
}

// PropertyValidationStatsCollector 属性验证统计收集器
type PropertyValidationStatsCollector struct {
	logger *logrus.Entry
	stats  *PropertyValidationStats
}

// NewPropertyValidationStatsCollector 创建新的统计收集器
func NewPropertyValidationStatsCollector(logger *logrus.Entry) *PropertyValidationStatsCollector {
	return &PropertyValidationStatsCollector{
		logger: logger,
		stats: &PropertyValidationStats{
			StartTime:  time.Now(),
			TypeStats:  make(map[string]PropertyTypeStats),
			FixDetails: make([]PropertyFixDetail, 0),
		},
	}
}

// StartValidation 开始验证统计
func (c *PropertyValidationStatsCollector) StartValidation(totalProperties int) {
	c.stats.StartTime = time.Now()
	c.stats.TotalProperties = totalProperties
	c.logger.Infof("📊 开始属性验证统计，总属性数: %d", totalProperties)
}

// RecordValidProperty 记录有效属性
func (c *PropertyValidationStatsCollector) RecordValidProperty(propertyType string) {
	c.stats.ValidProperties++
	c.updateTypeStats(propertyType, "valid")
}

// RecordFixedProperty 记录修复的属性
func (c *PropertyValidationStatsCollector) RecordFixedProperty(
	propertyType string,
	detail PropertyFixDetail,
) {
	c.stats.FixedProperties++
	c.stats.FixDetails = append(c.stats.FixDetails, detail)
	c.updateTypeStats(propertyType, "fixed")

	c.logger.Debugf("🔧 记录属性修复: PID=%d, %s → %s",
		detail.PID, detail.OriginalValue, detail.FixedValue)
}

// RecordSkippedProperty 记录跳过的属性
func (c *PropertyValidationStatsCollector) RecordSkippedProperty(propertyType string) {
	c.stats.SkippedProperties++
	c.updateTypeStats(propertyType, "skipped")
}

// FinishValidation 完成验证统计
func (c *PropertyValidationStatsCollector) FinishValidation() *PropertyValidationStats {
	c.stats.EndTime = time.Now()

	duration := c.stats.EndTime.Sub(c.stats.StartTime)
	c.logger.Infof("📊 属性验证统计完成，耗时: %v", duration)

	return c.stats
}

// updateTypeStats 更新类型统计
func (c *PropertyValidationStatsCollector) updateTypeStats(propertyType, action string) {
	stats, exists := c.stats.TypeStats[propertyType]
	if !exists {
		stats = PropertyTypeStats{}
	}

	stats.Total++
	switch action {
	case "valid":
		stats.Valid++
	case "fixed":
		stats.Fixed++
	case "skipped":
		stats.Skipped++
	}

	c.stats.TypeStats[propertyType] = stats
}

// GenerateReport 生成验证报告
func (c *PropertyValidationStatsCollector) GenerateReport() string {
	if c.stats == nil {
		return "无统计数据"
	}

	duration := c.stats.EndTime.Sub(c.stats.StartTime)

	report := fmt.Sprintf(`
📊 属性验证统计报告
==================
⏱️  验证耗时: %v
📋 总属性数: %d
✅ 有效属性: %d (%.1f%%)
🔧 修复属性: %d (%.1f%%)
⏭️  跳过属性: %d (%.1f%%)

📈 按类型统计:
`,
		duration,
		c.stats.TotalProperties,
		c.stats.ValidProperties,
		float64(c.stats.ValidProperties)/float64(c.stats.TotalProperties)*100,
		c.stats.FixedProperties,
		float64(c.stats.FixedProperties)/float64(c.stats.TotalProperties)*100,
		c.stats.SkippedProperties,
		float64(c.stats.SkippedProperties)/float64(c.stats.TotalProperties)*100,
	)

	for propertyType, stats := range c.stats.TypeStats {
		report += fmt.Sprintf("  %s: 总计=%d, 有效=%d, 修复=%d, 跳过=%d\n",
			propertyType, stats.Total, stats.Valid, stats.Fixed, stats.Skipped)
	}

	if len(c.stats.FixDetails) > 0 {
		report += "\n🔧 修复详情:\n"
		for i, detail := range c.stats.FixDetails {
			if i >= 10 { // 只显示前10个修复详情
				report += fmt.Sprintf("  ... 还有 %d 个修复记录\n", len(c.stats.FixDetails)-10)
				break
			}
			report += fmt.Sprintf("  PID=%d: '%s'(VID=%d) → '%s'(VID=%d) [%s]\n",
				detail.PID, detail.OriginalValue, detail.OriginalVID,
				detail.FixedValue, detail.FixedVID, detail.FixReason)
		}
	}

	return report
}

// GetStats 获取统计信息
func (c *PropertyValidationStatsCollector) GetStats() *PropertyValidationStats {
	return c.stats
}
