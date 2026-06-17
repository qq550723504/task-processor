package consumer

import (
	"fmt"
	"strings"

	"task-processor/internal/app/runner"
	"task-processor/internal/core/config"
	appfetcher "task-processor/internal/crawler/fetcher"
	"task-processor/internal/infra/clients/management"
	"task-processor/internal/infra/rabbitmq"
)

func BuildPlatformProductFetcher(
	cfg *config.Config,
	platform string,
	managementClient *management.ClientManager,
	crawlSource runner.CrawlSource,
	rabbitmqClient *rabbitmq.Client,
) (appfetcher.ProductFetcher, error) {
	if managementClient == nil {
		return nil, fmt.Errorf("management client is required")
	}

	factory := appfetcher.NewFetcherFactory()
	fetcherType, err := ResolvePlatformFetcherType(cfg, platform)
	if err != nil {
		return nil, err
	}

	if fetcherType == "" {
		return factory.CreateFetcherFromConfig(cfg, managementClient.GetRawJsonDataAdapter(), crawlSource, rabbitmqClient)
	}

	return factory.CreateFetcher(fetcherType, managementClient.GetRawJsonDataAdapter(), &cfg.Amazon, crawlSource, rabbitmqClient)
}

func ResolvePlatformFetcherType(cfg *config.Config, platform string) (appfetcher.FetcherType, error) {
	if cfg == nil {
		return "", nil
	}

	mode := "auto"
	switch strings.ToLower(strings.TrimSpace(platform)) {
	case "temu":
		mode = strings.TrimSpace(cfg.Platforms.Temu.FetchMode)
	case "shein":
		mode = strings.TrimSpace(cfg.Platforms.Shein.FetchMode)
	}

	switch strings.ToLower(mode) {
	case "", "auto":
		return "", nil
	case "local":
		return appfetcher.LocalFetcher, nil
	case "distributed":
		return appfetcher.DistributedFetcher, nil
	case "remote-api", "remoteapi", "remote_api":
		return appfetcher.RemoteAPIFetcher, nil
	default:
		return "", fmt.Errorf("unsupported fetch mode %q for platform %q", mode, platform)
	}
}

func PlatformUsesLocalFetcher(cfg *config.Config, platform string) bool {
	fetcherType, err := ResolvePlatformFetcherType(cfg, platform)
	return err == nil && fetcherType == appfetcher.LocalFetcher
}
