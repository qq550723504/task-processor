// Package dispatcher 提供任务路由功能
package dispatcher

import (
	"fmt"
	"sort"
	"strings"
	"sync"

	"task-processor/internal/model"

	"github.com/sirupsen/logrus"
)

// taskRouter 任务路由器实现
type taskRouter struct {
	rules  []RouterRule
	mutex  sync.RWMutex
	logger *logrus.Logger
}

// NewTaskRouter 创建任务路由器
func NewTaskRouter(logger *logrus.Logger) TaskRouter {
	router := &taskRouter{
		rules:  make([]RouterRule, 0),
		logger: logger,
	}

	// 添加默认路由规则
	router.addDefaultRules()

	return router
}

// addDefaultRules 添加默认路由规则
func (r *taskRouter) addDefaultRules() {
	// 默认规则：根据target_platform字段路由
	defaultRule := &DefaultPlatformRule{priority: 1}
	r.rules = append(r.rules, defaultRule)

	r.logger.Info("[Router] 已添加默认路由规则")
}

// AddRule 添加路由规则
func (r *taskRouter) AddRule(rule RouterRule) error {
	if rule == nil {
		return fmt.Errorf("路由规则不能为空")
	}

	r.mutex.Lock()
	defer r.mutex.Unlock()

	r.rules = append(r.rules, rule)

	// 按优先级排序（优先级高的在前）
	sort.Slice(r.rules, func(i, j int) bool {
		return r.rules[i].GetPriority() > r.rules[j].GetPriority()
	})

	r.logger.Infof("[Router] 添加路由规则，优先级: %d", rule.GetPriority())
	return nil
}

// RemoveRule 移除路由规则
func (r *taskRouter) RemoveRule(ruleID string) error {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	// 这里简化实现，实际可以为规则添加ID字段
	r.logger.Warnf("[Router] 移除路由规则功能待实现: %s", ruleID)
	return fmt.Errorf("移除路由规则功能待实现")
}

// Route 路由任务到目标平台
func (r *taskRouter) Route(task *model.UnifiedTask) (string, error) {
	if task == nil {
		return "", fmt.Errorf("任务不能为空")
	}

	r.mutex.RLock()
	defer r.mutex.RUnlock()

	// 按优先级顺序匹配规则
	for _, rule := range r.rules {
		if rule.Match(task) {
			platform := rule.GetTargetPlatform(task)
			if platform != "" {
				r.logger.Debugf("[Router] 任务 %s 路由到平台: %s", task.ID, platform)
				return platform, nil
			}
		}
	}

	return "", fmt.Errorf("没有找到匹配的路由规则，任务ID: %s", task.ID)
}

// GetRules 获取所有路由规则
func (r *taskRouter) GetRules() []RouterRule {
	r.mutex.RLock()
	defer r.mutex.RUnlock()

	// 返回副本
	rules := make([]RouterRule, len(r.rules))
	copy(rules, r.rules)
	return rules
}

// DefaultPlatformRule 默认平台路由规则
type DefaultPlatformRule struct {
	priority int
}

// Match 检查任务是否匹配此规则
func (r *DefaultPlatformRule) Match(task *model.UnifiedTask) bool {
	// 默认规则匹配所有任务
	return task.TargetPlatform != ""
}

// GetTargetPlatform 获取目标平台
func (r *DefaultPlatformRule) GetTargetPlatform(task *model.UnifiedTask) string {
	return strings.ToLower(task.TargetPlatform)
}

// GetPriority 获取规则优先级
func (r *DefaultPlatformRule) GetPriority() int {
	return r.priority
}

// PlatformMappingRule 平台映射路由规则
type PlatformMappingRule struct {
	mapping  map[string]string // 源平台到目标平台的映射
	priority int
}

// NewPlatformMappingRule 创建平台映射规则
func NewPlatformMappingRule(mapping map[string]string, priority int) RouterRule {
	return &PlatformMappingRule{
		mapping:  mapping,
		priority: priority,
	}
}

// Match 检查任务是否匹配此规则
func (r *PlatformMappingRule) Match(task *model.UnifiedTask) bool {
	_, exists := r.mapping[task.Platform]
	return exists
}

// GetTargetPlatform 获取目标平台
func (r *PlatformMappingRule) GetTargetPlatform(task *model.UnifiedTask) string {
	if target, exists := r.mapping[task.Platform]; exists {
		return target
	}
	return ""
}

// GetPriority 获取规则优先级
func (r *PlatformMappingRule) GetPriority() int {
	return r.priority
}

// RegionBasedRule 基于地区的路由规则
type RegionBasedRule struct {
	regionMapping map[string]string // 地区到平台的映射
	priority      int
}

// NewRegionBasedRule 创建基于地区的路由规则
func NewRegionBasedRule(regionMapping map[string]string, priority int) RouterRule {
	return &RegionBasedRule{
		regionMapping: regionMapping,
		priority:      priority,
	}
}

// Match 检查任务是否匹配此规则
func (r *RegionBasedRule) Match(task *model.UnifiedTask) bool {
	_, exists := r.regionMapping[task.Region]
	return exists
}

// GetTargetPlatform 获取目标平台
func (r *RegionBasedRule) GetTargetPlatform(task *model.UnifiedTask) string {
	if platform, exists := r.regionMapping[task.Region]; exists {
		return platform
	}
	return ""
}

// GetPriority 获取规则优先级
func (r *RegionBasedRule) GetPriority() int {
	return r.priority
}
