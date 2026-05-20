package httpapi

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/sirupsen/logrus"

	appruntime "task-processor/internal/app/runtime"
	listingkithttpapi "task-processor/internal/listingkit/httpapi"
)

// RunListingKitTemporalWorker boots the minimum ListingKit dependencies required
// to host the SHEIN publish Temporal worker as a standalone process.
func RunListingKitTemporalWorker(logger *logrus.Logger, options Options) error {
	deps, err := buildRuntimeDeps(logger, options.ConfigPath)
	if err != nil {
		return fmt.Errorf("build runtime deps: %w", err)
	}
	defer closeResources(logger, deps.closers)

	configureSheinLoginService(deps.cfg)

	if _, err := buildProductModule(logger, deps); err != nil {
		return fmt.Errorf("build product module: %w", err)
	}
	if _, err := buildImageModule(logger, deps); err != nil {
		return fmt.Errorf("build image module: %w", err)
	}

	bundle, err := listingkithttpapi.BuildService(newListingKitBuildServiceInput(logger, deps))
	if err != nil {
		return fmt.Errorf("build listing kit service: %w", err)
	}
	deps.closers = append(deps.closers, bundle.Closers...)

	workerCloser, err := appruntime.StartListingKitSheinPublishTemporalWorker(bundle.Service, logger)
	if err != nil {
		return fmt.Errorf("start listing kit temporal worker: %w", err)
	}
	if workerCloser != nil {
		defer closeResources(logger, []func() error{workerCloser})
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
