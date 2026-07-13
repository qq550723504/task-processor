package httpapi

import (
	"os"
	"time"
)

const defaultShutdownTimeout = 30 * time.Second

type Options struct {
	ConfigPath      string
	Port            int
	ShutdownSignal  chan os.Signal
	ShutdownTimeout time.Duration
}

func (o Options) shutdownTimeout() time.Duration {
	if o.ShutdownTimeout > 0 {
		return o.ShutdownTimeout
	}
	return defaultShutdownTimeout
}
