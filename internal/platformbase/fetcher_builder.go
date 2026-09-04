package platformbase

import (
	"task-processor/internal/core/config"
	"task-processor/internal/crawler/fetcher"
	"task-processor/internal/platform/queue/rabbitmq"
)

type ProductFetcherBuilder interface {
	Build(
		amazonConfig *config.AmazonConfig,
		rabbitmqClient *rabbitmq.Client,
	) (fetcher.ProductFetcher, error)
}

type ProductFetcherBuilderFunc func(
	amazonConfig *config.AmazonConfig,
	rabbitmqClient *rabbitmq.Client,
) (fetcher.ProductFetcher, error)

func (f ProductFetcherBuilderFunc) Build(
	amazonConfig *config.AmazonConfig,
	rabbitmqClient *rabbitmq.Client,
) (fetcher.ProductFetcher, error) {
	return f(amazonConfig, rabbitmqClient)
}
