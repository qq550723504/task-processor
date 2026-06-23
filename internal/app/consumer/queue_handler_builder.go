package consumer

import (
	"fmt"
	"strings"

	"task-processor/internal/infra/rabbitmq"
)

type queueHandlerBuilder struct {
	service *RabbitMQService
}

func newQueueHandlerBuilder(service *RabbitMQService) *queueHandlerBuilder {
	return &queueHandlerBuilder{service: service}
}

func (b *queueHandlerBuilder) build() map[string]rabbitmq.MessageHandler {
	handlers := b.service.processorRegistry.GetAllHandlers()
	queueHandlers := make(map[string]rabbitmq.MessageHandler)

	if !b.service.isOwnershipCoordinator() {
		for platform, handler := range handlers {
			if b.isCrawlerPlatform(platform) {
				b.registerCrawlerHandlers(queueHandlers, platform, handler)
				continue
			}
			b.registerTaskHandlers(queueHandlers, platform, handler)
		}
	}
	b.registerDeadLetterHandler(queueHandlers)

	if len(handlers) == 0 {
		b.service.logger.Warn("没有注册任何消息处理器")
	}
	return queueHandlers
}

func (b *queueHandlerBuilder) isCrawlerPlatform(platform string) bool {
	return platform == "amazon.crawler" || platform == "1688.crawler"
}

func (b *queueHandlerBuilder) registerCrawlerHandlers(queueHandlers map[string]rabbitmq.MessageHandler, platform string, handler rabbitmq.MessageHandler) {
	if !b.service.config.Node.HandlesCrawlerWork() {
		b.service.logger.Infof("跳过爬虫处理器注册: platform=%s, role=%s", platform, b.service.config.Node.NormalizedRole())
		return
	}

	regions := b.service.config.Node.Regions
	if len(regions) > 0 {
		for _, region := range regions {
			basePlatform := strings.TrimSuffix(platform, ".crawler")
			queueName := fmt.Sprintf("%s.crawler.%s", basePlatform, strings.ToLower(region))
			queueHandlers[queueName] = handler
		}
		b.service.logger.Infof("注册爬虫处理器（按 region）: 平台=%s, regions=%v", platform, regions)
		return
	}

	queueHandlers[platform] = handler
	b.service.logger.Infof("注册爬虫处理器（全局队列）: 平台=%s", platform)
}

func (b *queueHandlerBuilder) registerTaskHandlers(queueHandlers map[string]rabbitmq.MessageHandler, platform string, handler rabbitmq.MessageHandler) {
	if !b.service.config.Node.HandlesTaskWork() {
		b.service.logger.Infof("跳过任务处理器注册: platform=%s, role=%s", platform, b.service.config.Node.NormalizedRole())
		return
	}

	if b.service.usesDedicatedStoreQueues() {
		if len(b.service.ownedStores) == 0 {
			b.service.logger.Infof("跳过共享队列注册: 平台=%s, useStoreQueues=true, ownedStores=0", platform)
			return
		}
		for _, storeID := range b.service.ownedStores {
			queueName := fmt.Sprintf("%s.tasks.store.%d", platform, storeID)
			queueHandlers[queueName] = handler
		}
		b.service.logger.Infof("注册处理器: 平台=%s, 店铺=%v", platform, b.service.ownedStores)
		return
	}

	b.registerSharedPlatformHandlers(queueHandlers, platform, handler)
}

func (b *queueHandlerBuilder) registerSharedPlatformHandlers(queueHandlers map[string]rabbitmq.MessageHandler, platform string, handler rabbitmq.MessageHandler) {
	queueName := fmt.Sprintf("%s.tasks", platform)
	queueHandlers[queueName] = handler

	if strings.EqualFold(platform, "shein") {
		buckets := b.service.sharedBucketsForPlatform(platform)
		for _, bucket := range buckets {
			bucketQueueName := fmt.Sprintf("%s.tasks.bucket.%d", platform, bucket)
			queueHandlers[bucketQueueName] = handler
		}
		b.service.logger.Infof("注册处理器（平台共享分桶队列）: 平台=%s, buckets=%v", platform, buckets)
		return
	}

	b.service.logger.Infof("注册处理器（平台共享队列）: 平台=%s", platform)
}

func (b *queueHandlerBuilder) registerDeadLetterHandler(queueHandlers map[string]rabbitmq.MessageHandler) {
	if !b.service.config.DeadLetter.Enabled {
		return
	}
	runtime := b.service.resolveTaskStatusRuntime()
	if runtime == nil {
		b.service.logger.Warn("跳过死信处理器注册: 未找到任务状态 runtime")
		return
	}
	queueName := strings.TrimSpace(b.service.config.DeadLetter.QueueName)
	if queueName == "" {
		queueName = defaultDLQQueueName
	}
	queueHandlers[queueName] = NewDeadLetterHandler(DeadLetterHandlerConfig{
		Runtime: runtime,
		Logger:  b.service.logger,
	})
	b.service.logger.Infof("注册死信处理器: queue=%s", queueName)
}
