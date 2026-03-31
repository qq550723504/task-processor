package platformbase

import (
	"testing"

	appfetcher "task-processor/internal/app/crawler/fetcher"
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

	if _, ok := productFetcher.(*appfetcher.RemoteAPIProductFetcher); !ok {
		t.Fatalf("Build() returned %T, want *RemoteAPIProductFetcher", productFetcher)
	}
}
