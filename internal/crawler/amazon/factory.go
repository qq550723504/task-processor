// Package amazon 提供Amazon爬虫工厂方法
package amazon

import (
	"task-processor/internal/core/config"

	"github.com/sirupsen/logrus"
)

// CreateProcessor 创建Amazon处理器
func CreateProcessor(cfg *config.Config, logger *logrus.Logger) *AmazonProcessor {
	logger.Info("🔧 创建Amazon爬虫处理器...")

	// 创建Amazon爬虫处理器
	amazonProcessor := NewAmazonProcessor(cfg)
	effectivePoolSize := effectiveBrowserPoolSize(cfg)

	if amazonProcessor != nil && amazonProcessor.initErr != nil {
		logger.Errorf("⚠️ Amazon爬虫处理器创建完成，但浏览器池不可用: %v", amazonProcessor.initErr)
	} else {
		logger.Infof("✅ Amazon爬虫处理器创建成功，浏览器池大小: %d", effectivePoolSize)
	}
	return amazonProcessor
}
