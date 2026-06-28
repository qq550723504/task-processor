package platformbase

import (
	"testing"

	"task-processor/internal/core/config"
	appfetcher "task-processor/internal/crawler/fetcher"

	"github.com/stretchr/testify/require"
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

func TestResolvePlatformFetcherType(t *testing.T) {
	t.Parallel()

	cfg := config.NewDefaultConfig()
	cfg.Platforms.Shein.FetchMode = "local"
	cfg.Platforms.Temu.FetchMode = "remote-api"

	fetcherType, err := ResolvePlatformFetcherType(cfg, "shein")
	require.NoError(t, err)
	require.Equal(t, appfetcher.LocalFetcher, fetcherType)

	fetcherType, err = ResolvePlatformFetcherType(cfg, "temu")
	require.NoError(t, err)
	require.Equal(t, appfetcher.RemoteAPIFetcher, fetcherType)

	fetcherType, err = ResolvePlatformFetcherType(cfg, "")
	require.NoError(t, err)
	require.Equal(t, appfetcher.FetcherType(""), fetcherType)
}

func TestResolvePlatformFetcherTypeRejectsInvalidMode(t *testing.T) {
	t.Parallel()

	cfg := config.NewDefaultConfig()
	cfg.Platforms.Shein.FetchMode = "invalid"

	_, err := ResolvePlatformFetcherType(cfg, "shein")
	require.Error(t, err)
}
