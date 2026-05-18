package temporal

import (
	"fmt"

	sdkclient "go.temporal.io/sdk/client"
	sdkworker "go.temporal.io/sdk/worker"
	sdkworkflow "go.temporal.io/sdk/workflow"

	"task-processor/internal/listingkit"
)

type WorkerConfig struct {
	Client sdkclient.Client
	Host   listingkit.SheinPublishActivityHost
}

func NewWorker(config WorkerConfig) (sdkworker.Worker, error) {
	if config.Client == nil {
		return nil, fmt.Errorf("temporal client is required")
	}
	if config.Host == nil {
		return nil, fmt.Errorf("shein publish activity host is required")
	}
	worker := sdkworker.New(config.Client, TaskQueueSheinSubmitPublish, sdkworker.Options{})
	if err := RegisterWorker(worker, config.Host); err != nil {
		return nil, err
	}
	return worker, nil
}

func RegisterWorker(worker sdkworker.Worker, host listingkit.SheinPublishActivityHost) error {
	if worker == nil {
		return fmt.Errorf("temporal worker is required")
	}
	if host == nil {
		return fmt.Errorf("shein publish activity host is required")
	}
	worker.RegisterWorkflowWithOptions(PublishWorkflow, sdkworkflow.RegisterOptions{Name: "PublishWorkflow"})
	return RegisterSubmitActivities(worker, &SubmitActivities{Host: host})
}
