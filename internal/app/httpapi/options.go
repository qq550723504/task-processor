package httpapi

import (
	"os"
	"time"

	"task-processor/internal/productimage"
)

const defaultShutdownTimeout = 30 * time.Second

type Options struct {
	ConfigPath      string
	Port            int
	ShutdownSignal  chan os.Signal
	ShutdownTimeout time.Duration
	// SourceImageFetcher is for trusted process composition (for example local
	// test fixtures); nil preserves the production public-HTTPS fetcher.
	SourceImageFetcher productimage.SourceImageFetcher
}

func (o Options) shutdownTimeout() time.Duration {
	if o.ShutdownTimeout > 0 {
		return o.ShutdownTimeout
	}
	return defaultShutdownTimeout
}
