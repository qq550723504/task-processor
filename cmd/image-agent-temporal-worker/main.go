package main

import (
	"context"
	"fmt"
	"os/signal"
	"syscall"

	"github.com/sirupsen/logrus"

	appruntime "task-processor/internal/app/runtime"
)

type dependencyResolver func() (appruntime.ImageAgentTemporalDependencies, error)
type temporalWorkerRunner func(context.Context, appruntime.ImageAgentTemporalDependencies, *logrus.Logger) error

func run(ctx context.Context, resolve dependencyResolver, start temporalWorkerRunner, logger *logrus.Logger) error {
	dependencies, err := resolve()
	if err != nil {
		return err
	}
	return start(ctx, dependencies, logger)
}

func resolveImageAgentTemporalDependencies() (appruntime.ImageAgentTemporalDependencies, error) {
	return appruntime.ImageAgentTemporalDependencies{}, fmt.Errorf("product image slot executor is not composed; complete Task 4 wiring before starting the image agent temporal worker")
}

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	logger := logrus.New()
	if err := run(ctx, resolveImageAgentTemporalDependencies, appruntime.RunImageAgentTemporalWorker, logger); err != nil {
		logger.Fatalf("image agent temporal worker exited: %v", err)
	}
}
