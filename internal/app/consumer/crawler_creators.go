package consumer

import (
	"task-processor/internal/app/runner"
	"task-processor/internal/core/config"
	"task-processor/internal/crawler/amazon"
	"task-processor/internal/product"

	"github.com/sirupsen/logrus"
)

type AmazonCrawlerCreator func(cfg *config.Config, logger *logrus.Logger) *amazon.AmazonProcessor

type ProductFetcherProvider func(cfg *config.Config, logger *logrus.Logger, crawlSource runner.CrawlSource) (*product.ProductFetcher, error)
