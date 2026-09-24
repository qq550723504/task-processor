package httpapi

import (
	"context"
	"net/http"
	"time"

	"github.com/sirupsen/logrus"
)

func startSubscriptionPurchaseRecoveryLoop(parent context.Context, server *http.Server, reconcile func(context.Context) error, interval time.Duration, logger *logrus.Logger) {
	if parent == nil || server == nil || reconcile == nil || interval <= 0 {
		return
	}
	ctx, cancel := context.WithCancel(parent)
	server.RegisterOnShutdown(cancel)
	go func() {
		defer cancel()
		run := func() {
			runCtx, runCancel := context.WithTimeout(ctx, 20*time.Second)
			defer runCancel()
			if err := reconcile(runCtx); err != nil && logger != nil {
				logger.WithError(err).Warn("subscription purchase recovery sweep failed")
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
