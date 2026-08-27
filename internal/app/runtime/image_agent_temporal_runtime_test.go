package runtime

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
	sdkclient "go.temporal.io/sdk/client"

	"task-processor/internal/imageagent"
	"task-processor/internal/imageagent/objectstore"
	"task-processor/internal/imageagent/store"
	imageagenttemporal "task-processor/internal/imageagent/temporal"
)

func TestImageAgentTemporalRuntimeDisabledDoesNotDial(t *testing.T) {
	t.Setenv(envImageAgentTemporalEnabled, "false")
	dialed := false
	closeFn, err := startImageAgentTemporalWorkerWithDependencies(ImageAgentTemporalDependencies{}, imageAgentTemporalRuntimeDependencies{
		Dial: func(string, string) (sdkclient.Client, func() error, error) {
			dialed = true
			return nil, nil, errors.New("must not dial")
		},
	})
	require.NoError(t, err)
	require.Nil(t, closeFn)
	require.False(t, dialed)
}

func TestImageAgentTemporalRuntimeComposesAndClosesWorker(t *testing.T) {
	t.Setenv(envImageAgentTemporalEnabled, "true")
	t.Setenv(envImageAgentTemporalAddress, "temporal.internal:7233")
	t.Setenv(envImageAgentTemporalNamespace, "listingkit")
	worker := &recordingImageAgentWorker{}
	clientClosed := false
	var gotAddress, gotNamespace string
	var gotConfig imageagenttemporal.WorkerConfig
	closeFn, err := startImageAgentTemporalWorkerWithDependencies(ImageAgentTemporalDependencies{
		Repository: store.NewMemoryRepository(), SlotExecutor: runtimeSlotExecutor{}, Publisher: runtimePublisher{},
		StagedSlotExecutor: runtimeSlotExecutor{}, ArtifactStore: runtimeArtifactStore{},
	}, imageAgentTemporalRuntimeDependencies{
		Dial: func(address, namespace string) (sdkclient.Client, func() error, error) {
			gotAddress, gotNamespace = address, namespace
			return nil, func() error { clientClosed = true; return nil }, nil
		},
		NewWorker: func(config imageagenttemporal.WorkerConfig) (imageAgentWorker, error) {
			gotConfig = config
			return worker, nil
		},
	})
	require.NoError(t, err)
	require.NotNil(t, closeFn)
	require.Equal(t, "temporal.internal:7233", gotAddress)
	require.Equal(t, "listingkit", gotNamespace)
	require.NotNil(t, gotConfig.Activities)
	require.True(t, worker.started)

	require.NoError(t, closeFn())
	require.True(t, worker.stopped)
	require.True(t, clientClosed)
}

func TestImageAgentTemporalRuntimeFailsClosedWithoutProductPorts(t *testing.T) {
	t.Setenv(envImageAgentTemporalEnabled, "true")
	_, err := startImageAgentTemporalWorkerWithDependencies(ImageAgentTemporalDependencies{Repository: store.NewMemoryRepository()}, imageAgentTemporalRuntimeDependencies{})
	require.ErrorContains(t, err, "slot executor")
}

func TestImageAgentTemporalRuntimeFailsClosedWithPartialV3Ports(t *testing.T) {
	t.Setenv(envImageAgentTemporalEnabled, "true")
	_, err := startImageAgentTemporalWorkerWithDependencies(ImageAgentTemporalDependencies{
		Repository: store.NewMemoryRepository(), SlotExecutor: runtimeSlotExecutor{}, Publisher: runtimePublisher{},
		StagedSlotExecutor: runtimeSlotExecutor{},
	}, imageAgentTemporalRuntimeDependencies{})
	require.ErrorContains(t, err, "durable artifact store")
}

type recordingImageAgentWorker struct {
	started bool
	stopped bool
}

func (w *recordingImageAgentWorker) Start() error { w.started = true; return nil }
func (w *recordingImageAgentWorker) Stop()        { w.stopped = true }

type runtimeSlotExecutor struct{}

func (runtimeSlotExecutor) ExecuteSlot(_ context.Context, input imageagent.SlotExecutionInput) (imageagent.SlotExecutionResult, error) {
	return imageagent.SlotExecutionResult{SlotID: input.Slot.ID, Attempt: input.Attempt}, nil
}

func (runtimeSlotExecutor) GenerateSlot(_ context.Context, input imageagent.SlotExecutionInput) (imageagent.SlotGeneratedOutput, error) {
	return imageagent.SlotGeneratedOutput{SlotID: input.Slot.ID, Attempt: input.Attempt, SourceAssetID: "source-1", Assets: []imageagent.GeneratedAsset{{URL: "C:/generated.png"}}}, nil
}

func (runtimeSlotExecutor) PublishSlot(_ context.Context, input imageagent.SlotExecutionInput, _ imageagent.SlotGeneratedOutput) (imageagent.SlotExecutionResult, error) {
	return imageagent.SlotExecutionResult{SlotID: input.Slot.ID, Attempt: input.Attempt}, nil
}

func (runtimeSlotExecutor) BuildSlotResult(_ context.Context, input imageagent.SlotExecutionInput, _ imageagent.PublishedSlotOutput) (imageagent.SlotExecutionResult, error) {
	return imageagent.SlotExecutionResult{SlotID: input.Slot.ID, Attempt: input.Attempt}, nil
}

type runtimeArtifactStore struct{}

func (runtimeArtifactStore) PrepareSlotArtifacts(objectstore.PrepareSlotArtifactsInput) (objectstore.PreparedSlotArtifacts, error) {
	return objectstore.PreparedSlotArtifacts{}, nil
}
func (runtimeArtifactStore) EnsureStaged(context.Context, objectstore.PreparedSlotArtifacts) error {
	return nil
}
func (runtimeArtifactStore) Finalize(context.Context, imageagent.StagingManifest) (imageagent.FinalManifest, error) {
	return imageagent.FinalManifest{}, nil
}

type runtimePublisher struct{}

func (runtimePublisher) PublishApproved(context.Context, imageagent.PublishApprovedInput) (imageagent.PublicationAcknowledgement, error) {
	return imageagent.PublicationAcknowledgement{}, nil
}
