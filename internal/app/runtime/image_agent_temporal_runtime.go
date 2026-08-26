package runtime

import (
	"context"
	"fmt"
	"os"
	"strings"
	"sync"

	"github.com/sirupsen/logrus"
	sdkclient "go.temporal.io/sdk/client"

	"task-processor/internal/imageagent"
	imageagenttemporal "task-processor/internal/imageagent/temporal"
)

const (
	envImageAgentTemporalEnabled   = "IMAGE_AGENT_TEMPORAL_ENABLED"
	envImageAgentTemporalAddress   = "IMAGE_AGENT_TEMPORAL_ADDRESS"
	envImageAgentTemporalNamespace = "IMAGE_AGENT_TEMPORAL_NAMESPACE"
)

type ImageAgentTemporalDependencies struct {
	Repository   imageagent.Repository
	SlotExecutor imageagent.SlotExecutor
	Publisher    imageagent.ApprovedAssetPublisher
}

type imageAgentWorker interface {
	Start() error
	Stop()
}

type imageAgentTemporalRuntimeDependencies struct {
	Dial      func(address, namespace string) (sdkclient.Client, func() error, error)
	NewWorker func(imageagenttemporal.WorkerConfig) (imageAgentWorker, error)
}

func StartImageAgentTemporalWorker(dependencies ImageAgentTemporalDependencies, logger *logrus.Logger) (func() error, error) {
	closeFn, err := startImageAgentTemporalWorkerWithDependencies(dependencies, defaultImageAgentTemporalRuntimeDependencies())
	if err != nil {
		return nil, err
	}
	if closeFn != nil && logger != nil {
		logger.WithFields(logrus.Fields{
			"address":   envOrDefault(envImageAgentTemporalAddress, "localhost:7233"),
			"namespace": envOrDefault(envImageAgentTemporalNamespace, "default"),
			"taskQueue": imageagenttemporal.TaskQueueName(),
		}).Info("started image agent temporal worker")
	}
	return closeFn, nil
}

func DialImageAgentTemporalWorkflowClient(logger *logrus.Logger) (imageagent.WorkflowClient, func() error, error) {
	if !envBool(envImageAgentTemporalEnabled) {
		return nil, nil, nil
	}
	address := envOrDefault(envImageAgentTemporalAddress, "localhost:7233")
	namespace := envOrDefault(envImageAgentTemporalNamespace, "default")
	client, closeFn, err := dialImageAgentTemporal(address, namespace)
	if err != nil {
		return nil, nil, fmt.Errorf("dial image agent temporal: %w", err)
	}
	if logger != nil {
		logger.WithFields(logrus.Fields{"address": address, "namespace": namespace, "taskQueue": imageagenttemporal.TaskQueueName()}).Info("connected image agent temporal client")
	}
	return imageagenttemporal.NewClient(client), closeFn, nil
}

func RunImageAgentTemporalWorker(ctx context.Context, dependencies ImageAgentTemporalDependencies, logger *logrus.Logger) error {
	closeFn, err := StartImageAgentTemporalWorker(dependencies, logger)
	if err != nil {
		return err
	}
	if closeFn == nil {
		return fmt.Errorf("image agent temporal worker is disabled")
	}
	<-ctx.Done()
	return closeFn()
}

func startImageAgentTemporalWorkerWithDependencies(dependencies ImageAgentTemporalDependencies, runtimeDependencies imageAgentTemporalRuntimeDependencies) (func() error, error) {
	if !envBool(envImageAgentTemporalEnabled) {
		return nil, nil
	}
	activities, err := imageagenttemporal.NewActivities(imageagenttemporal.ActivityDependencies{
		Repository: dependencies.Repository, SlotExecutor: dependencies.SlotExecutor, Publisher: dependencies.Publisher,
	})
	if err != nil {
		return nil, err
	}
	defaults := defaultImageAgentTemporalRuntimeDependencies()
	if runtimeDependencies.Dial == nil {
		runtimeDependencies.Dial = defaults.Dial
	}
	if runtimeDependencies.NewWorker == nil {
		runtimeDependencies.NewWorker = defaults.NewWorker
	}
	address := envOrDefault(envImageAgentTemporalAddress, "localhost:7233")
	namespace := envOrDefault(envImageAgentTemporalNamespace, "default")
	client, closeClient, err := runtimeDependencies.Dial(address, namespace)
	if err != nil {
		return nil, fmt.Errorf("dial image agent temporal: %w", err)
	}
	worker, err := runtimeDependencies.NewWorker(imageagenttemporal.WorkerConfig{Client: client, Activities: activities})
	if err != nil {
		_ = closeClient()
		return nil, err
	}
	if err := worker.Start(); err != nil {
		worker.Stop()
		_ = closeClient()
		return nil, fmt.Errorf("start image agent temporal worker: %w", err)
	}
	var once sync.Once
	var closeErr error
	return func() error {
		once.Do(func() {
			worker.Stop()
			closeErr = closeClient()
		})
		return closeErr
	}, nil
}

func defaultImageAgentTemporalRuntimeDependencies() imageAgentTemporalRuntimeDependencies {
	return imageAgentTemporalRuntimeDependencies{
		Dial: dialImageAgentTemporal,
		NewWorker: func(config imageagenttemporal.WorkerConfig) (imageAgentWorker, error) {
			return imageagenttemporal.NewWorker(config)
		},
	}
}

func dialImageAgentTemporal(address, namespace string) (sdkclient.Client, func() error, error) {
	client, err := sdkclient.Dial(sdkclient.Options{HostPort: address, Namespace: namespace})
	if err != nil {
		return nil, nil, err
	}
	return client, func() error { client.Close(); return nil }, nil
}

func envOrDefault(name, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(name)); value != "" {
		return value
	}
	return fallback
}
