package tools

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"

	"task-processor/internal/imageagent"
	productimage "task-processor/internal/productimage"
)

func TestExecutorCallsSceneRendererOncePerSceneSlot(t *testing.T) {
	renderer := &recordingSceneRenderer{result: []productimage.ImageAsset{{URL: "asset://scene-1"}}}
	executor := NewProductImageSlotExecutor(Dependencies{SceneRenderer: renderer, SubjectExtractor: stubSubjectExtractor()})
	for _, id := range []string{"scene-1", "scene-2", "scene-3", "scene-4"} {
		_, err := executor.ExecuteSlot(context.Background(), sceneSlotInput(id))
		require.NoError(t, err)
	}
	require.Equal(t, 4, renderer.calls)
}

func TestExecutorUsesSubjectExtractionAndWhiteBackgroundForMainSlot(t *testing.T) {
	extractor := &recordingSubjectExtractor{result: &productimage.ImageAsset{URL: "asset://subject"}}
	whiteBackground := &recordingWhiteBackgroundRenderer{result: &productimage.ImageAsset{URL: "asset://main"}}
	executor := NewProductImageSlotExecutor(Dependencies{SubjectExtractor: extractor, WhiteBackgroundRenderer: whiteBackground})

	result, err := executor.ExecuteSlot(context.Background(), slotInput(" main-1 ", imageagent.SlotRoleMain))

	require.NoError(t, err)
	require.Equal(t, 1, extractor.calls)
	require.Equal(t, 1, whiteBackground.calls)
	require.Equal(t, "main-1", result.SlotID)
	require.Equal(t, 1, result.Attempt)
	require.Equal(t, []imageagent.AssetCandidate{{
		AssetID: "main-1-candidate-1", URL: "asset://main", SourceAssetID: "source-1",
	}}, result.Candidates)
}

func TestExecutorMapsSceneRoleBriefAndAuthorizedStyleReferencesToSceneContext(t *testing.T) {
	renderer := &recordingSceneRenderer{result: []productimage.ImageAsset{{URL: "asset://scene"}}}
	executor := NewProductImageSlotExecutor(Dependencies{
		SceneRenderer:               renderer,
		ProductContext:              &productimage.ProductContext{Title: "Example product"},
		AuthorizedStyleReferenceIDs: map[string]struct{}{"style-1": {}},
	})
	input := sceneSlotInput("detail-1")
	input.Slot.Role = imageagent.SlotRoleDetail
	input.Slot.Brief = "  show material texture  "
	input.Slot.StyleReferenceIDs = []string{" style-1 ", "unapproved-style"}

	_, err := executor.ExecuteSlot(context.Background(), input)

	require.NoError(t, err)
	require.NotNil(t, renderer.context)
	require.Equal(t, "detail", renderer.context.Attributes["slot_role"])
	require.Equal(t, "show material texture", renderer.context.Attributes["slot_brief"])
	require.Equal(t, "style-1", renderer.context.Attributes["style_reference_ids"])
}

func TestExecutorKeepsProviderOutputsAsCandidatesForDeclaredSlot(t *testing.T) {
	assets := make([]productimage.ImageAsset, 11)
	for index := range assets {
		assets[index] = productimage.ImageAsset{URL: "asset://candidate-" + string(rune('a'+index))}
	}
	renderer := &recordingSceneRenderer{result: assets}
	executor := NewProductImageSlotExecutor(Dependencies{SceneRenderer: renderer})

	result, err := executor.ExecuteSlot(context.Background(), sceneSlotInput("scene-1"))

	require.NoError(t, err)
	require.Equal(t, "scene-1", result.SlotID)
	require.Len(t, result.Candidates, 11)
	require.Equal(t, "scene-1-candidate-1", result.Candidates[0].AssetID)
	require.Equal(t, "scene-1-candidate-11", result.Candidates[10].AssetID)
	for _, candidate := range result.Candidates {
		require.Equal(t, "source-1", candidate.SourceAssetID)
	}
}

func TestExecutorFailsClosedForProviderFailureAndNonGeneratedOutput(t *testing.T) {
	tests := []struct {
		name     string
		renderer *recordingSceneRenderer
	}{
		{name: "provider error", renderer: &recordingSceneRenderer{err: errors.New("provider unavailable")}},
		{name: "empty output", renderer: &recordingSceneRenderer{}},
		{name: "source asset output", renderer: &recordingSceneRenderer{result: []productimage.ImageAsset{{URL: "https://example.test/source.jpg"}}}},
		{name: "local canvas output", renderer: &recordingSceneRenderer{result: []productimage.ImageAsset{{URL: "asset://canvas", Metadata: map[string]string{"scene_mode": "local_canvas"}}}}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			executor := NewProductImageSlotExecutor(Dependencies{
				SceneRenderer: tt.renderer,
				SourceAssets: map[string]productimage.ImageAsset{
					"source-1": {URL: "https://example.test/source.jpg", SourceURL: "https://example.test/source.jpg"},
				},
			})
			result, err := executor.ExecuteSlot(context.Background(), sceneSlotInput("scene-1"))
			require.Error(t, err)
			require.Empty(t, result.Candidates)
		})
	}
}

func TestExecutorSizeSlotRequiresReliableDimensions(t *testing.T) {
	renderer := &recordingSceneRenderer{result: []productimage.ImageAsset{{URL: "asset://size"}}}
	executor := NewProductImageSlotExecutor(Dependencies{SceneRenderer: renderer})
	input := slotInput("size-1", imageagent.SlotRoleSize)

	_, err := executor.ExecuteSlot(context.Background(), input)
	require.ErrorContains(t, err, "reliable dimensions")
	require.Zero(t, renderer.calls)

	executor = NewProductImageSlotExecutor(Dependencies{
		SceneRenderer: renderer,
		SourceAssets: map[string]productimage.ImageAsset{
			"source-1": {URL: "https://example.test/source.jpg", SourceURL: "https://example.test/source.jpg", Width: 1200, Height: 900},
		},
	})
	result, err := executor.ExecuteSlot(context.Background(), input)
	require.NoError(t, err)
	require.Len(t, result.Candidates, 1)
	require.Equal(t, 1, renderer.calls)
}

func sceneSlotInput(id string) imageagent.SlotExecutionInput {
	return slotInput(id, imageagent.SlotRoleScene)
}

func slotInput(id string, role imageagent.SlotRole) imageagent.SlotExecutionInput {
	return imageagent.SlotExecutionInput{
		RunID: "run-1", TenantID: "tenant-1", UserID: "user-1", PlanRevision: 1, Attempt: 1, IdempotencyKey: "attempt-1",
		Slot: imageagent.Slot{ID: id, Role: role, SourceAssetIDs: []string{"source-1"}, IdempotencyKey: "slot-1"},
	}
}

type recordingSceneRenderer struct {
	calls   int
	result  []productimage.ImageAsset
	err     error
	asset   *productimage.ImageAsset
	context *productimage.ProductContext
}

func (r *recordingSceneRenderer) Render(_ context.Context, asset *productimage.ImageAsset, productContext *productimage.ProductContext) ([]productimage.ImageAsset, error) {
	r.calls++
	r.asset = asset
	r.context = productContext
	return r.result, r.err
}

type recordingSubjectExtractor struct {
	calls  int
	result *productimage.ImageAsset
}

func (r *recordingSubjectExtractor) Extract(_ context.Context, _ string, _ *productimage.ProductContext) (*productimage.ImageAsset, error) {
	r.calls++
	return r.result, nil
}

type recordingWhiteBackgroundRenderer struct {
	calls  int
	result *productimage.ImageAsset
}

func (r *recordingWhiteBackgroundRenderer) Render(_ context.Context, _ *productimage.ImageAsset, _ *productimage.ProductContext) (*productimage.ImageAsset, error) {
	r.calls++
	return r.result, nil
}

func stubSubjectExtractor() productimage.SubjectExtractor {
	return &recordingSubjectExtractor{result: &productimage.ImageAsset{URL: "asset://subject"}}
}

func candidateIDs(candidates []imageagent.AssetCandidate) []string {
	ids := make([]string, 0, len(candidates))
	for _, candidate := range candidates {
		ids = append(ids, candidate.AssetID)
	}
	return ids
}
