package local

import (
	"time"

	"task-processor/internal/core/config"
)

type LocalRuntime struct {
	resources       *RuntimeResources
	provider        *LocalDataProvider
	cookieProvider  SheinCookieProvider
	imageDownloader *ImageDownloader
}

type LocalRuntimeOptions struct {
	SheinCookieProvider SheinCookieProvider
}

func NewLocalRuntime(resources *RuntimeResources, options LocalRuntimeOptions) *LocalRuntime {
	if resources == nil {
		return nil
	}
	return &LocalRuntime{
		resources:      resources,
		provider:       NewLocalDataProviderFromResources(resources),
		cookieProvider: options.SheinCookieProvider,
	}
}

func NewRedisSheinCookieProvider(cfg *config.RedisConfig) (SheinCookieProvider, error) {
	return newRedisSheinCookieProvider(cfg)
}

func (r *LocalRuntime) GetImageDownloader() interface {
	DownloadImage(url string) ([]byte, error)
} {
	if r == nil {
		return nil
	}
	if r.imageDownloader == nil {
		r.imageDownloader = NewImageDownloader(120 * time.Second)
	}
	return r.imageDownloader
}

func (r *LocalRuntime) ValidateLocalListingRuntimeFields() (map[string]bool, error) {
	report, err := ValidateLocalListingRuntime(r.resources)
	return report.Fields(), err
}
