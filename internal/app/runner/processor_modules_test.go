package runner

import (
	"testing"

	appfetcher "task-processor/internal/crawler/fetcher"
	"task-processor/internal/core/config"
	"task-processor/internal/infra/clients/management"
)

func TestBuildRuntimeProductFetcherUsesPlatformFetchModeForShein(t *testing.T) {
	t.Parallel()

	cfg := config.NewDefaultConfig()
	cfg.Platforms.Shein.Enabled = true
	cfg.Platforms.Shein.FetchMode = "local"
	cfg.RabbitMQ.Enabled = true
	cfg.Amazon.Enabled = false

	service := &processorServiceImpl{
		managementClient: management.NewClientManager(nil),
	}

	fetcher, err := buildRuntimeProductFetcher(cfg, service, "shein")
	if err != nil {
		t.Fatalf("buildRuntimeProductFetcher() error = %v", err)
	}

	if _, ok := fetcher.(*appfetcher.RemoteAPIProductFetcher); ok {
		t.Fatal("buildRuntimeProductFetcher() returned remote api fetcher, want local fetcher")
	}
	if _, ok := fetcher.(*appfetcher.DistributedProductFetcher); ok {
		t.Fatal("buildRuntimeProductFetcher() returned distributed fetcher, want local fetcher")
	}
}
