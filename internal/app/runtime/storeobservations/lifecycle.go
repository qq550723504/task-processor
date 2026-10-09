package storeobservationsruntime

import (
	"sync/atomic"
	o "task-processor/internal/marketplace/shein/observations"
)

type Worker interface {
	Start() error
	Stop()
}

// Lifecycle is assembled before serving; only readiness changes concurrently.
// The current process owner starts and stops it with the existing HTTP runtime.
type Lifecycle struct {
	Worker Worker
	ready  atomic.Bool
	failed atomic.Bool
}

func (l *Lifecycle) Ready() bool { return l != nil && l.ready.Load() }
func (l *Lifecycle) Unavailable(error) {
	if l != nil {
		l.failed.Store(true)
		l.ready.Store(false)
	}
}
func (l *Lifecycle) Start() error {
	if l == nil || l.Worker == nil {
		return o.ErrUnavailable
	}
	l.failed.Store(false)
	if err := l.Worker.Start(); err != nil {
		l.ready.Store(false)
		return err
	}
	l.ready.Store(true)
	if l.failed.Load() {
		l.ready.Store(false)
		l.Worker.Stop()
		return o.ErrUnavailable
	}
	return nil
}
func (l *Lifecycle) Stop() {
	if l != nil {
		l.ready.Store(false)
		if l.Worker != nil {
			l.Worker.Stop()
		}
	}
}
