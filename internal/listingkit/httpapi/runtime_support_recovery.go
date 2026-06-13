package httpapi

import (
	"context"
	"sync"
	"time"

	"github.com/sirupsen/logrus"

	"task-processor/internal/listingkit"
)

func startTaskRecoverySweep(input BuildModuleInput, bundle *ServiceBundle, closers *closerStack) {
	recoveryService, ok := any(bundle.runtime.service).(listingkit.TaskRecoveryService)
	if !ok || recoveryService == nil || closers == nil {
		return
	}

	interval := BuildListingKitTaskRecoverySweepInterval()
	limit := BuildListingKitTaskRecoverySweepLimit()
	ticker := time.NewTicker(interval)
	closers.Add(startTaskRecoverySweepLoop(taskRecoverySweepLoopConfig{
		recoveryService: recoveryService,
		logger:          input.ServiceInput.Logger,
		limit:           limit,
		now: func() time.Time {
			return time.Now().UTC()
		},
		ticks: ticker.C,
		stopTicker: func() {
			ticker.Stop()
		},
	}))
}

type taskRecoverySweepLoopConfig struct {
	recoveryService listingkit.TaskRecoveryService
	logger          *logrus.Logger
	limit           int
	now             func() time.Time
	ticks           <-chan time.Time
	stopTicker      func()
}

func startTaskRecoverySweepLoop(config taskRecoverySweepLoopConfig) func() error {
	if config.recoveryService == nil {
		return func() error { return nil }
	}
	nowFn := config.now
	if nowFn == nil {
		nowFn = func() time.Time { return time.Now().UTC() }
	}
	ctx, cancel := context.WithCancel(context.Background())
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		runTaskRecoverySweep(config, ctx, nowFn())
		for {
			select {
			case <-ctx.Done():
				return
			case now, ok := <-config.ticks:
				if !ok {
					return
				}
				runTaskRecoverySweep(config, ctx, now.UTC())
			}
		}
	}()
	return func() error {
		if config.stopTicker != nil {
			config.stopTicker()
		}
		cancel()
		wg.Wait()
		return nil
	}
}

func runTaskRecoverySweep(config taskRecoverySweepLoopConfig, ctx context.Context, now time.Time) {
	recovered, err := config.recoveryService.RunRecoverySweep(ctx, now.UTC(), config.limit)
	if config.logger == nil {
		return
	}
	logger := config.logger.WithField("component", "listingkit/httpapi").WithField("recovery_limit", config.limit)
	switch {
	case err != nil:
		logger.WithError(err).Warn("listingkit task recovery sweep failed")
	case recovered > 0:
		logger.WithField("recovered", recovered).Info("listingkit task recovery sweep requeued blocked tasks")
	}
}
