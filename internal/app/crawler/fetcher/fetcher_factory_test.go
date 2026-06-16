package fetcher

import (
	"strings"
	"testing"

	"task-processor/internal/core/config"
	"task-processor/internal/infra/rabbitmq"
)

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

	fetcher, err := factory.CreateFetcherFromConfig(cfg, nil, nil, &rabbitmq.Client{})
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
