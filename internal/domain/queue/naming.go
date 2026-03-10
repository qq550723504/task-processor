// Package queue 提供队列相关的领域逻辑
package queue

import (
	"fmt"
	"strings"
)

// PriorityLevel 优先级级别
type PriorityLevel string

const (
	PriorityLevelHigh   PriorityLevel = "high"
	PriorityLevelNormal PriorityLevel = "normal"
	PriorityLevelLow    PriorityLevel = "low"
)

// 优先级范围常量
const (
	PriorityHighMin   = 1
	PriorityHighMax   = 3
	PriorityNormalMin = 4
	PriorityNormalMax = 7
	PriorityLowMin    = 8
	PriorityLowMax    = 10

	PriorityDefault       = 5
	PriorityAmazonBonus   = 2
	PriorityCategoryBonus = 1
)

// NamingService 队列命名服务
type NamingService struct{}

// NewNamingService 创建队列命名服务
func NewNamingService() *NamingService {
	return &NamingService{}
}

// BuildCrawlerQueueName 构建爬虫队列名称
// 格式: {platform}.crawler.{priority_level}
// 示例: amazon.crawler.high, amazon.crawler.normal, amazon.crawler.low
func (s *NamingService) BuildCrawlerQueueName(platform string, priority int) string {
	basePlatform := s.extractBasePlatform(platform)
	priorityLevel := s.getPriorityLevel(priority)
	return fmt.Sprintf("%s.crawler.%s", strings.ToLower(basePlatform), priorityLevel)
}

// BuildTaskQueueName 构建任务队列名称
// 格式: {platform}.tasks.{priority_level}
// 示例: amazon.tasks.high, temu.tasks.normal
func (s *NamingService) BuildTaskQueueName(platform string, priority int) string {
	basePlatform := s.extractBasePlatform(platform)
	priorityLevel := s.getPriorityLevel(priority)
	return fmt.Sprintf("%s.tasks.%s", strings.ToLower(basePlatform), priorityLevel)
}

// GetPriorityLevel 获取优先级级别（公开方法）
func (s *NamingService) GetPriorityLevel(priority int) PriorityLevel {
	return s.getPriorityLevel(priority)
}

// getPriorityLevel 根据优先级数值获取优先级级别
func (s *NamingService) getPriorityLevel(priority int) PriorityLevel {
	switch {
	case priority >= PriorityHighMin && priority <= PriorityHighMax:
		return PriorityLevelHigh
	case priority >= PriorityNormalMin && priority <= PriorityNormalMax:
		return PriorityLevelNormal
	default:
		return PriorityLevelLow
	}
}

// extractBasePlatform 提取基础平台名称（移除 .crawler 后缀）
func (s *NamingService) extractBasePlatform(platform string) string {
	return strings.TrimSuffix(platform, ".crawler")
}

// IsCrawlerPlatform 判断是否是爬虫平台
func (s *NamingService) IsCrawlerPlatform(platform string) bool {
	return strings.Contains(platform, ".crawler")
}
