package consumer

import (
	"task-processor/internal/app/runner"
	"task-processor/internal/core/config"
	"task-processor/internal/product"

	"github.com/sirupsen/logrus"
)

type amazonCrawlerCreator func(cfg *config.Config, logger *logrus.Logger) runner.CrawlSource

type productFetcherProvider func(cfg *config.Config, logger *logrus.Logger, crawlSource runner.CrawlSource) (*product.ProductFetcher, error)
