// Package processor is a temporary compatibility layer that forwards legacy
// internal/app imports to internal/processor. New code should import
// task-processor/internal/processor directly.
package processor

import (
	"context"

	"task-processor/internal/crawler/amazon"
	"task-processor/internal/infra/worker"
	rootprocessor "task-processor/internal/processor"
	"task-processor/internal/product"

	"github.com/sirupsen/logrus"
)

type BaseProcessor = rootprocessor.BaseProcessor
type BaseProcessorConfig = rootprocessor.BaseProcessorConfig
type BaseTaskHandler = rootprocessor.BaseTaskHandler
type CrawlerProcessor = rootprocessor.CrawlerProcessor
type RabbitMQPublisher = rootprocessor.RabbitMQPublisher
type VariantTaskSubmitter = rootprocessor.VariantTaskSubmitter

func NewBaseProcessor(ctx context.Context, cfg *BaseProcessorConfig) *BaseProcessor {
	return rootprocessor.NewBaseProcessor(ctx, cfg)
}

func NewBaseTaskHandler(proc worker.Processor, platform string) *BaseTaskHandler {
	return rootprocessor.NewBaseTaskHandler(proc, platform)
}

func NewCrawlerProcessor(
	logger *logrus.Logger,
	amazonProcessor *amazon.AmazonProcessor,
	productFetcher *product.ProductFetcher,
	taskSubmitter VariantTaskSubmitter,
	rabbitmqClient RabbitMQPublisher,
) *CrawlerProcessor {
	return rootprocessor.NewCrawlerProcessor(logger, amazonProcessor, productFetcher, taskSubmitter, rabbitmqClient)
}
