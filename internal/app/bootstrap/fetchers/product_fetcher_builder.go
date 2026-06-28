package fetchers

import (
	"fmt"

	"task-processor/internal/app/ports"
	"task-processor/internal/core/config"
	appfetcher "task-processor/internal/crawler/fetcher"
	"task-processor/internal/infra/rabbitmq"
	"task-processor/internal/platformbase"
	"task-processor/internal/product"
)

func BuildPlatformProductFetcher(
	cfg *config.Config,
	platform string,
	rawJsonDataClient product.RawJsonDataClient,
	crawlSource ports.CrawlSource,
	rabbitmqClient *rabbitmq.Client,
) (appfetcher.ProductFetcher, error) {
	if rawJsonDataClient == nil {
		return nil, fmt.Errorf("raw json data client is required")
	}

	factory := appfetcher.NewFetcherFactory()
	fetcherType, err := platformbase.ResolvePlatformFetcherType(cfg, platform)
	if err != nil {
		return nil, err
	}

	if fetcherType == "" {
		return factory.CreateFetcherFromConfig(cfg, rawJsonDataClient, crawlSource, rabbitmqClient)
	}

	return factory.CreateFetcher(fetcherType, rawJsonDataClient, &cfg.Amazon, crawlSource, rabbitmqClient)
}
