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
		Repository: store.NewMemoryRepository(), SlotExecutor: runtimeSlotExecutor{}, Publisher: runtimePublisher{}, PublisherV3: runtimePublisher{},
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
	require.Equal(t, imageagenttemporal.WorkerWireModeV3, gotConfig.WireMode)
	require.Empty(t, gotConfig.TaskQueue)
	require.True(t, worker.started)

	require.NoError(t, closeFn())
	require.True(t, worker.stopped)
	require.True(t, clientClosed)
}

func TestImageAgentTemporalRuntimeForwardsExplicitWorkerModeAndQueue(t *testing.T) {
	t.Setenv(envImageAgentTemporalEnabled, "true")
	for _, test := range []struct {
		name  string
		mode  imageagenttemporal.WorkerWireMode
		queue string
	}{
		{name: "v2 compatibility", mode: imageagenttemporal.WorkerWireModeV2, queue: imageagenttemporal.TaskQueue},
		{name: "v3 recovery", mode: imageagenttemporal.WorkerWireModeV3, queue: imageagenttemporal.TaskQueueV3},
	} {
		t.Run(test.name, func(t *testing.T) {
			worker := &recordingImageAgentWorker{}
			var got imageagenttemporal.WorkerConfig
			closeFn, err := startImageAgentTemporalWorkerWithOptionsAndDependencies(ImageAgentTemporalDependencies{
				Repository: store.NewMemoryRepository(), SlotExecutor: runtimeSlotExecutor{}, Publisher: runtimePublisher{}, PublisherV3: runtimePublisher{},
				StagedSlotExecutor: runtimeSlotExecutor{}, ArtifactStore: runtimeArtifactStore{},
			}, ImageAgentTemporalWorkerOptions{WireMode: test.mode, TaskQueue: test.queue}, imageAgentTemporalRuntimeDependencies{
				Dial: func(string, string) (sdkclient.Client, func() error, error) {
					return nil, func() error { return nil }, nil
				},
				NewWorker: func(config imageagenttemporal.WorkerConfig) (imageAgentWorker, error) {
					got = config
					return worker, nil
				},
			})
			require.NoError(t, err)
			require.Equal(t, test.mode, got.WireMode)
			require.Equal(t, test.queue, got.TaskQueue)
			require.NoError(t, closeFn())
		})
	}
}

func TestImageAgentTemporalRuntimeDoesNotReplaceInvalidExplicitWorkerConfiguration(t *testing.T) {
	t.Setenv(envImageAgentTemporalEnabled, "true")
	want := errors.New("NewWorker rejected opposite/default mismatch")
	var got imageagenttemporal.WorkerConfig
	_, err := startImageAgentTemporalWorkerWithOptionsAndDependencies(ImageAgentTemporalDependencies{
		Repository: store.NewMemoryRepository(), SlotExecutor: runtimeSlotExecutor{}, Publisher: runtimePublisher{}, PublisherV3: runtimePublisher{},
		StagedSlotExecutor: runtimeSlotExecutor{}, ArtifactStore: runtimeArtifactStore{},
	}, ImageAgentTemporalWorkerOptions{WireMode: imageagenttemporal.WorkerWireModeV2, TaskQueue: imageagenttemporal.TaskQueueV3}, imageAgentTemporalRuntimeDependencies{
		Dial: func(string, string) (sdkclient.Client, func() error, error) {
			return nil, func() error { return nil }, nil
		},
		NewWorker: func(config imageagenttemporal.WorkerConfig) (imageAgentWorker, error) {
			got = config
			return nil, want
		},
	})
	require.ErrorIs(t, err, want)
	require.Equal(t, imageagenttemporal.WorkerWireModeV2, got.WireMode)
	require.Equal(t, imageagenttemporal.TaskQueueV3, got.TaskQueue)
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

func TestImageAgentCompatibilityCanaryDialsWithoutProductDependencies(t *testing.T) {
	dialed, ran, closed := false, false, false
	err := runImageAgentCompatibilityCanaryWithDependencies(context.Background(), nil, "image-agent-manual-v3-canary", imageAgentCompatibilityCanaryDependencies{
		Dial: func(address, namespace string) (sdkclient.Client, func() error, error) {
			dialed = true
			require.Equal(t, "localhost:7233", address)
			require.Equal(t, "default", namespace)
			return nil, func() error { closed = true; return nil }, nil
		},
		RunCanary: func(_ context.Context, client sdkclient.Client, queue string) error {
			ran = true
			require.Nil(t, client)
			require.Equal(t, "image-agent-manual-v3-canary", queue)
			return nil
		},
	})
	require.NoError(t, err)
	require.True(t, dialed)
	require.True(t, ran)
	require.True(t, closed)
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

func (runtimeArtifactStore) PublicURL(key string) string { return "https://cdn.example.test/" + key }

func (runtimeArtifactStore) PrepareSlotArtifacts(objectstore.PrepareSlotArtifactsInput) (objectstore.PreparedSlotArtifacts, error) {
	return objectstore.PreparedSlotArtifacts{}, nil
}
func (runtimeArtifactStore) PreserveSlotArtifacts(context.Context, imageagent.SlotExternalEffectIdentity, objectstore.PreparedSlotArtifacts) error {
	return nil
}
func (runtimeArtifactStore) RecoverSlotArtifacts(_ context.Context, _ imageagent.SlotExternalEffectIdentity, expected imageagent.StagingManifest) (objectstore.PreparedSlotArtifacts, error) {
	return objectstore.PreparedSlotArtifacts{Manifest: expected}, nil
}
func (runtimeArtifactStore) EnsureStaged(context.Context, objectstore.PreparedSlotArtifacts) error {
	return nil
}
func (runtimeArtifactStore) Finalize(context.Context, imageagent.StagingManifest) (imageagent.FinalManifest, error) {
	return imageagent.FinalManifest{}, nil
}
func (runtimeArtifactStore) FinalizeWithProgress(context.Context, imageagent.StagingManifest, func(context.Context, int) error) (imageagent.FinalManifest, error) {
	return imageagent.FinalManifest{}, nil
}

type runtimePublisher struct{}

func (runtimePublisher) PublishApproved(context.Context, imageagent.PublishApprovedInput) (imageagent.PublicationAcknowledgement, error) {
	return imageagent.PublicationAcknowledgement{}, nil
}

func (runtimePublisher) PublishApprovedV3(context.Context, imageagent.PublishApprovedV3Input) (imageagent.PublicationAcknowledgement, error) {
	return imageagent.PublicationAcknowledgement{}, nil
}
