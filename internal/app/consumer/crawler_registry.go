package consumer

import (
	"fmt"

	"task-processor/internal/app/crawler/distributed"
	"task-processor/internal/app/processor"
	"task-processor/internal/core/config"
	"task-processor/internal/crawler/amazon"
	"task-processor/internal/infra/rabbitmq"

	"github.com/sirupsen/logrus"
)

type CrawlerRegistry struct {
	config                 *config.Config
	logger                 *logrus.Logger
	rabbitmqClient         *rabbitmq.Client
	amazonCrawlerCreator   AmazonCrawlerCreator
	productFetcherProvider ProductFetcherProvider
}

func NewCrawlerRegistry(cfg *config.Config, logger *logrus.Logger, rabbitmqClient *rabbitmq.Client) *CrawlerRegistry {
	return NewCrawlerRegistryWithDependencies(cfg, logger, rabbitmqClient, CrawlerRegistryDependencies{
		AmazonCrawlerCreator: defaultAmazonCrawlerCreator,
	})
}

func NewCrawlerRegistryWithDependencies(cfg *config.Config, logger *logrus.Logger, rabbitmqClient *rabbitmq.Client, deps CrawlerRegistryDependencies) *CrawlerRegistry {
	return &CrawlerRegistry{
		config:                 cfg,
		logger:                 logger,
		rabbitmqClient:         rabbitmqClient,
		amazonCrawlerCreator:   deps.AmazonCrawlerCreator,
		productFetcherProvider: deps.ProductFetcherProvider,
	}
}

func (r *CrawlerRegistry) RegisterCrawlerProcessor(serviceManager *ServiceManager, sharedAmazonProcessor *amazon.AmazonProcessor) error {
	r.logger.Info("Registering Amazon crawler processor...")
	if r.amazonCrawlerCreator == nil {
		return fmt.Errorf("amazon crawler creator not configured")
	}
	if r.productFetcherProvider == nil {
		return fmt.Errorf("product fetcher provider not configured")
	}

	var amazonProcessor *amazon.AmazonProcessor
	if sharedAmazonProcessor != nil {
		r.logger.Info("Using shared Amazon processor for crawler registration")
		amazonProcessor = sharedAmazonProcessor
	} else {
		r.logger.Info("Creating dedicated Amazon processor for crawler registration")
		amazonProcessor = r.amazonCrawlerCreator(r.config, r.logger)
	}

	productFetcher, err := r.productFetcherProvider(r.config, r.logger, amazonProcessor)
	if err != nil {
		return fmt.Errorf("create product fetcher: %w", err)
	}

	taskSubmitter := NewTaskSubmitter(r.rabbitmqClient, r.logger)
	rabbitmqPublisher := distributed.NewRabbitMQAdapter(r.rabbitmqClient)

	crawlerProcessor := processor.NewCrawlerProcessor(
		r.logger,
		amazonProcessor,
		productFetcher,
		taskSubmitter,
		rabbitmqPublisher,
	)

	if err := serviceManager.RegisterProcessor("amazon.crawler", crawlerProcessor); err != nil {
		return fmt.Errorf("register amazon crawler processor: %w", err)
	}

	r.logger.Info("Amazon crawler processor registered")
	return nil
}

func (r *CrawlerRegistry) RegisterAmazonCrawler(serviceManager *ServiceManager) error {
	r.logger.Info("Registering Amazon crawler...")
	if r.amazonCrawlerCreator == nil {
		return fmt.Errorf("amazon crawler creator not configured")
	}
	if r.productFetcherProvider == nil {
		return fmt.Errorf("product fetcher provider not configured")
	}

	amazonProcessor := r.amazonCrawlerCreator(r.config, r.logger)
	productFetcher, err := r.productFetcherProvider(r.config, r.logger, amazonProcessor)
	if err != nil {
		return fmt.Errorf("create product fetcher: %w", err)
	}

	taskSubmitter := NewTaskSubmitter(r.rabbitmqClient, r.logger)
	rabbitmqPublisher := distributed.NewRabbitMQAdapter(r.rabbitmqClient)

	crawlerProcessor := processor.NewCrawlerProcessor(
		r.logger,
		amazonProcessor,
		productFetcher,
		taskSubmitter,
		rabbitmqPublisher,
	)

	if err := serviceManager.RegisterProcessor("amazon.crawler", crawlerProcessor); err != nil {
		return fmt.Errorf("register amazon crawler: %w", err)
	}

	r.logger.Info("Amazon crawler registered")
	return nil
}

func (r *CrawlerRegistry) Register1688Crawler(serviceManager *ServiceManager) error {
	r.logger.Info("Registering 1688 crawler...")
	r.logger.Warn("1688 crawler is not implemented yet")
	return fmt.Errorf("1688 crawler is not implemented yet")
}
