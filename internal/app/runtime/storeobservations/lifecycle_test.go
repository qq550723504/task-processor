package storeobservationsruntime

import (
	"errors"
	"github.com/stretchr/testify/require"
	"testing"
)

type lifecycleWorker struct {
	start func() error
	stops int
}

func (w *lifecycleWorker) Start() error { return w.start() }
func (w *lifecycleWorker) Stop()        { w.stops++ }
func TestObservationLifecycleReadinessOnlyAfterSuccessfulStart(t *testing.T) {
	for _, mode := range []string{"success", "start-error", "fatal-during-start"} {
		t.Run(mode, func(t *testing.T) {
			l := &Lifecycle{}
			w := &lifecycleWorker{start: func() error {
				require.False(t, l.Ready())
				if mode == "start-error" {
					return errors.New("synthetic-start-error")
				}
				if mode == "fatal-during-start" {
					l.Unavailable(errors.New("synthetic-fatal"))
				}
				return nil
			}}
			l.Worker = w
			require.False(t, l.Ready())
			if mode == "success" {
				require.NoError(t, l.Start())
				require.True(t, l.Ready())
				l.Unavailable(errors.New("fatal"))
				require.False(t, l.Ready())
			} else {
				require.Error(t, l.Start())
				require.False(t, l.Ready())
			}
			l.Stop()
			require.False(t, l.Ready())
			require.GreaterOrEqual(t, w.stops, 1)
		})
	}
}
