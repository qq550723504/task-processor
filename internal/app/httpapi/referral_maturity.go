package httpapi

import (
	"context"
	"net/http"
	"time"

	"github.com/sirupsen/logrus"
)

// startReferralMaturityLoop owns the referral-specific due-claim sweep. It is
// deliberately local to the application owner rather than a general-purpose
// scheduler: each run is transactionally idempotent in the referral store,
// and server shutdown cancels the loop without leaving a background worker.
func startReferralMaturityLoop(server *http.Server, mature func(context.Context, time.Time) error, interval time.Duration, logger *logrus.Logger) {
	if server == nil || mature == nil || interval <= 0 {
		return
	}
	ctx, cancel := context.WithCancel(context.Background())
	server.RegisterOnShutdown(cancel)
	go func() {
		defer cancel()
		run := func() {
			if err := mature(ctx, time.Now().UTC()); err != nil && logger != nil {
				logger.WithError(err).Warn("referral earnings maturity sweep failed")
			}
		}
		run()
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				run()
			case <-ctx.Done():
				return
			}
		}
	}()
}
