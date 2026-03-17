// Package filter 提供TEMU平台的筛选规则统计功能
package filter

import (
	"task-processor/internal/pipeline"

	"github.com/sirupsen/logrus"
)

// FilterRuleStatsProvider 筛选规则统计提供器
type FilterRuleStatsProvider struct {
	logger      *logrus.Entry
	ruleManager *FilterRuleManager
}

// NewFilterRuleStatsProvider 创建新的筛选规则统计提供器
func NewFilterRuleStatsProvider(ruleManager *FilterRuleManager, logger *logrus.Entry) *FilterRuleStatsProvider {
	return &FilterRuleStatsProvider{
		logger:      logger.WithField("component", "FilterRuleStatsProvider"),
		ruleManager: ruleManager,
	}
}

// GetFilterRuleStats 获取筛选规则统计信息（用于调试和监控）
func (p *FilterRuleStatsProvider) GetFilterRuleStats(ctx pipeline.TaskContext) (map[string]any, error) {
	rules, err := p.ruleManager.GetFilterRules(ctx)
	if err != nil {
		return nil, err
	}

	stats := map[string]any{
		"total_rules":    len(*rules),
		"enabled_rules":  0,
		"disabled_rules": 0,
		"rule_details":   make([]map[string]any, 0),
	}

	for _, rule := range *rules {
		if rule.Status == 1 {
			stats["enabled_rules"] = stats["enabled_rules"].(int) + 1
		} else {
			stats["disabled_rules"] = stats["disabled_rules"].(int) + 1
		}

		ruleDetail := map[string]any{
			"id":          rule.ID,
			"name":        rule.Name,
			"description": rule.Description,
			"status":      rule.Status,
		}
		stats["rule_details"] = append(stats["rule_details"].([]map[string]any), ruleDetail)
	}

	return stats, nil
}
