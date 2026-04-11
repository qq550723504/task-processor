package bootstrap

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
	cfg.Platforms.Temu.FetchMode = "distributed"

	fetcherType, err := resolvePlatformFetcherType(cfg, "shein")
	require.NoError(t, err)
	require.Equal(t, appfetcher.LocalFetcher, fetcherType)

	fetcherType, err = resolvePlatformFetcherType(cfg, "temu")
	require.NoError(t, err)
	require.Equal(t, appfetcher.DistributedFetcher, fetcherType)

	fetcherType, err = resolvePlatformFetcherType(cfg, "")
	require.NoError(t, err)
	require.Equal(t, appfetcher.FetcherType(""), fetcherType)
}

func TestResolvePlatformFetcherTypeRejectsInvalidMode(t *testing.T) {
	t.Parallel()

	cfg := config.NewDefaultConfig()
	cfg.Platforms.Shein.FetchMode = "invalid"

	_, err := resolvePlatformFetcherType(cfg, "shein")
	require.Error(t, err)
}
