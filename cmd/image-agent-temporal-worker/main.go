package main

import (
	"context"
	"flag"
	"fmt"
	"os/signal"
	"strings"
	"syscall"

	"github.com/sirupsen/logrus"

	appruntime "task-processor/internal/app/runtime"
	imageagentworker "task-processor/internal/app/worker/imageagent"
	imageagenttemporal "task-processor/internal/imageagent/temporal"
	"task-processor/internal/pkg/appenv"
)

type resolvedDependencies struct {
	dependencies appruntime.ImageAgentTemporalDependencies
	close        func() error
}

type dependencyResolver func() (resolvedDependencies, error)
type temporalWorkerRunner func(context.Context, appruntime.ImageAgentTemporalDependencies, appruntime.ImageAgentTemporalWorkerOptions, *logrus.Logger) error
type compatibilityCanaryRunner func(context.Context, *logrus.Logger, string) error

func run(ctx context.Context, resolve dependencyResolver, start temporalWorkerRunner, options appruntime.ImageAgentTemporalWorkerOptions, logger *logrus.Logger) error {
	if err := validateProcessWorkerOptions(options); err != nil {
		return err
	}
	resolved, err := resolve()
	if err != nil {
		return err
	}
	if resolved.close != nil {
		defer resolved.close()
	}
	return start(ctx, resolved.dependencies, options, logger)
}

func validateProcessWorkerOptions(options appruntime.ImageAgentTemporalWorkerOptions) error {
	if _, err := options.WireMode.DefaultTaskQueue(); err != nil {
		return err
	}
	if strings.TrimSpace(options.TaskQueue) == "" {
		return fmt.Errorf("image agent temporal task queue is required")
	}
	return nil
}

func runCanary(ctx context.Context, start compatibilityCanaryRunner, logger *logrus.Logger, taskQueue string) error {
	return start(ctx, logger, taskQueue)
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
	canary := flag.Bool("canary", false, "run the side-effect-free v3 Temporal compatibility canary and exit")
	canaryTaskQueue := flag.String("canary-task-queue", imageagenttemporal.TaskQueueV3, "v3 Temporal task queue for -canary")
	wireMode := flag.String("wire-mode", "", "worker wire mode: v2 or v3 (required unless -canary)")
	taskQueue := flag.String("task-queue", "", "Temporal task queue polled by the worker (required unless -canary)")
	flag.Parse()
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	logger := appenv.SetupLoggerWithLevel(*logLevel)
	if *canary {
		if err := runCanary(ctx, appruntime.RunImageAgentCompatibilityCanary, logger, *canaryTaskQueue); err != nil {
			logger.Fatalf("image agent temporal compatibility canary failed: %v", err)
		}
		return
	}
	options := appruntime.ImageAgentTemporalWorkerOptions{WireMode: imageagenttemporal.WorkerWireMode(*wireMode), TaskQueue: *taskQueue}
	if err := run(ctx, resolveImageAgentTemporalDependencies(*configPath, logger), appruntime.RunImageAgentTemporalWorkerWithOptions, options, logger); err != nil {
		logger.Fatalf("image agent temporal worker exited: %v", err)
	}
}
