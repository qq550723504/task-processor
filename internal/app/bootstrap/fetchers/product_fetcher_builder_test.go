package fetchers

import (
	"testing"

	appfetcher "task-processor/internal/crawler/fetcher"
	"task-processor/internal/core/config"

	"github.com/stretchr/testify/require"
)

func TestResolvePlatformFetcherType(t *testing.T) {
	t.Parallel()

	cfg := config.NewDefaultConfig()
	cfg.Platforms.Shein.FetchMode = "local"
	cfg.Platforms.Temu.FetchMode = "distributed"

	fetcherType, err := ResolvePlatformFetcherType(cfg, "shein")
	require.NoError(t, err)
	require.Equal(t, appfetcher.LocalFetcher, fetcherType)

	fetcherType, err = ResolvePlatformFetcherType(cfg, "temu")
	require.NoError(t, err)
	require.Equal(t, appfetcher.DistributedFetcher, fetcherType)

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
