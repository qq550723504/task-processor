package consumer

import (
	"fmt"

	"task-processor/internal/app/bootstrap"
	"task-processor/internal/app/crawler/distributed"
	"task-processor/internal/app/processor"
	"task-processor/internal/core/config"
	"task-processor/internal/crawler/amazon"
	"task-processor/internal/infra/rabbitmq"
	"task-processor/internal/product"

	"github.com/sirupsen/logrus"
)

type CrawlerRegistry struct {
	config         *config.Config
	logger         *logrus.Logger
	rabbitmqClient *rabbitmq.Client
}

func NewCrawlerRegistry(cfg *config.Config, logger *logrus.Logger, rabbitmqClient *rabbitmq.Client) *CrawlerRegistry {
	return &CrawlerRegistry{
		config:         cfg,
		logger:         logger,
		rabbitmqClient: rabbitmqClient,
	}
}

func (r *CrawlerRegistry) RegisterCrawlerProcessor(serviceManager *ServiceManager, sharedAmazonProcessor *amazon.AmazonProcessor) error {
	r.logger.Info(" 娉ㄥ唽Amazon鐖櫕澶勭悊鍣?..")

	var amazonProcessor *amazon.AmazonProcessor
	if sharedAmazonProcessor != nil {
		r.logger.Info(" 澶嶇敤鍏变韓鐨凙mazon澶勭悊鍣紙閬垮厤閲嶅鍒濆鍖栨祻瑙堝櫒姹狅級")
		amazonProcessor = sharedAmazonProcessor
	} else {
		r.logger.Info(" 鍒涘缓鏂扮殑Amazon澶勭悊鍣?")
		amazonProcessor = amazon.CreateProcessor(r.config, r.logger)
	}

	productFetcher, err := r.createProductFetcher(amazonProcessor)
	if err != nil {
		return fmt.Errorf("鍒涘缓浜у搧鑾峰彇鍣ㄥけ璐? %w", err)
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
		return fmt.Errorf("娉ㄥ唽Amazon鐖櫕澶勭悊鍣ㄥけ璐? %w", err)
	}

	r.logger.Info(" Amazon鐖櫕澶勭悊鍣ㄦ敞鍐屾垚鍔?")
	return nil
}

func (r *CrawlerRegistry) RegisterAmazonCrawler(serviceManager *ServiceManager) error {
	r.logger.Info(" 娉ㄥ唽 Amazon 鐖櫕澶勭悊鍣?..")

	amazonProcessor := amazon.CreateProcessor(r.config, r.logger)
	productFetcher, err := r.createProductFetcher(amazonProcessor)
	if err != nil {
		return fmt.Errorf("鍒涘缓浜у搧鑾峰彇鍣ㄥけ璐? %w", err)
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
		return fmt.Errorf("娉ㄥ唽 Amazon 鐖櫕澶勭悊鍣ㄥけ璐? %w", err)
	}

	r.logger.Info(" Amazon 鐖櫕澶勭悊鍣ㄦ敞鍐屾垚鍔?")
	return nil
}

func (r *CrawlerRegistry) Register1688Crawler(serviceManager *ServiceManager) error {
	r.logger.Info(" 娉ㄥ唽 1688 鐖櫕澶勭悊鍣?..")
	r.logger.Warn(" 1688 鐖櫕澶勭悊鍣ㄥ皻鏈疄鐜?")
	return fmt.Errorf("1688 鐖櫕澶勭悊鍣ㄥ皻鏈疄鐜?")
}

func (r *CrawlerRegistry) createProductFetcher(amazonProcessor *amazon.AmazonProcessor) (*product.ProductFetcher, error) {
	resources, err := bootstrap.BuildSharedResources(r.config, r.logger, bootstrap.SharedResourceOptions{})
	if err != nil {
		return nil, err
	}

	productFetcher := product.NewProductFetcher(
		resources.ManagementClient.GetRawJsonDataAdapter(),
		&r.config.Amazon,
		amazonProcessor,
	)

	return productFetcher, nil
}
