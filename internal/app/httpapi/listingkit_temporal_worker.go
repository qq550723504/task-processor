package httpapi

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/sirupsen/logrus"

	appruntime "task-processor/internal/app/runtime"
	kernelmodule "task-processor/internal/kernel/module"
	listingkithttpapi "task-processor/internal/listingkit/httpapi"
	sheinclient "task-processor/internal/shein/client"
)

// RunListingKitTemporalWorker boots the minimum ListingKit dependencies required
// to host the SHEIN publish Temporal worker as a standalone process.
func RunListingKitTemporalWorker(logger *logrus.Logger, options Options) error {
	deps, err := buildRuntimeDeps(logger, options.ConfigPath)
	if err != nil {
		return fmt.Errorf("build runtime deps: %w", err)
	}
	defer closeResources(logger, deps.shared.closers)

	sheinclient.ConfigureLoginAccountFromConfig(deps.shared.cfg)

	_, err = newListingKitFeatureBuilder().build(logger, deps, listingKitFeatureBuildOptions{
		includeImage: true,
	})
	if err != nil {
		return fmt.Errorf("build listingkit runtime prerequisites: %w", err)
	}

	temporalRuntime, err := listingkithttpapi.BuildTemporalRuntime(listingkithttpapi.TemporalRuntimeBuildInput{
		Logger:  logger,
		Runtime: newListingKitRuntimeBuildInput(logger, deps).Runtime,
	})
	if err != nil {
		return fmt.Errorf("build listing kit temporal runtime: %w", err)
	}
	if temporalRuntime == nil {
		return fmt.Errorf("listing kit temporal runtime is unavailable")
	}
	deps.addClosers(temporalRuntime.Closers...)

	bundle, err := appruntime.BuildTemporalRuntimeBundleFromModules(deps.shared.cfg, []kernelmodule.Module{temporalRuntime.Module})
	if err != nil {
		return fmt.Errorf("build temporal runtime bundle: %w", err)
	}
	workerClosers, err := bundle.Start()
	if err != nil {
		return fmt.Errorf("start temporal runtime bundle: %w", err)
	}
	if len(workerClosers) > 0 {
		defer closeResources(logger, workerClosers)
	}

	sigChan := options.ShutdownSignal
	if sigChan == nil {
		sigChan = make(chan os.Signal, 1)
		signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	}
	sig := <-sigChan
	logger.Infof("received signal %v, shutting down listingkit temporal worker", sig)
	return nil
}
