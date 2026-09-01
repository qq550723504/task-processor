package fetcher

import (
	"testing"

	"task-processor/internal/core/config"
	"task-processor/internal/platform/queue/rabbitmq"
)

func TestResolveProductFetcherTypePreservesRuntimeModes(t *testing.T) {
	t.Parallel()

	remoteConfig := &config.AmazonConfig{RemoteAPI: config.RemoteAPIConfig{Enabled: true}}
	for _, tc := range []struct {
		name           string
		amazonConfig   *config.AmazonConfig
		rabbitmqClient *rabbitmq.Client
		want           FetcherType
	}{
		{name: "local", want: LocalFetcher},
		{name: "remote", amazonConfig: remoteConfig, want: RemoteAPIFetcher},
		{name: "distributed overrides remote", amazonConfig: remoteConfig, rabbitmqClient: &rabbitmq.Client{}, want: DistributedFetcher},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := resolveProductFetcherType(tc.amazonConfig, tc.rabbitmqClient); got != tc.want {
				t.Fatalf("resolveProductFetcherType() = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestProductFetcherBuilderBuildsRemoteFetcherWithBoundDependencies(t *testing.T) {
	t.Parallel()

	builder := NewProductFetcherBuilder(nil, nil)
	productFetcher, err := builder.Build(&config.AmazonConfig{
		RemoteAPI: config.RemoteAPIConfig{
			Enabled: true,
			BaseURL: "http://amazon-crawler-api:8080",
			Timeout: 30,
		},
	}, nil)
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}
	if got := productFetcher.GetStats()["type"]; got != "remote-api" {
		t.Fatalf("Build() fetcher type = %v, want remote-api", got)
	}
}
