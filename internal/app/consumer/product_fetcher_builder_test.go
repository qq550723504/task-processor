package consumer

import (
	"testing"

	appfetcher "task-processor/internal/app/crawler/fetcher"
	"task-processor/internal/core/config"

	"github.com/stretchr/testify/require"
)

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
}
