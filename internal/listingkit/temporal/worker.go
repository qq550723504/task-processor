package temporal

import (
	"fmt"

	sdkclient "go.temporal.io/sdk/client"
	sdkworker "go.temporal.io/sdk/worker"
	sdkworkflow "go.temporal.io/sdk/workflow"

	"task-processor/internal/listingkit"
)

type WorkerConfig struct {
	Client    sdkclient.Client
	Host      listingkit.SheinPublishActivityHost
	LayerHost listingkit.LayerWorkflowActivityHost
}

func NewWorker(config WorkerConfig) (sdkworker.Worker, error) {
	if config.Client == nil {
		return nil, fmt.Errorf("temporal client is required")
	}
	if config.Host == nil {
		return nil, fmt.Errorf("shein publish activity host is required")
	}
	worker := sdkworker.New(config.Client, TaskQueueSheinSubmitPublish, sdkworker.Options{})
	if err := RegisterWorker(worker, config.Host, config.LayerHost); err != nil {
		return nil, err
	}
	return worker, nil
}

func RegisterWorker(worker sdkworker.Worker, host listingkit.SheinPublishActivityHost, layerHost listingkit.LayerWorkflowActivityHost) error {
	if worker == nil {
		return fmt.Errorf("temporal worker is required")
	}
	if host == nil {
		return fmt.Errorf("shein publish activity host is required")
	}
	if layerHost == nil {
		return fmt.Errorf("layer workflow activity host is required")
	}
	worker.RegisterWorkflowWithOptions(PublishWorkflow, sdkworkflow.RegisterOptions{Name: "PublishWorkflow"})
	worker.RegisterWorkflowWithOptions(StandardProductWorkflow, sdkworkflow.RegisterOptions{Name: "StandardProductWorkflow"})
	worker.RegisterWorkflowWithOptions(PlatformAdaptWorkflow, sdkworkflow.RegisterOptions{Name: "PlatformAdaptWorkflow"})
	if err := RegisterSubmitActivities(worker, &SubmitActivities{Host: host}); err != nil {
		return err
	}
	return RegisterLayerActivities(worker, &LayerActivities{Host: layerHost})
}
