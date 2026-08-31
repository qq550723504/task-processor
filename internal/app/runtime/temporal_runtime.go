package runtime

import (
	"context"
	"fmt"
	"os"
	"strings"
	"sync"

	"github.com/sirupsen/logrus"
	sdkclient "go.temporal.io/sdk/client"

	"task-processor/internal/listingkit"
	listingtemporal "task-processor/internal/listingkit/temporal"
	platformtemporal "task-processor/internal/platform/temporal"
)

const (
	envListingKitTemporalEnabled   = "LISTINGKIT_TEMPORAL_ENABLED"
	envListingKitTemporalAddress   = "LISTINGKIT_TEMPORAL_ADDRESS"
	envListingKitTemporalNamespace = "LISTINGKIT_TEMPORAL_NAMESPACE"
	envListingKitTemporalWorker    = "LISTINGKIT_TEMPORAL_START_WORKER"
)

func DialListingKitSheinPublishTemporalClient(logger *logrus.Logger) (listingkit.SheinPublishWorkflowClient, func() error, error) {
	if !envBool(envListingKitTemporalEnabled) {
		return nil, nil, nil
	}
	rawClient, closeClient, config, err := dialListingKitTemporalSDKClient(context.Background(), platformtemporal.Dial)
	if err != nil {
		return nil, nil, fmt.Errorf("dial temporal: %w", err)
	}
	if logger != nil {
		logger.WithFields(logrus.Fields{
			"address":   config.Address,
			"namespace": config.Namespace,
			"taskQueue": listingtemporal.TaskQueueSheinSubmitPublishName(),
		}).Info("connected listingkit shein publish temporal client")
	}
	return listingtemporal.NewClient(rawClient), closeClient, nil
}

type listingKitTemporalWorkerService interface {
	listingkit.SheinPublishActivityHostSource
	listingkit.LayerWorkflowActivityHostSource
}

type listingKitTemporalWorker interface {
	Start() error
	Stop()
}

type listingKitTemporalDial func(context.Context, platformtemporal.Config) (sdkclient.Client, func() error, error)

type listingKitTemporalRuntimeDependencies struct {
	Dial            listingKitTemporalDial
	NewActivityHost func(any) (listingkit.SheinPublishActivityHost, error)
	NewLayerHost    func(any) (listingkit.LayerWorkflowActivityHost, error)
	NewWorker       func(listingtemporal.WorkerConfig) (listingKitTemporalWorker, error)
}

func StartListingKitSheinPublishTemporalWorker(svc listingKitTemporalWorkerService, logger *logrus.Logger) (func() error, error) {
	return startListingKitSheinPublishTemporalWorkerWithDependencies(context.Background(), svc, logger, defaultListingKitTemporalRuntimeDependencies())
}

func startListingKitSheinPublishTemporalWorkerWithDependencies(ctx context.Context, svc any, logger *logrus.Logger, dependencies listingKitTemporalRuntimeDependencies) (func() error, error) {
	if !envBool(envListingKitTemporalEnabled) {
		return nil, nil
	}
	defaults := defaultListingKitTemporalRuntimeDependencies()
	if dependencies.Dial == nil {
		dependencies.Dial = defaults.Dial
	}
	if dependencies.NewActivityHost == nil {
		dependencies.NewActivityHost = defaults.NewActivityHost
	}
	if dependencies.NewLayerHost == nil {
		dependencies.NewLayerHost = defaults.NewLayerHost
	}
	if dependencies.NewWorker == nil {
		dependencies.NewWorker = defaults.NewWorker
	}
	rawClient, closeClient, config, err := dialListingKitTemporalSDKClient(ctx, dependencies.Dial)
	if err != nil {
		return nil, fmt.Errorf("dial temporal: %w", err)
	}

	host, err := dependencies.NewActivityHost(svc)
	if err != nil {
		_ = closeClient()
		return nil, err
	}
	layerHost, err := dependencies.NewLayerHost(svc)
	if err != nil {
		_ = closeClient()
		return nil, err
	}
	worker, err := dependencies.NewWorker(listingtemporal.WorkerConfig{
		Client:    rawClient,
		Host:      host,
		LayerHost: layerHost,
	})
	if err != nil {
		_ = closeClient()
		return nil, err
	}
	if err := worker.Start(); err != nil {
		_ = closeClient()
		return nil, fmt.Errorf("start temporal worker: %w", err)
	}
	if logger != nil {
		logger.WithFields(logrus.Fields{
			"address":   config.Address,
			"namespace": config.Namespace,
			"taskQueue": listingtemporal.TaskQueueSheinSubmitPublishName(),
		}).Info("started listingkit shein publish temporal worker")
	}

	var once sync.Once
	var closeErr error
	closeFn := func() error {
		once.Do(func() {
			worker.Stop()
			closeErr = closeClient()
		})
		return closeErr
	}
	return closeFn, nil
}

func ShouldStartListingKitSheinPublishTemporalWorkerInProcess() bool {
	raw := strings.TrimSpace(os.Getenv(envListingKitTemporalWorker))
	if raw == "" {
		return true
	}
	return envBool(envListingKitTemporalWorker)
}

func dialListingKitTemporalSDKClient(ctx context.Context, dial listingKitTemporalDial) (sdkclient.Client, func() error, platformtemporal.Config, error) {
	address := strings.TrimSpace(os.Getenv(envListingKitTemporalAddress))
	if address == "" {
		address = "localhost:7233"
	}
	namespace := strings.TrimSpace(os.Getenv(envListingKitTemporalNamespace))
	if namespace == "" {
		namespace = "default"
	}
	config := platformtemporal.Config{Address: address, Namespace: namespace}
	rawClient, closeClient, err := dial(ctx, config)
	if err != nil {
		return nil, nil, platformtemporal.Config{}, err
	}
	return rawClient, closeClient, config, nil
}

func defaultListingKitTemporalRuntimeDependencies() listingKitTemporalRuntimeDependencies {
	return listingKitTemporalRuntimeDependencies{
		Dial:            platformtemporal.Dial,
		NewActivityHost: listingkit.NewSheinPublishActivityHost,
		NewLayerHost:    listingkit.NewLayerWorkflowActivityHost,
		NewWorker: func(config listingtemporal.WorkerConfig) (listingKitTemporalWorker, error) {
			return listingtemporal.NewWorker(config)
		},
	}
}

func envBool(name string) bool {
	switch strings.ToLower(strings.TrimSpace(os.Getenv(name))) {
	case "1", "true", "yes", "y", "on", "enabled":
		return true
	default:
		return false
	}
}
