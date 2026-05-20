package fetcher

import (
	appfetcher "task-processor/internal/app/crawler/fetcher"
	"task-processor/internal/core/config"
	"task-processor/internal/infra/rabbitmq"
	domainProduct "task-processor/internal/product"
)

type FetcherType = appfetcher.FetcherType

const (
	LocalFetcher       = appfetcher.LocalFetcher
	DistributedFetcher = appfetcher.DistributedFetcher
	RemoteAPIFetcher   = appfetcher.RemoteAPIFetcher
)

type ProductFetcher = appfetcher.ProductFetcher
type FetcherFactory = appfetcher.FetcherFactory
type DistributedProductFetcher = appfetcher.DistributedProductFetcher
type RemoteAPIProductFetcher = appfetcher.RemoteAPIProductFetcher

func NewFetcherFactory() *FetcherFactory {
	return appfetcher.NewFetcherFactory()
}

func NewDistributedProductFetcher(
	rawJsonDataClient domainProduct.RawJsonDataClient,
	amazonConfig *config.AmazonConfig,
	rabbitmqClient *rabbitmq.Client,
) (*DistributedProductFetcher, error) {
	return appfetcher.NewDistributedProductFetcher(rawJsonDataClient, amazonConfig, rabbitmqClient)
}

func NewRemoteAPIProductFetcher(
	rawJsonDataClient domainProduct.RawJsonDataClient,
	amazonConfig *config.AmazonConfig,
) (*RemoteAPIProductFetcher, error) {
	return appfetcher.NewRemoteAPIProductFetcher(rawJsonDataClient, amazonConfig)
}
