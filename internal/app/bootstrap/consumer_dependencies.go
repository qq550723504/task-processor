package bootstrap

import (
	"context"

	"task-processor/internal/app/consumer"
	"task-processor/internal/core/config"
	"task-processor/internal/crawler/amazon"
	"task-processor/internal/product"
	"task-processor/internal/shein/pipeline"
	"task-processor/internal/temu"

	"github.com/sirupsen/logrus"
)

func BuildConsumerDependencies() consumer.PlatformRegistryDependencies {
	return consumer.PlatformRegistryDependencies{
		ProcessorCreators: consumer.ProcessorCreators{
			TemuProcessorCreator: func(ctx context.Context, cfg *config.Config, logger *logrus.Logger, deps temu.Dependencies) (*temu.TemuProcessor, error) {
				return temu.NewTemuProcessor(ctx, cfg, logger, deps)
			},
			SheinProcessorCreator: func(ctx context.Context, cfg *config.Config, logger *logrus.Logger, deps pipeline.Dependencies) (*pipeline.SheinProcessor, error) {
				return pipeline.NewSheinProcessor(ctx, cfg, logger, deps)
			},
		},
		SharedResourceProvider: func(cfg *config.Config, logger *logrus.Logger, needsAmazon bool) (*consumer.SharedResources, error) {
			resources, err := BuildSharedResources(cfg, logger, SharedResourceOptions{
				NeedAmazonCrawler: needsAmazon,
			})
			if err != nil {
				return nil, err
			}

			return &consumer.SharedResources{
				ManagementClient: resources.ManagementClient,
				AmazonProcessor:  resources.AmazonCrawler,
			}, nil
		},
	}
}

func BuildCrawlerDependencies() consumer.CrawlerRegistryDependencies {
	return consumer.CrawlerRegistryDependencies{
		AmazonCrawlerCreator: func(cfg *config.Config, logger *logrus.Logger) *amazon.AmazonProcessor {
			return amazon.CreateProcessor(cfg, logger)
		},
		ProductFetcherProvider: func(cfg *config.Config, logger *logrus.Logger, amazonProcessor *amazon.AmazonProcessor) (*product.ProductFetcher, error) {
			resources, err := BuildSharedResources(cfg, logger, SharedResourceOptions{})
			if err != nil {
				return nil, err
			}

			productFetcher := product.NewProductFetcher(
				resources.ManagementClient.GetRawJsonDataAdapter(),
				&cfg.Amazon,
				amazonProcessor,
			)

			return productFetcher, nil
		},
	}
}
