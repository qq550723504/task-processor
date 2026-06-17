package fetcher

import (
	"os"
	"strings"
	"testing"

	"task-processor/internal/core/config"
)

func TestContractsOwnFetcherTypeAndProductFetcherInterface(t *testing.T) {
	source, err := os.ReadFile("contracts.go")
	if err != nil {
		t.Fatalf("ReadFile(contracts.go) error = %v", err)
	}
	content := string(source)
	if !strings.Contains(content, "type FetcherType string") {
		t.Fatal("internal/crawler/fetcher should own FetcherType instead of aliasing app/crawler/fetcher")
	}
	if !strings.Contains(content, "type ProductFetcher interface") {
		t.Fatal("internal/crawler/fetcher should own the ProductFetcher contract interface")
	}
	if strings.Contains(content, "type FetcherType = appfetcher.FetcherType") {
		t.Fatal("FetcherType should not be an alias to internal/app/crawler/fetcher")
	}
	if strings.Contains(content, "type ProductFetcher = appfetcher.ProductFetcher") {
		t.Fatal("ProductFetcher should not be an alias to internal/app/crawler/fetcher")
	}
	if strings.Contains(content, "type RemoteAPIProductFetcher = appfetcher.RemoteAPIProductFetcher") {
		t.Fatal("RemoteAPIProductFetcher concrete app implementation should not be re-exported from the neutral fetcher package")
	}
	if strings.Contains(content, "type DistributedProductFetcher = appfetcher.DistributedProductFetcher") {
		t.Fatal("DistributedProductFetcher concrete app implementation should not be re-exported from the neutral fetcher package")
	}
	if strings.Contains(content, "appFactory") {
		t.Fatal("neutral fetcher factory should not store an app/crawler/fetcher factory")
	}
	if strings.Contains(content, ".appFactory.CreateFetcher") {
		t.Fatal("neutral fetcher factory should own selection logic instead of delegating to app factory CreateFetcher")
	}
}

func TestFetcherFactoryCreateDistributedRequiresRabbitMQClient(t *testing.T) {
	_, err := NewFetcherFactory().CreateFetcher(
		DistributedFetcher,
		nil,
		&config.AmazonConfig{Enabled: true},
		nil,
		nil,
	)
	if err == nil {
		t.Fatal("CreateFetcher() error = nil, want distributed RabbitMQ requirement")
	}
	if !strings.Contains(err.Error(), "distributed fetcher requires RabbitMQ client") {
		t.Fatalf("CreateFetcher() error = %v, want RabbitMQ requirement", err)
	}
}

func TestCreateFetcherFromConfigRejectsDistributedModeWithoutAmazonCrawler(t *testing.T) {
	factory := NewFetcherFactory()
	cfg := &config.Config{
		Amazon: config.AmazonConfig{
			Enabled: false,
			RemoteAPI: config.RemoteAPIConfig{
				Enabled: false,
			},
		},
		RabbitMQ: &config.RabbitMQConfig{
			Enabled: true,
		},
	}

	fetcher, err := factory.CreateFetcherFromConfig(cfg, nil, nil, nil)
	if err == nil {
		t.Fatal("CreateFetcherFromConfig() error = nil, want config validation failure")
	}
	if fetcher != nil {
		t.Fatalf("CreateFetcherFromConfig() fetcher = %#v, want nil", fetcher)
	}
	if !strings.Contains(err.Error(), "amazon.enabled") {
		t.Fatalf("CreateFetcherFromConfig() error = %q, want mention amazon.enabled", err)
	}
}

func TestGetRecommendedFetcherPrefersDistributedWhenRabbitMQConfigured(t *testing.T) {
	factory := NewFetcherFactory()

	got := factory.GetRecommendedFetcher(&config.Config{
		RabbitMQ: &config.RabbitMQConfig{Enabled: true, URL: "amqp://example"},
	})
	if got != DistributedFetcher {
		t.Fatalf("GetRecommendedFetcher() = %q, want distributed", got)
	}
}
