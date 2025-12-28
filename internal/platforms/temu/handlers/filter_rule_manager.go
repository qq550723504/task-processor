// Package handlers 提供TEMU平台的筛选规则管理功能
package handlers

import (
	"fmt"
	"task-processor/internal/common/management/api"
	"task-processor/internal/pipeline"

	"github.com/sirupsen/logrus"
)

// FilterRuleManager 筛选规则管理器
type FilterRuleManager struct {
	logger           *logrus.Entry
	filterRuleClient api.FilterRuleAPI
}

// NewFilterRuleManager 创建新的筛选规则管理器
func NewFilterRuleManager(filterRuleClient api.FilterRuleAPI, logger *logrus.Entry) *FilterRuleManager {
	return &FilterRuleManager{
		logger:           logger.WithField("component", "FilterRuleManager"),
		filterRuleClient: filterRuleClient,
	}
}

// GetFilterRules 获取筛选规则
func (m *FilterRuleManager) GetFilterRules(ctx pipeline.TaskContext) (*[]api.FilterRuleRespDTO, error) {
	task := ctx.GetTask()
	if task == nil {
		return nil, fmt.Errorf("任务信息为空")
	}

	req := &api.FilterRuleReqDTO{
		TenantID: task.TenantID,
		StoreID:  task.StoreID,
	}

	// 如果有分类信息，添加到请求中
	if task.CategoryID > 0 {
		req.CategoryID = task.CategoryID
	}

	m.logger.WithFields(logrus.Fields{
		"tenant_id":   req.TenantID,
		"store_id":    req.StoreID,
		"category_id": req.CategoryID,
	}).Debug("正在获取筛选规则")

	rules, err := m.filterRuleClient.GetFilterRule(req)
	if err != nil {
		// 包装错误信息，提供更多上下文
		return nil, fmt.Errorf("获取过滤规则失败: %w", err)
	}

	return rules, nil
}
