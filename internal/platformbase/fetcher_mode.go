package platformbase

import (
	"fmt"
	"strings"

	"task-processor/internal/core/config"
	appfetcher "task-processor/internal/crawler/fetcher"
)

func ResolvePlatformFetcherType(cfg *config.Config, platform string) (appfetcher.FetcherType, error) {
	if cfg == nil {
		return "", nil
	}

	mode := "auto"
	switch strings.ToLower(strings.TrimSpace(platform)) {
	case "temu":
		mode = strings.TrimSpace(cfg.Platforms.Temu.FetchMode)
	case "shein":
		mode = strings.TrimSpace(cfg.Platforms.Shein.FetchMode)
	}

	switch strings.ToLower(mode) {
	case "", "auto":
		return "", nil
	case "local":
		return appfetcher.LocalFetcher, nil
	case "distributed":
		return appfetcher.DistributedFetcher, nil
	case "remote-api", "remoteapi", "remote_api":
		return appfetcher.RemoteAPIFetcher, nil
	default:
		return "", fmt.Errorf("unsupported fetch mode %q for platform %q", mode, platform)
	}
}

func PlatformUsesLocalFetcher(cfg *config.Config, platform string) bool {
	fetcherType, err := ResolvePlatformFetcherType(cfg, platform)
	return err == nil && fetcherType == appfetcher.LocalFetcher
}
