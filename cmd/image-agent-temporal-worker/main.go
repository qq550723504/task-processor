package main

import (
	"context"
	"flag"
	"os/signal"
	"syscall"

	"github.com/sirupsen/logrus"

	appruntime "task-processor/internal/app/runtime"
	imageagentworker "task-processor/internal/app/worker/imageagent"
	"task-processor/internal/pkg/appenv"
)

type resolvedDependencies struct {
	dependencies appruntime.ImageAgentTemporalDependencies
	close        func() error
}

type dependencyResolver func() (resolvedDependencies, error)
type temporalWorkerRunner func(context.Context, appruntime.ImageAgentTemporalDependencies, *logrus.Logger) error

func run(ctx context.Context, resolve dependencyResolver, start temporalWorkerRunner, logger *logrus.Logger) error {
	resolved, err := resolve()
	if err != nil {
		return err
	}
	if resolved.close != nil {
		defer resolved.close()
	}
	return start(ctx, resolved.dependencies, logger)
}

func resolveImageAgentTemporalDependencies(configPath string, logger *logrus.Logger) dependencyResolver {
	return func() (resolvedDependencies, error) {
		dependencies, closeFn, err := imageagentworker.ResolveImageAgentTemporalDependencies(configPath, logger)
		return resolvedDependencies{dependencies: dependencies, close: closeFn}, err
	}
}

func main() {
	configPath := flag.String("config", "config/config-prod.yaml", "config file path")
	logLevel := flag.String("log-level", "info", "log level")
	flag.Parse()
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	logger := appenv.SetupLoggerWithLevel(*logLevel)
	if err := run(ctx, resolveImageAgentTemporalDependencies(*configPath, logger), appruntime.RunImageAgentTemporalWorker, logger); err != nil {
		logger.Fatalf("image agent temporal worker exited: %v", err)
	}
}
