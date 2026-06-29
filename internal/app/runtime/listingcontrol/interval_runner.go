package listingcontrol

import (
	"context"
	"time"
)

type intervalRunner struct {
	interval time.Duration
	run      func(context.Context) error
	now      func() time.Time
	lastRun  time.Time
}

func newIntervalRunner(interval time.Duration, run func(context.Context) error, now func() time.Time) *intervalRunner {
	if now == nil {
		now = time.Now
	}
	return &intervalRunner{
		interval: interval,
		run:      run,
		now:      now,
	}
}

func (r *intervalRunner) RunIfDue(ctx context.Context) error {
	if r == nil || r.run == nil || r.interval <= 0 {
		return nil
	}
	now := r.now()
	if !r.lastRun.IsZero() && now.Sub(r.lastRun) < r.interval {
		return nil
	}
	if err := r.run(ctx); err != nil {
		return err
	}
	r.lastRun = now
	return nil
}
