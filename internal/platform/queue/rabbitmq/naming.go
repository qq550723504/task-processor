// Package rabbitmq 提供 RabbitMQ 队列命名规则
package rabbitmq

import (
	"fmt"
	"strings"
)

// NamingService 队列命名服务
type NamingService struct{}

// NewNamingService 创建队列命名服务
func NewNamingService() *NamingService {
	return &NamingService{}
}

// BuildCrawlerQueueName 构建爬虫队列名称，格式: {platform}.crawler
func (s *NamingService) BuildCrawlerQueueName(platform string, priority int) string {
	basePlatform := s.extractBasePlatform(platform)
	return fmt.Sprintf("%s.crawler", strings.ToLower(basePlatform))
}

// BuildCrawlerQueueNameByRegion 构建带 region 的爬虫队列名称，格式: {platform}.crawler.{region}
func (s *NamingService) BuildCrawlerQueueNameByRegion(platform, region string, priority int) string {
	basePlatform := s.extractBasePlatform(platform)
	return fmt.Sprintf("%s.crawler.%s", strings.ToLower(basePlatform), strings.ToLower(region))
}

// BuildTaskQueueName 构建任务队列名称，格式: {platform}.tasks
func (s *NamingService) BuildTaskQueueName(platform string, priority int) string {
	basePlatform := s.extractBasePlatform(platform)
	return fmt.Sprintf("%s.tasks", strings.ToLower(basePlatform))
}

// BuildTaskQueueNameForStore 构建店铺专属任务队列名称，格式: {platform}.tasks.store.{storeID}。
// 当 storeID 非法时回退到平台共享队列，保持向后兼容。
func (s *NamingService) BuildTaskQueueNameForStore(platform string, priority int, storeID int64) string {
	if storeID <= 0 {
		return s.BuildTaskQueueName(platform, priority)
	}
	basePlatform := s.extractBasePlatform(platform)
	return fmt.Sprintf("%s.tasks.store.%d", strings.ToLower(basePlatform), storeID)
}

// IsCrawlerPlatform 判断是否是爬虫平台
func (s *NamingService) IsCrawlerPlatform(platform string) bool {
	return strings.Contains(platform, ".crawler")
}

func (s *NamingService) extractBasePlatform(platform string) string {
	return strings.TrimSuffix(platform, ".crawler")
}
