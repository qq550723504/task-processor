package temporal_test

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/stretchr/testify/require"
	"go.temporal.io/sdk/testsuite"
	"go.temporal.io/sdk/worker"

	"task-processor/internal/imageagent"
	"task-processor/internal/imageagent/objectstore"
	imageagentstore "task-processor/internal/imageagent/store"
	imageagenttemporal "task-processor/internal/imageagent/temporal"
	imageagenttools "task-processor/internal/imageagent/tools"
	"task-processor/internal/infra/storage"
	"task-processor/internal/productimage"
)

type podLossRecoveryAcceptanceResult struct {
	MainAssetID         string
	GalleryAssetIDs     []string
	PersistedJSON       []byte
	FirstPodDiscarded   bool
	RecoveryGenerations int
	ApprovedAssetIDs    []string
	PublishedObjects    int
}

func TestManualImageAgentAcceptanceRecoversAfterPodLossAndApprovesAllAssets(t *testing.T) {
	result := executePodLossRecoveryAcceptance(t)

	require.True(t, result.FirstPodDiscarded, "acceptance must remove the original temp directory and activity/executor instance")
	require.Zero(t, result.RecoveryGenerations, "the replacement activity must not invoke the provider again")
	require.NotEmpty(t, result.MainAssetID)
	require.Len(t, result.GalleryAssetIDs, 2)
	require.Equal(t, append([]string{result.MainAssetID}, result.GalleryAssetIDs...), result.ApprovedAssetIDs)
	require.Equal(t, 3, result.PublishedObjects)
	require.NotContains(t, string(result.PersistedJSON), "local_path")
	require.NotContains(t, string(result.PersistedJSON), "authorization")
}

func executePodLossRecoveryAcceptance(t *testing.T) podLossRecoveryAcceptanceResult {
	t.Helper()
	ctx := context.Background()
	repository := imageagentstore.NewMemoryRepository()
	plan := acceptancePlan(3, 3)
	run := imageagent.Run{
		ID: "run-pod-loss", BusinessTaskID: "task-pod-loss", TenantID: "tenant-a", UserID: "user-a",
		Mode: imageagent.RunModeManual, IdempotencyKey: "run-key-pod-loss", Status: imageagent.RunStatusExecuting,
		CurrentNode: "execute_slots", ActivePlanRevision: plan.Revision, Version: 1,
	}
	scope := imageagent.RunScope{TenantID: run.TenantID, OwnerUserID: run.UserID, RunID: run.ID}
	catalog, err := imageagent.NormalizeAssetCatalog(acceptanceAssetCatalog(3))
	require.NoError(t, err)
	slots := make([]imageagent.SlotProjection, len(plan.Slots))
	for index, slot := range plan.Slots {
		slots[index] = imageagent.SlotProjection{Slot: slot}
	}
	_, err = repository.InitializeRun(ctx, imageagent.ProjectionInitialization{
		Scope: scope, Run: run, Plan: plan, Catalog: catalog,
		Snapshot: imageagent.RunProjection{Run: run, Plan: plan, Slots: slots, AssetCatalog: catalog, ProjectionVersion: 1, LastEventID: 1},
		CommitID: "start:run-key-pod-loss", EventType: "run.created", EventPayload: json.RawMessage(`{}`),
	})
	require.NoError(t, err)

	transientDir := t.TempDir()
	transientPath := filepath.Join(transientDir, "generated.png")
	require.NoError(t, os.WriteFile(transientPath, acceptancePNG, 0o600))
	firstExecutor := newAcceptanceProductImageExecutor(transientPath)
	durableAPI := &podLossAcceptanceS3{objects: map[string]podLossAcceptanceS3Object{}}
	uploader := storage.NewS3UploaderWithAPI(durableAPI, storage.S3UploaderOptions{
		Bucket: "acceptance-assets", PublicBase: "https://cdn.example.test",
		ArtifactCapabilities: storage.ArtifactStorageCapabilities{Mode: storage.ArtifactStorageModeAWS},
	})
	durableStore, err := objectstore.NewS3DurableArtifactStore(uploader, objectstore.S3DurableArtifactStoreConfig{
		MaxArtifactBytes: 1 << 20, OperationTimeout: 5 * time.Second,
	})
	require.NoError(t, err)
	podLost := errors.New("original activity process lost after artifact_staged")
	firstActivities := newPodLossAcceptanceActivities(t, repository, firstExecutor, durableStore, acceptancePublisher{}, func(context.Context) (string, error) {
		return "", podLost
	})
	firstActivityIdentity := fmt.Sprintf("%p", firstActivities)

	for _, slot := range plan.Slots {
		input := podLossAcceptanceActivityInput(run, plan, catalog, slot)
		_, executeErr := firstActivities.ExecuteSlotV3(ctx, input)
		require.ErrorIs(t, executeErr, podLost)
		effect, effectErr := repository.(imageagent.SlotExternalEffectV3Repository).GetSlotExternalEffectV3(ctx, imageagent.SlotExternalEffectIdentity{
			RunScope: scope, PlanRevision: plan.Revision, SlotID: slot.ID, Attempt: 1,
		})
		require.NoError(t, effectErr)
		require.Equal(t, imageagent.SlotEffectV3ArtifactStaged, effect.Phase)
	}

	require.NoError(t, os.RemoveAll(transientDir))
	_, statErr := os.Stat(transientPath)
	require.ErrorIs(t, statErr, os.ErrNotExist)
	firstActivities = nil
	firstExecutor = nil

	recoveryExecutor := newAcceptanceProductImageExecutor(transientPath)
	approvalPublisher := &recordingAcceptanceApprovalPublisher{}
	recoveryActivities := newPodLossAcceptanceActivities(t, repository, recoveryExecutor, durableStore, approvalPublisher, func(context.Context) (string, error) {
		return "recovered-workflow-run/execute-slot-v3/2", nil
	})
	require.NotEqual(t, firstActivityIdentity, fmt.Sprintf("%p", recoveryActivities))

	for _, slot := range plan.Slots {
		input := podLossAcceptanceActivityInput(run, plan, catalog, slot)
		published, executeErr := recoveryActivities.ExecuteSlotV3(ctx, input)
		require.NoError(t, executeErr)
		require.Len(t, published.Candidates, 1)
		require.NoError(t, recoveryActivities.PersistSlotResultV3(ctx, imageagenttemporal.PersistSlotResultV3ActivityInput{
			RunID: run.ID, Identity: input.Identity, PlanRevision: plan.Revision,
			Result:     imageagenttemporal.SlotWorkflowV3Result{Published: published, Status: imageagent.SlotStatusAccepted},
			AttemptKey: input.IdempotencyKey,
		}))
	}

	projection, err := repository.GetProjection(ctx, scope)
	require.NoError(t, err)
	approvedIDs := make([]string, 0, len(projection.Slots))
	mainID := ""
	galleryIDs := make([]string, 0, len(projection.Slots)-1)
	for _, slot := range projection.Slots {
		require.Equal(t, imageagent.SlotStatusAccepted, slot.Slot.Status)
		require.Len(t, slot.Candidates, 1)
		candidateID := slot.Candidates[0].AssetID
		approvedIDs = append(approvedIDs, candidateID)
		if slot.Slot.Role == imageagent.SlotRoleMain {
			require.Empty(t, mainID, "acceptance plan must have exactly one main candidate")
			mainID = candidateID
		} else {
			galleryIDs = append(galleryIDs, candidateID)
		}
	}
	require.NotEmpty(t, mainID)
	require.NoError(t, recoveryActivities.PublishApprovedV3(ctx, imageagenttemporal.PublishApprovedV3ActivityInput{
		RunID: run.ID, Identity: imageagent.ExecutionIdentity{TenantID: run.TenantID, UserID: run.UserID},
		PlanRevision: plan.Revision, CandidateAssetIDs: approvedIDs, IdempotencyKey: "approve-pod-loss-v3",
	}))
	require.Equal(t, approvedIDs, approvalPublisher.input.CandidateAssetIDs)

	effects := make([]imageagent.SlotEffectV3Attempt, 0, len(plan.Slots))
	for _, slot := range plan.Slots {
		effect, effectErr := repository.(imageagent.SlotExternalEffectV3Repository).GetSlotExternalEffectV3(ctx, imageagent.SlotExternalEffectIdentity{
			RunScope: scope, PlanRevision: plan.Revision, SlotID: slot.ID, Attempt: 1,
		})
		require.NoError(t, effectErr)
		require.Equal(t, imageagent.SlotEffectV3PublicationComplete, effect.Phase)
		effects = append(effects, effect)
	}
	persistedJSON, err := json.Marshal(struct {
		Projection imageagent.RunProjection         `json:"projection"`
		Effects    []imageagent.SlotEffectV3Attempt `json:"effects"`
	}{Projection: projection, Effects: effects})
	require.NoError(t, err)

	return podLossRecoveryAcceptanceResult{
		MainAssetID: mainID, GalleryAssetIDs: galleryIDs, PersistedJSON: persistedJSON,
		FirstPodDiscarded: true, RecoveryGenerations: len(recoveryExecutor.calledIDs()),
		ApprovedAssetIDs: append([]string(nil), approvalPublisher.input.CandidateAssetIDs...),
		PublishedObjects: durableAPI.countPrefix("image-agent/public/"),
	}
}

func newAcceptanceProductImageExecutor(artifactPath string) *recordingAcceptanceExecutor {
	return &recordingAcceptanceExecutor{delegate: imageagenttools.NewProductImageSlotExecutor(imageagenttools.Dependencies{
		SubjectExtractor: acceptanceSubjectExtractor{}, WhiteBackgroundRenderer: acceptanceWhiteRenderer{artifactPath: artifactPath},
		SceneRenderer: acceptanceSceneRenderer{artifactPath: artifactPath},
	})}
}

func newPodLossAcceptanceActivities(
	t *testing.T,
	repository imageagent.Repository,
	executor *recordingAcceptanceExecutor,
	durableStore imageagenttemporal.DurableArtifactStore,
	publisherV3 imageagent.ApprovedAssetPublisherV3,
	publicationOwner func(context.Context) (string, error),
) *imageagenttemporal.Activities {
	t.Helper()
	activities, err := imageagenttemporal.NewActivities(imageagenttemporal.ActivityDependencies{
		Repository: repository, SlotExecutor: executor, Publisher: acceptancePublisher{}, PublisherV3: publisherV3,
		SlotEffectsV3: repository.(imageagent.SlotExternalEffectV3Repository), StagedSlotExecutor: executor,
		ArtifactStore: durableStore, PublicationOwner: publicationOwner, PublicationLeaseDuration: time.Minute,
	})
	require.NoError(t, err)
	return activities
}

func podLossAcceptanceActivityInput(run imageagent.Run, plan imageagent.Plan, catalog imageagent.AssetCatalog, slot imageagent.Slot) imageagenttemporal.ExecuteSlotV3ActivityInput {
	return imageagenttemporal.ExecuteSlotV3ActivityInput{
		RunID: run.ID, Identity: imageagent.ExecutionIdentity{TenantID: run.TenantID, UserID: run.UserID},
		PlanRevision: plan.Revision, Slot: slot, Attempt: 1,
		IdempotencyKey: "attempt:" + slot.IdempotencyKey, AssetCatalog: catalog,
	}
}

func TestManualImageAgentAcceptancePreservesSixSuccessfulSlotsWhenOneBlocks(t *testing.T) {
	plan := acceptancePlan(7, 9)
	result, calledSlotIDs, events := executeAcceptanceWorkflow(t, plan, "scene-2")

	require.Equal(t, imageagent.RunStatusBlocked, result.Status)
	require.NotNil(t, result.Block)
	require.Equal(t, "scene-2", result.Block.SlotID)
	require.Len(t, result.CompletedSlotIDs, 6)
	require.Len(t, result.Slots, 7)
	require.Equal(t, sortedSlotIDs(plan), sortedStrings(calledSlotIDs))
	require.Equal(t, 6, acceptedSlotEventCount(t, events))
}

func TestManualImageAgentAcceptanceAllowsElevenStandardSlotsThroughWorkflow(t *testing.T) {
	plan := acceptancePlan(11, 9)
	require.NoError(t, imageagent.ValidatePlan(plan))

	result, calledSlotIDs, _ := executeAcceptanceWorkflow(t, plan, "scene-10")

	require.Equal(t, imageagent.RunStatusBlocked, result.Status)
	require.Len(t, result.Slots, 11)
	require.Len(t, result.CompletedSlotIDs, 10)
	require.Equal(t, sortedSlotIDs(plan), sortedStrings(calledSlotIDs))
}

func executeAcceptanceWorkflow(t *testing.T, plan imageagent.Plan, invalidSlotID string) (imageagenttemporal.WorkflowResult, []string, []imageagent.RunEvent) {
	t.Helper()
	repository := imageagentstore.NewMemoryRepository()
	run := &imageagent.Run{
		ID: "run-acceptance", BusinessTaskID: "task-acceptance",
		TenantID: "tenant-a", UserID: "user-a", Mode: imageagent.RunModeManual,
		IdempotencyKey: "run-key-acceptance", Status: imageagent.RunStatusPlanning,
		CurrentNode: "plan", Version: 1,
	}
	scope := imageagent.RunScope{TenantID: run.TenantID, OwnerUserID: run.UserID, RunID: run.ID}
	catalog := acceptanceAssetCatalog(9)
	normalizedCatalog, err := imageagent.NormalizeAssetCatalog(catalog)
	require.NoError(t, err)
	run.ActivePlanRevision = plan.Revision
	initialSlots := make([]imageagent.SlotProjection, len(plan.Slots))
	for index, slot := range plan.Slots {
		initialSlots[index] = imageagent.SlotProjection{Slot: slot}
	}
	_, err = repository.InitializeRun(context.Background(), imageagent.ProjectionInitialization{
		Scope: scope, Run: *run, Plan: plan, Catalog: normalizedCatalog,
		Snapshot: imageagent.RunProjection{Run: *run, Plan: plan, Slots: initialSlots, AssetCatalog: normalizedCatalog, ProjectionVersion: 1, LastEventID: 1},
		CommitID: "start:run-key-acceptance", EventType: "run.created", EventPayload: json.RawMessage(`{}`),
	})
	require.NoError(t, err)

	artifactPath := filepath.Join(t.TempDir(), "generated.png")
	require.NoError(t, os.WriteFile(artifactPath, acceptancePNG, 0o600))
	delegate := imageagenttools.NewProductImageSlotExecutor(imageagenttools.Dependencies{
		SubjectExtractor:        acceptanceSubjectExtractor{},
		WhiteBackgroundRenderer: acceptanceWhiteRenderer{artifactPath: artifactPath},
		SceneRenderer:           acceptanceSceneRenderer{artifactPath: artifactPath},
	})
	executor := &recordingAcceptanceExecutor{delegate: delegate, invalidSlotID: invalidSlotID}
	activities, err := imageagenttemporal.NewActivities(imageagenttemporal.ActivityDependencies{
		Repository: repository, SlotExecutor: executor, Publisher: acceptancePublisher{}, PublisherV3: acceptancePublisher{},
		SlotEffectsV3: repository.(imageagent.SlotExternalEffectV3Repository), StagedSlotExecutor: executor,
		ArtifactStore: acceptanceDurableArtifactStore{},
	})
	require.NoError(t, err)

	var suite testsuite.WorkflowTestSuite
	env := suite.NewTestWorkflowEnvironment()
	env.SetWorkerOptions(worker.Options{DeadlockDetectionTimeout: 5 * time.Second})
	env.RegisterWorkflow(imageagenttemporal.ImageSlotWorkflowV3)
	require.NoError(t, imageagenttemporal.RegisterActivitiesForMode(env, activities, imageagenttemporal.WorkerWireModeV3))
	env.ExecuteWorkflow(imageagenttemporal.ImageAgentWorkflow, imageagenttemporal.WorkflowInput{
		RunID: run.ID, Mode: imageagent.RunModeManual,
		Identity: imageagent.ExecutionIdentity{TenantID: run.TenantID, UserID: run.UserID},
		Plan:     plan, AssetCatalog: normalizedCatalog, MaxConcurrentSlots: 4, WaitForCommands: false,
	})
	require.True(t, env.IsWorkflowCompleted())
	require.NoError(t, env.GetWorkflowError())
	var result imageagenttemporal.WorkflowResult
	require.NoError(t, env.GetWorkflowResult(&result))
	events, err := repository.ListEvents(context.Background(), scope, 0, 100)
	require.NoError(t, err)
	return result, executor.calledIDs(), events
}

func acceptanceAssetCatalog(count int) imageagent.AssetCatalog {
	assets := make([]imageagent.AuthorizedAsset, 0, count+1)
	for index := 1; index <= count; index++ {
		id := fmt.Sprintf("source-%d", index)
		assets = append(assets, imageagent.AuthorizedAsset{
			ID: id, Type: imageagent.AuthorizedAssetSource,
			DisplayURL: fmt.Sprintf("https://source.example/%s.png", id),
			Width:      1200, Height: 1200,
		})
	}
	assets = append(assets, imageagent.AuthorizedAsset{ID: "style-modern", Type: imageagent.AuthorizedAssetStyle})
	return imageagent.AssetCatalog{Assets: assets}
}

func acceptancePlan(slotCount, sourceCount int) imageagent.Plan {
	sourceIDs := make([]string, sourceCount)
	for index := range sourceIDs {
		sourceIDs[index] = fmt.Sprintf("source-%d", index+1)
	}
	slots := make([]imageagent.Slot, slotCount)
	for index := range slots {
		id := fmt.Sprintf("scene-%d", index)
		role := imageagent.SlotRoleScene
		if index == 0 {
			id = "main-1"
			role = imageagent.SlotRoleMain
		}
		slots[index] = imageagent.Slot{
			ID: id, Role: role,
			SourceAssetIDs:    []string{sourceIDs[index%len(sourceIDs)]},
			StyleReferenceIDs: []string{"style-modern"},
			Brief:             id,
			IdempotencyKey:    "slot-key-" + id,
			Status:            imageagent.SlotStatusPending,
		}
	}
	return imageagent.Plan{
		Revision: 1, IdempotencyKey: "plan-key-acceptance",
		SourceAssetIDs: sourceIDs, StyleReferenceIDs: []string{"style-modern"},
		Slots: slots, CreatedBy: "user-a",
	}
}

type recordingAcceptanceExecutor struct {
	delegate      acceptanceExecutorDelegate
	invalidSlotID string
	mu            sync.Mutex
	calls         []string
}

type acceptanceExecutorDelegate interface {
	imageagent.StagedSlotExecutor
	imageagent.RecoverableSlotExecutor
}

func (e *recordingAcceptanceExecutor) QuoteSlot(context.Context, imageagent.SlotExecutionInput, imageagent.BudgetPolicy) (imageagent.SlotUsageQuote, error) {
	maximum := imageagent.UsageVector{Images: 1, AgentSteps: 1}
	return imageagent.SlotUsageQuote{Maximum: maximum, Operations: []imageagent.SlotUsageOperation{{Name: "acceptance_provider", Fingerprint: "acceptance-provider-v1", Maximum: maximum, MaximumOutputs: 1}}, Fingerprint: "acceptance-slot-quote-v1"}, nil
}

func (e *recordingAcceptanceExecutor) GenerateQuotedSlot(ctx context.Context, input imageagent.SlotExecutionInput, _ imageagent.SlotUsageQuote) (imageagent.SlotGeneratedOutput, error) {
	generated, err := e.GenerateSlot(ctx, input)
	if err == nil {
		generated.UsageReceipt = imageagent.SlotUsageReceipt{Actual: imageagent.UsageVector{Images: int64(len(generated.Assets)), AgentSteps: 1}, CostBasis: imageagent.UsageCostReservedUpperBound}
	}
	return generated, err
}

func (e *recordingAcceptanceExecutor) ExecuteSlot(ctx context.Context, input imageagent.SlotExecutionInput) (imageagent.SlotExecutionResult, error) {
	generated, err := e.GenerateSlot(ctx, input)
	if err != nil {
		return imageagent.SlotExecutionResult{}, err
	}
	return e.PublishSlot(ctx, input, generated)
}

func (e *recordingAcceptanceExecutor) GenerateSlot(ctx context.Context, input imageagent.SlotExecutionInput) (imageagent.SlotGeneratedOutput, error) {
	result, err := e.delegate.GenerateSlot(ctx, input)
	e.mu.Lock()
	e.calls = append(e.calls, input.Slot.ID)
	e.mu.Unlock()
	if err == nil && input.Slot.ID == e.invalidSlotID {
		return imageagent.SlotGeneratedOutput{}, fmt.Errorf("intentional acceptance provider failure for %s", input.Slot.ID)
	}
	return result, err
}

func (e *recordingAcceptanceExecutor) PublishSlot(ctx context.Context, input imageagent.SlotExecutionInput, generated imageagent.SlotGeneratedOutput) (imageagent.SlotExecutionResult, error) {
	result, err := e.delegate.PublishSlot(ctx, input, generated)
	if err == nil && input.Slot.ID == e.invalidSlotID {
		return imageagent.SlotExecutionResult{SlotID: input.Slot.ID, Attempt: input.Attempt}, nil
	}
	return result, err
}

func (e *recordingAcceptanceExecutor) BuildSlotResult(ctx context.Context, input imageagent.SlotExecutionInput, published imageagent.PublishedSlotOutput) (imageagent.SlotExecutionResult, error) {
	return e.delegate.BuildSlotResult(ctx, input, published)
}

func (e *recordingAcceptanceExecutor) calledIDs() []string {
	e.mu.Lock()
	defer e.mu.Unlock()
	return append([]string(nil), e.calls...)
}

type acceptanceSubjectExtractor struct{}

func (acceptanceSubjectExtractor) Extract(_ context.Context, imageURL string, _ *productimage.ProductContext) (*productimage.ImageAsset, error) {
	return &productimage.ImageAsset{
		URL: imageURL + "?subject=1", SourceURL: imageURL,
		Type: productimage.AssetTypeSubjectCutout,
	}, nil
}

type acceptanceWhiteRenderer struct{ artifactPath string }

func (r acceptanceWhiteRenderer) Render(_ context.Context, _ *productimage.ImageAsset, context *productimage.ProductContext) (*productimage.ImageAsset, error) {
	return &productimage.ImageAsset{
		URL:  r.artifactPath,
		Type: productimage.AssetTypeWhiteBgImage,
	}, nil
}

type acceptanceSceneRenderer struct{ artifactPath string }

func (r acceptanceSceneRenderer) Render(_ context.Context, _ *productimage.ImageAsset, context *productimage.ProductContext) ([]productimage.ImageAsset, error) {
	return []productimage.ImageAsset{{
		URL:  r.artifactPath,
		Type: productimage.AssetTypeGalleryImage,
	}}, nil
}

type acceptancePublisher struct{}

func (acceptancePublisher) PublishApproved(context.Context, imageagent.PublishApprovedInput) (imageagent.PublicationAcknowledgement, error) {
	return imageagent.PublicationAcknowledgement{}, nil
}

func (acceptancePublisher) PublishApprovedV3(context.Context, imageagent.PublishApprovedV3Input) (imageagent.PublicationAcknowledgement, error) {
	return imageagent.PublicationAcknowledgement{}, nil
}

type recordingAcceptanceApprovalPublisher struct {
	input imageagent.PublishApprovedV3Input
}

func (p *recordingAcceptanceApprovalPublisher) PublishApprovedV3(_ context.Context, input imageagent.PublishApprovedV3Input) (imageagent.PublicationAcknowledgement, error) {
	p.input = input
	return imageagent.PublicationAcknowledgement{IdempotencyKey: input.IdempotencyKey}, nil
}

type podLossAcceptanceS3Object struct {
	data        []byte
	contentType string
	metadata    map[string]string
	checksum    string
}

type podLossAcceptanceS3 struct {
	mu      sync.Mutex
	objects map[string]podLossAcceptanceS3Object
}

func (s *podLossAcceptanceS3) PutObject(_ context.Context, input *s3.PutObjectInput, _ ...func(*s3.Options)) (*s3.PutObjectOutput, error) {
	data, err := io.ReadAll(input.Body)
	if err != nil {
		return nil, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	key := aws.ToString(input.Key)
	if _, exists := s.objects[key]; exists {
		return nil, objectstore.ErrObjectConflict
	}
	s.objects[key] = podLossAcceptanceS3Object{
		data: append([]byte(nil), data...), contentType: aws.ToString(input.ContentType),
		metadata: cloneAcceptanceMetadata(input.Metadata), checksum: aws.ToString(input.ChecksumSHA256),
	}
	return &s3.PutObjectOutput{}, nil
}

func (s *podLossAcceptanceS3) HeadObject(_ context.Context, input *s3.HeadObjectInput, _ ...func(*s3.Options)) (*s3.HeadObjectOutput, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	object, exists := s.objects[aws.ToString(input.Key)]
	if !exists {
		return nil, &types.NotFound{}
	}
	return &s3.HeadObjectOutput{
		ContentLength: aws.Int64(int64(len(object.data))), ContentType: aws.String(object.contentType),
		Metadata: cloneAcceptanceMetadata(object.metadata), ChecksumSHA256: aws.String(object.checksum),
	}, nil
}

func (s *podLossAcceptanceS3) GetObject(_ context.Context, input *s3.GetObjectInput, _ ...func(*s3.Options)) (*s3.GetObjectOutput, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	object, exists := s.objects[aws.ToString(input.Key)]
	if !exists {
		return nil, &types.NoSuchKey{}
	}
	return &s3.GetObjectOutput{
		Body: io.NopCloser(strings.NewReader(string(object.data))), ContentLength: aws.Int64(int64(len(object.data))),
		ContentType: aws.String(object.contentType), Metadata: cloneAcceptanceMetadata(object.metadata), ChecksumSHA256: aws.String(object.checksum),
	}, nil
}

func (s *podLossAcceptanceS3) CopyObject(_ context.Context, input *s3.CopyObjectInput, _ ...func(*s3.Options)) (*s3.CopyObjectOutput, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	sourceKey := strings.TrimPrefix(aws.ToString(input.CopySource), "acceptance-assets/")
	source, exists := s.objects[sourceKey]
	if !exists {
		return nil, &types.NotFound{}
	}
	destinationKey := aws.ToString(input.Key)
	if _, exists := s.objects[destinationKey]; !exists {
		source.contentType = aws.ToString(input.ContentType)
		source.metadata = cloneAcceptanceMetadata(input.Metadata)
		s.objects[destinationKey] = source
	}
	return &s3.CopyObjectOutput{}, nil
}

func (*podLossAcceptanceS3) DeleteObject(context.Context, *s3.DeleteObjectInput, ...func(*s3.Options)) (*s3.DeleteObjectOutput, error) {
	return nil, errors.New("acceptance does not delete durable recovery objects")
}

func (s *podLossAcceptanceS3) countPrefix(prefix string) int {
	s.mu.Lock()
	defer s.mu.Unlock()
	count := 0
	for key := range s.objects {
		if strings.HasPrefix(key, prefix) {
			count++
		}
	}
	return count
}

func cloneAcceptanceMetadata(metadata map[string]string) map[string]string {
	cloned := make(map[string]string, len(metadata))
	for key, value := range metadata {
		cloned[key] = value
	}
	return cloned
}

var acceptancePNG, _ = base64.StdEncoding.DecodeString("iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mNk+A8AAQUBAScY42YAAAAASUVORK5CYII=")

type acceptanceDurableArtifactStore struct{}

func (acceptanceDurableArtifactStore) PublicURL(key string) string {
	return "https://cdn.example.test/" + key
}

func (acceptanceDurableArtifactStore) PrepareSlotArtifacts(input objectstore.PrepareSlotArtifactsInput) (objectstore.PreparedSlotArtifacts, error) {
	assets := make([]imageagent.StagedAssetRef, len(input.Assets))
	ownerKey, err := imageagent.ArtifactOwnerKey(input.Identity.OwnerUserID)
	if err != nil {
		return objectstore.PreparedSlotArtifacts{}, err
	}
	for index, asset := range input.Assets {
		operations, err := imageagent.NormalizeArtifactOperations(asset.Operations)
		if err != nil {
			return objectstore.PreparedSlotArtifacts{}, err
		}
		sum := sha256.Sum256(asset.Bytes)
		hash := hex.EncodeToString(sum[:])
		assets[index] = imageagent.StagedAssetRef{
			ObjectKey: fmt.Sprintf("image-agent/staging/%s/%s/%s/%d/%s/%d/%d-%s.png", input.Identity.TenantID, ownerKey, input.Identity.RunID, input.Identity.PlanRevision, input.Identity.SlotID, input.Identity.Attempt, index, hash),
			SHA256:    hash, SizeBytes: int64(len(asset.Bytes)), ContentType: asset.ContentType, Width: asset.Width, Height: asset.Height,
			SourceAssetID: asset.SourceAssetID, Operations: operations, ProviderReceiptID: asset.ProviderReceiptID,
		}
	}
	manifest, err := imageagent.NormalizeStagingManifest(imageagent.StagingManifest{Assets: assets})
	if err != nil {
		return objectstore.PreparedSlotArtifacts{}, err
	}
	return objectstore.PreparedSlotArtifacts{Manifest: manifest}, nil
}

func (acceptanceDurableArtifactStore) PreserveSlotArtifacts(context.Context, imageagent.SlotExternalEffectIdentity, objectstore.PreparedSlotArtifacts) error {
	return nil
}

func (acceptanceDurableArtifactStore) RecoverSlotArtifacts(_ context.Context, _ imageagent.SlotExternalEffectIdentity, expected imageagent.StagingManifest) (objectstore.PreparedSlotArtifacts, error) {
	if len(expected.Assets) == 0 {
		return objectstore.PreparedSlotArtifacts{}, objectstore.ErrArtifactUnavailable
	}
	return objectstore.PreparedSlotArtifacts{Manifest: expected}, nil
}

func (acceptanceDurableArtifactStore) EnsureStaged(_ context.Context, prepared objectstore.PreparedSlotArtifacts) error {
	return imageagent.ValidateStagingManifest(prepared.Manifest)
}

func (acceptanceDurableArtifactStore) Finalize(ctx context.Context, manifest imageagent.StagingManifest) (imageagent.FinalManifest, error) {
	return acceptanceDurableArtifactStore{}.FinalizeWithProgress(ctx, manifest, nil)
}

func (acceptanceDurableArtifactStore) FinalizeWithProgress(ctx context.Context, manifest imageagent.StagingManifest, progress func(context.Context, int) error) (imageagent.FinalManifest, error) {
	manifest, err := imageagent.NormalizeStagingManifest(manifest)
	if err != nil {
		return imageagent.FinalManifest{}, err
	}
	assets := make([]imageagent.PublishedAssetRef, len(manifest.Assets))
	for index, asset := range manifest.Assets {
		if progress != nil {
			if err := progress(ctx, index); err != nil {
				return imageagent.FinalManifest{}, err
			}
		}
		assets[index] = imageagent.PublishedAssetRef{
			ObjectKey: "image-agent/public/" + asset.ObjectKey[len("image-agent/staging/"):],
			SHA256:    asset.SHA256, SizeBytes: asset.SizeBytes, ContentType: asset.ContentType, Width: asset.Width, Height: asset.Height,
			SourceAssetID: asset.SourceAssetID, Operations: asset.Operations, ProviderReceiptID: asset.ProviderReceiptID,
		}
	}
	return imageagent.NormalizeFinalManifest(imageagent.FinalManifest{Assets: assets})
}

func acceptedSlotEventCount(t *testing.T, events []imageagent.RunEvent) int {
	t.Helper()
	count := 0
	for _, event := range events {
		if event.Type != "slot.result.persisted" {
			continue
		}
		var payload struct {
			Status imageagent.SlotStatus `json:"status"`
		}
		require.NoError(t, json.Unmarshal(event.Payload, &payload))
		if payload.Status == imageagent.SlotStatusAccepted {
			count++
		}
	}
	return count
}

func sortedSlotIDs(plan imageagent.Plan) []string {
	ids := make([]string, len(plan.Slots))
	for index, slot := range plan.Slots {
		ids[index] = slot.ID
	}
	return sortedStrings(ids)
}

func sortedStrings(values []string) []string {
	result := append([]string(nil), values...)
	sort.Strings(result)
	return result
}
