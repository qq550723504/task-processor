package runner

import (
	"testing"

	"task-processor/internal/core/config"
	appfetcher "task-processor/internal/crawler/fetcher"
	"task-processor/internal/product"
)

func TestBuildRuntimeProductFetcherUsesPlatformFetchModeForShein(t *testing.T) {
	t.Parallel()

	cfg := config.NewDefaultConfig()
	cfg.Platforms.Shein.Enabled = true
	cfg.Platforms.Shein.FetchMode = "local"
	cfg.RabbitMQ.Enabled = true
	cfg.Amazon.Enabled = false

	service := &processorServiceImpl{
		rawJSONDataClient: fakeRawJSONDataClient{},
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

func TestShouldStartSchedulerServiceUsesProcessorSwitch(t *testing.T) {
	t.Parallel()

	cfg := config.NewDefaultConfig()
	cfg.Processor.SchedulerEnabled = true
	if !shouldStartSchedulerService(cfg) {
		t.Fatal("expected scheduler service to start when processor scheduler switch is enabled")
	}

	cfg.Processor.SchedulerEnabled = false
	if shouldStartSchedulerService(cfg) {
		t.Fatal("expected scheduler service to be skipped when processor scheduler switch is disabled")
	}
}

type fakeRawJSONDataClient struct{}

func (fakeRawJSONDataClient) GetRawJsonData(*product.RawJsonReq) (*product.RawJsonResp, error) {
	return nil, nil
}

func (fakeRawJSONDataClient) CreateRawJsonData(*product.RawJsonCreateReq) (int64, error) {
	return 0, nil
}
