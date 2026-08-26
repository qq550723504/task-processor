package imageagent_test

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.temporal.io/sdk/testsuite"
	"go.temporal.io/sdk/worker"

	"task-processor/internal/imageagent"
	imageagentstore "task-processor/internal/imageagent/store"
	imageagenttemporal "task-processor/internal/imageagent/temporal"
	imageagenttools "task-processor/internal/imageagent/tools"
	"task-processor/internal/productimage"
)

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
	require.NoError(t, repository.CreateRun(context.Background(), run))
	scope := imageagent.RunScope{TenantID: run.TenantID, RunID: run.ID}
	require.NoError(t, repository.AppendPlan(context.Background(), scope, 0, plan))

	delegate := imageagenttools.NewProductImageSlotExecutor(imageagenttools.Dependencies{
		SubjectExtractor:        acceptanceSubjectExtractor{},
		WhiteBackgroundRenderer: acceptanceWhiteRenderer{},
		SceneRenderer:           acceptanceSceneRenderer{},
		SourceAssets:            acceptanceSourceAssets(9),
		AuthorizedStyleReferenceIDs: map[string]struct{}{
			"style-modern": {},
		},
	})
	executor := &recordingAcceptanceExecutor{delegate: delegate, invalidSlotID: invalidSlotID}
	activities, err := imageagenttemporal.NewActivities(imageagenttemporal.ActivityDependencies{
		Repository: repository, SlotExecutor: executor, Publisher: acceptancePublisher{},
	})
	require.NoError(t, err)

	var suite testsuite.WorkflowTestSuite
	env := suite.NewTestWorkflowEnvironment()
	env.SetWorkerOptions(worker.Options{DeadlockDetectionTimeout: 5 * time.Second})
	env.RegisterWorkflow(imageagenttemporal.ImageSlotWorkflow)
	require.NoError(t, imageagenttemporal.RegisterActivities(env, activities))
	env.ExecuteWorkflow(imageagenttemporal.ImageAgentWorkflow, imageagenttemporal.WorkflowInput{
		RunID: run.ID, Mode: imageagent.RunModeManual,
		Identity: imageagent.ExecutionIdentity{TenantID: run.TenantID, UserID: run.UserID},
		Plan:     plan, MaxConcurrentSlots: 4, WaitForCommands: false,
	})
	require.True(t, env.IsWorkflowCompleted())
	require.NoError(t, env.GetWorkflowError())
	var result imageagenttemporal.WorkflowResult
	require.NoError(t, env.GetWorkflowResult(&result))
	events, err := repository.ListEvents(context.Background(), scope, 0, 100)
	require.NoError(t, err)
	return result, executor.calledIDs(), events
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

func acceptanceSourceAssets(count int) map[string]productimage.ImageAsset {
	assets := make(map[string]productimage.ImageAsset, count)
	for index := 1; index <= count; index++ {
		id := fmt.Sprintf("source-%d", index)
		url := fmt.Sprintf("https://source.example/%s.png", id)
		assets[id] = productimage.ImageAsset{
			URL: url, SourceURL: url, Type: productimage.AssetTypeSourceImage,
			Width: 1200, Height: 1200,
		}
	}
	return assets
}

type recordingAcceptanceExecutor struct {
	delegate      imageagent.SlotExecutor
	invalidSlotID string
	mu            sync.Mutex
	calls         []string
}

func (e *recordingAcceptanceExecutor) ExecuteSlot(ctx context.Context, input imageagent.SlotExecutionInput) (imageagent.SlotExecutionResult, error) {
	result, err := e.delegate.ExecuteSlot(ctx, input)
	e.mu.Lock()
	e.calls = append(e.calls, input.Slot.ID)
	e.mu.Unlock()
	if err == nil && input.Slot.ID == e.invalidSlotID {
		return imageagent.SlotExecutionResult{SlotID: input.Slot.ID, Attempt: input.Attempt}, nil
	}
	return result, err
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

type acceptanceWhiteRenderer struct{}

func (acceptanceWhiteRenderer) Render(_ context.Context, _ *productimage.ImageAsset, context *productimage.ProductContext) (*productimage.ImageAsset, error) {
	return &productimage.ImageAsset{
		URL:  "https://generated.example/" + context.Attributes["slot_brief"] + ".png",
		Type: productimage.AssetTypeWhiteBgImage,
	}, nil
}

type acceptanceSceneRenderer struct{}

func (acceptanceSceneRenderer) Render(_ context.Context, _ *productimage.ImageAsset, context *productimage.ProductContext) ([]productimage.ImageAsset, error) {
	return []productimage.ImageAsset{{
		URL:  "https://generated.example/" + context.Attributes["slot_brief"] + ".png",
		Type: productimage.AssetTypeGalleryImage,
	}}, nil
}

type acceptancePublisher struct{}

func (acceptancePublisher) PublishApproved(context.Context, imageagent.PublishApprovedInput) error {
	return nil
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
