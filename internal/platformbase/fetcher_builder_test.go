package platformbase

import (
	"testing"

	"task-processor/internal/core/config"
)

func TestDefaultProductFetcherBuilderBuildPrefersRemoteAPIWithoutCrawler(t *testing.T) {
	builder := NewDefaultProductFetcherBuilder()

	productFetcher, err := builder.Build(nil, &config.AmazonConfig{
		RemoteAPI: config.RemoteAPIConfig{
			Enabled: true,
			BaseURL: "http://amazon-crawler-api:8080",
			Timeout: 30,
		},
	}, nil, nil)
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}

	if got := productFetcher.GetStats()["type"]; got != "remote-api" {
		t.Fatalf("Build() fetcher type = %v, want remote-api", got)
	}
}
