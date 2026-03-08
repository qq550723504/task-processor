// Package amazon 提供Amazon爬虫工厂方法
package amazon

import (
	"task-processor/internal/core/config"

	"github.com/sirupsen/logrus"
)

// CreateProcessor 创建Amazon处理器
func CreateProcessor(cfg *config.Config, logger *logrus.Logger) *AmazonProcessor {
	logger.Info("🔧 创建Amazon爬虫处理器...")

	// 确保浏览器配置合理
	if cfg.Browser.PoolSize <= 0 {
		cfg.Browser.PoolSize = 3 // 爬虫默认使用3个浏览器实例
	}

	// 创建Amazon爬虫处理器
	amazonProcessor := NewAmazonProcessor(cfg)

	logger.Infof("✅ Amazon爬虫处理器创建成功，浏览器池大小: %d", cfg.Browser.PoolSize)
	return amazonProcessor
}
