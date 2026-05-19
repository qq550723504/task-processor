package temporal

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	sdkclient "go.temporal.io/sdk/client"
	sdkworker "go.temporal.io/sdk/worker"

	"task-processor/internal/listingkit"
	sheinpub "task-processor/internal/publishing/shein"
)

func TestPublishWorkflowResumesAfterWorkerRestartAgainstRealTemporal(t *testing.T) {
	address := strings.TrimSpace(os.Getenv("TEMPORAL_E2E_ADDRESS"))
	if address == "" {
		t.Skip("set TEMPORAL_E2E_ADDRESS to run real Temporal integration test")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()

	rawClient, err := sdkclient.Dial(sdkclient.Options{
		HostPort:  address,
		Namespace: temporalE2ENamespace(),
	})
	if err != nil {
		t.Fatalf("dial temporal: %v", err)
	}
	defer rawClient.Close()

	state := &temporalE2EWorkerState{
		persistStarted: make(chan struct{}, 1),
		allowPersist:   make(chan struct{}),
		confirmed:      make(chan string, 1),
	}
	worker1, err := newTemporalE2EWorker(rawClient, &temporalE2EHost{id: "worker-1", state: state})
	if err != nil {
		t.Fatalf("new worker-1: %v", err)
	}
	if err := worker1.Start(); err != nil {
		t.Fatalf("start worker-1: %v", err)
	}
	defer worker1.Stop()

	taskID := fmt.Sprintf("temporal-e2e-%d", time.Now().UnixNano())
	run, err := rawClient.ExecuteWorkflow(ctx, sdkclient.StartWorkflowOptions{
		ID:        WorkflowIDForSheinPublish(taskID),
		TaskQueue: TaskQueueSheinSubmitPublish,
	}, PublishWorkflow, SheinPublishWorkflowInput{
		TaskID:          taskID,
		Platform:        "shein",
		Action:          "publish",
		RequestID:       "temporal-e2e-request",
		RequestedAt:     time.Now(),
		TriggeredByUser: "temporal-e2e",
	})
	if err != nil {
		t.Fatalf("execute workflow: %v", err)
	}

	select {
	case <-state.persistStarted:
	case <-ctx.Done():
		t.Fatalf("persist success did not start before timeout: %v", ctx.Err())
	}

	stopDone := make(chan struct{})
	go func() {
		worker1.Stop()
		close(stopDone)
	}()
	close(state.allowPersist)

	select {
	case <-stopDone:
	case <-ctx.Done():
		t.Fatalf("worker-1 did not stop before timeout: %v", ctx.Err())
	}

	worker2, err := newTemporalE2EWorker(rawClient, &temporalE2EHost{id: "worker-2", state: state})
	if err != nil {
		t.Fatalf("new worker-2: %v", err)
	}
	if err := worker2.Start(); err != nil {
		t.Fatalf("start worker-2: %v", err)
	}
	defer worker2.Stop()

	if err := run.Get(ctx, nil); err != nil {
		t.Fatalf("workflow completion: %v", err)
	}

	select {
	case confirmedBy := <-state.confirmed:
		if confirmedBy != "worker-2" {
			t.Fatalf("confirm remote executed by %q, want worker-2 after restart", confirmedBy)
		}
	case <-ctx.Done():
		t.Fatalf("confirm remote did not complete before timeout: %v", ctx.Err())
	}
}

func newTemporalE2EWorker(client sdkclient.Client, host listingkit.SheinPublishActivityHost) (sdkworker.Worker, error) {
	layerHost, ok := host.(listingkit.LayerWorkflowActivityHost)
	if !ok {
		return nil, errors.New("temporal e2e host does not implement LayerWorkflowActivityHost")
	}
	worker := sdkworker.New(client, TaskQueueSheinSubmitPublish, sdkworker.Options{})
	if err := RegisterWorker(worker, host, layerHost); err != nil {
		return nil, err
	}
	return worker, nil
}

func temporalE2ENamespace() string {
	namespace := strings.TrimSpace(os.Getenv("TEMPORAL_E2E_NAMESPACE"))
	if namespace == "" {
		return "default"
	}
	return namespace
}

type temporalE2EWorkerState struct {
	persistStarted chan struct{}
	allowPersist   chan struct{}
	confirmed      chan string

	mu              sync.Mutex
	beginCalls      int
	confirmWorkerID string
}

type temporalE2EHost struct {
	id    string
	state *temporalE2EWorkerState
}

func (h *temporalE2EHost) BeginSheinPublishAttempt(context.Context, listingkit.SheinPublishAttemptInput) error {
	h.state.mu.Lock()
	h.state.beginCalls++
	h.state.mu.Unlock()
	return nil
}

func (h *temporalE2EHost) ValidateSheinPublishReadiness(context.Context, listingkit.SheinPublishAttemptInput) error {
	return nil
}

func (h *temporalE2EHost) PrepareSheinPublishPayload(context.Context, listingkit.SheinPublishAttemptInput) (*listingkit.SheinPreparedSubmitPayload, error) {
	return &listingkit.SheinPreparedSubmitPayload{
		TaskID:           "temporal-e2e",
		Action:           "publish",
		RequestID:        "temporal-e2e-request",
		NeedsImageUpload: false,
		Snapshot:         &sheinpub.SubmitSnapshot{},
	}, nil
}

func (h *temporalE2EHost) UploadSheinPublishImages(context.Context, *listingkit.SheinPreparedSubmitPayload) (*listingkit.SheinPreparedSubmitPayload, error) {
	return nil, fmt.Errorf("upload should not run in temporal e2e")
}

func (h *temporalE2EHost) PreValidateSheinPublish(context.Context, *listingkit.SheinPreparedSubmitPayload) error {
	return nil
}

func (h *temporalE2EHost) SubmitSheinPublishRemote(context.Context, *listingkit.SheinPreparedSubmitPayload) (*listingkit.SheinRemoteSubmitResult, error) {
	return &listingkit.SheinRemoteSubmitResult{
		TaskID:       "temporal-e2e",
		Action:       "publish",
		RequestID:    "temporal-e2e-request",
		SupplierCode: "SUP-E2E",
		Response: &sheinpub.SubmissionResponse{
			Success: true,
		},
		Snapshot: &sheinpub.SubmitSnapshot{},
	}, nil
}

func (h *temporalE2EHost) PersistSheinPublishSuccess(context.Context, listingkit.SheinPersistSubmitSuccessInput) error {
	select {
	case h.state.persistStarted <- struct{}{}:
	default:
	}
	<-h.state.allowPersist
	return nil
}

func (h *temporalE2EHost) PersistSheinPublishFailure(context.Context, listingkit.SheinPersistSubmitFailureInput) error {
	return nil
}

func (h *temporalE2EHost) ConfirmSheinPublishRemote(context.Context, listingkit.SheinConfirmRemoteInput) (*listingkit.SheinRemoteConfirmResult, error) {
	h.state.mu.Lock()
	h.state.confirmWorkerID = h.id
	h.state.mu.Unlock()
	select {
	case h.state.confirmed <- h.id:
	default:
	}
	return &listingkit.SheinRemoteConfirmResult{
		TaskID:       "temporal-e2e",
		Action:       "publish",
		RequestID:    "temporal-e2e-request",
		RemoteStatus: "confirmed",
	}, nil
}

func (h *temporalE2EHost) BuildSheinTaskPreview(context.Context, string) (*listingkit.ListingKitPreview, error) {
	return nil, nil
}
