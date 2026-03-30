package consumer

import (
	appfetcher "task-processor/internal/app/crawler/fetcher"
	"task-processor/internal/core/config"
	"task-processor/internal/crawler/amazon"
	"task-processor/internal/infra/clients/management"

	"github.com/sirupsen/logrus"
)

type SharedResources struct {
	ManagementClient *management.ClientManager
	CrawlSource      *amazon.AmazonProcessor
	ProductFetcher   appfetcher.ProductFetcher
}

type SharedResourceProvider func(cfg *config.Config, logger *logrus.Logger, needsAmazon bool) (*SharedResources, error)
