// Package service 提供Amazon处理器管理功能
package service

import (
	"sync"

	"task-processor/internal/common/amazon"
	"task-processor/internal/core/config"

	"github.com/sirupsen/logrus"
)

var (
	// 全局Amazon处理器单例
	sharedAmazonProcessor *amazon.AmazonProcessor
	amazonProcessorMutex  sync.Mutex
)

// GetSharedAmazonProcessor 获取共享的Amazon处理器
func GetSharedAmazonProcessor(cfg *config.Config, logger *logrus.Logger) *amazon.AmazonProcessor {
	amazonProcessorMutex.Lock()
	defer amazonProcessorMutex.Unlock()

	if sharedAmazonProcessor == nil {
		logger.Info("🔄 创建共享Amazon处理器...")
		sharedAmazonProcessor = amazon.NewAmazonProcessor(&cfg.Amazon)
		logger.Info("✅ 共享Amazon处理器创建完成")
	} else {
		logger.Info("♻️ 复用现有Amazon处理器")
	}

	return sharedAmazonProcessor
}

// CloseSharedAmazonProcessor 关闭共享的Amazon处理器
func CloseSharedAmazonProcessor(logger *logrus.Logger) {
	amazonProcessorMutex.Lock()
	defer amazonProcessorMutex.Unlock()

	if sharedAmazonProcessor != nil {
		logger.Info("🛑 关闭共享Amazon处理器...")
		sharedAmazonProcessor.Shutdown()
		sharedAmazonProcessor = nil
		logger.Info("✅ 共享Amazon处理器已关闭")
	}
}
