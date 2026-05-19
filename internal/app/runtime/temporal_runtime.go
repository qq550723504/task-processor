package runtime

import (
	"fmt"
	"os"
	"strings"

	"github.com/sirupsen/logrus"
	sdkclient "go.temporal.io/sdk/client"

	"task-processor/internal/listingkit"
	listingtemporal "task-processor/internal/listingkit/temporal"
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
	rawClient, address, namespace, err := dialListingKitTemporalSDKClient()
	if err != nil {
		return nil, nil, fmt.Errorf("dial temporal: %w", err)
	}
	if logger != nil {
		logger.WithFields(logrus.Fields{
			"address":   address,
			"namespace": namespace,
			"taskQueue": listingtemporal.TaskQueueSheinSubmitPublish,
		}).Info("connected listingkit shein publish temporal client")
	}
	closeFn := func() error {
		rawClient.Close()
		return nil
	}
	return listingtemporal.NewClient(rawClient), closeFn, nil
}

func StartListingKitSheinPublishTemporalWorker(svc listingkit.Service, logger *logrus.Logger) (func() error, error) {
	if !envBool(envListingKitTemporalEnabled) {
		return nil, nil
	}
	rawClient, address, namespace, err := dialListingKitTemporalSDKClient()
	if err != nil {
		return nil, fmt.Errorf("dial temporal: %w", err)
	}

	host, err := listingkit.NewSheinPublishActivityHost(svc)
	if err != nil {
		rawClient.Close()
		return nil, err
	}
	layerHost, err := listingkit.NewLayerWorkflowActivityHost(svc)
	if err != nil {
		rawClient.Close()
		return nil, err
	}
	worker, err := listingtemporal.NewWorker(listingtemporal.WorkerConfig{
		Client:    rawClient,
		Host:      host,
		LayerHost: layerHost,
	})
	if err != nil {
		rawClient.Close()
		return nil, err
	}
	if err := worker.Start(); err != nil {
		rawClient.Close()
		return nil, fmt.Errorf("start temporal worker: %w", err)
	}
	if logger != nil {
		logger.WithFields(logrus.Fields{
			"address":   address,
			"namespace": namespace,
			"taskQueue": listingtemporal.TaskQueueSheinSubmitPublish,
		}).Info("started listingkit shein publish temporal worker")
	}

	closeFn := func() error {
		worker.Stop()
		rawClient.Close()
		return nil
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

func dialListingKitTemporalSDKClient() (sdkclient.Client, string, string, error) {
	address := strings.TrimSpace(os.Getenv(envListingKitTemporalAddress))
	if address == "" {
		address = "localhost:7233"
	}
	namespace := strings.TrimSpace(os.Getenv(envListingKitTemporalNamespace))
	if namespace == "" {
		namespace = "default"
	}
	rawClient, err := sdkclient.Dial(sdkclient.Options{
		HostPort:  address,
		Namespace: namespace,
	})
	if err != nil {
		return nil, "", "", err
	}
	return rawClient, address, namespace, nil
}

func envBool(name string) bool {
	switch strings.ToLower(strings.TrimSpace(os.Getenv(name))) {
	case "1", "true", "yes", "y", "on", "enabled":
		return true
	default:
		return false
	}
}
