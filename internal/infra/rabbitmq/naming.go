// Package rabbitmq 提供 RabbitMQ 队列命名规则
package rabbitmq

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

// BuildCrawlerQueueName 构建爬虫队列名称，格式: {platform}.crawler.{priority_level}
func (s *NamingService) BuildCrawlerQueueName(platform string, priority int) string {
	basePlatform := s.extractBasePlatform(platform)
	priorityLevel := s.getPriorityLevel(priority)
	return fmt.Sprintf("%s.crawler.%s", strings.ToLower(basePlatform), priorityLevel)
}

// BuildTaskQueueName 构建任务队列名称，格式: {platform}.tasks.{priority_level}
func (s *NamingService) BuildTaskQueueName(platform string, priority int) string {
	basePlatform := s.extractBasePlatform(platform)
	priorityLevel := s.getPriorityLevel(priority)
	return fmt.Sprintf("%s.tasks.%s", strings.ToLower(basePlatform), priorityLevel)
}

// GetPriorityLevel 获取优先级级别
func (s *NamingService) GetPriorityLevel(priority int) PriorityLevel {
	return s.getPriorityLevel(priority)
}

// IsCrawlerPlatform 判断是否是爬虫平台
func (s *NamingService) IsCrawlerPlatform(platform string) bool {
	return strings.Contains(platform, ".crawler")
}

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

func (s *NamingService) extractBasePlatform(platform string) string {
	return strings.TrimSuffix(platform, ".crawler")
}
