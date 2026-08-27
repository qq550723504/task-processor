package tools

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"

	"task-processor/internal/imageagent"
	productimage "task-processor/internal/productimage"
)

func TestProductImageV3GenerationReturnsTransientMaterialOnly(t *testing.T) {
	publisher := &recordingAssetPublisher{}
	executor := NewProductImageSlotExecutor(Dependencies{
		SceneRenderer: &recordingSceneRenderer{result: []productimage.ImageAsset{{
			URL:        `C:\\worker\\generated.png`,
			Operations: []string{"render_scene_model"},
			Metadata:   map[string]string{"local_path": `C:\\worker\\generated.png`, "authorization": "secret"},
		}}},
		AssetPublisher: publisher,
	})

	generated, err := executor.GenerateSlot(context.Background(), sceneSlotInput("scene-1"))

	require.NoError(t, err)
	require.Zero(t, publisher.calls, "v3 generation must not publish or construct candidates")
	require.Equal(t, `C:\\worker\\generated.png`, generated.Assets[0].URL)
	require.Equal(t, map[string]string{"local_path": `C:\\worker\\generated.png`, "authorization": "secret"}, generated.Assets[0].Metadata)
}

func TestProductImageV3BuildResultRequiresDurablePublishedReferences(t *testing.T) {
	executor := NewProductImageSlotExecutor(Dependencies{})
	input := sceneSlotInput("scene-1")

	_, err := executor.BuildSlotResult(context.Background(), input, imageagent.PublishedSlotOutput{
		SlotID:  input.Slot.ID,
		Attempt: input.Attempt,
		Assets: []imageagent.PublishedAssetRef{{
			ObjectKey:     `C:\\worker\\generated.png`,
			SHA256:        strings.Repeat("a", 64),
			SizeBytes:     1,
			ContentType:   "image/png",
			Width:         1,
			Height:        1,
			SourceAssetID: "source-1",
		}},
	})

	require.ErrorIs(t, err, imageagent.ErrValidation)
}

func TestProductImageV3BuildResultUsesMetadataAllowlist(t *testing.T) {
	executor := NewProductImageSlotExecutor(Dependencies{})
	input := sceneSlotInput("scene-1")
	published := durablePublishedOutput(input, []imageagent.PublishedAssetRef{{
		ObjectKey:     "image-agent/public/tenant-1/run-1/1/scene-1/1/0-aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa.png",
		SHA256:        strings.Repeat("a", 64),
		SizeBytes:     42,
		ContentType:   "image/png",
		Width:         1200,
		Height:        1200,
		SourceAssetID: "source-1",
		Operations:    []string{"extract_subject", "render_scene_model"},
	}})

	result, err := executor.BuildSlotResult(context.Background(), input, published)

	require.NoError(t, err)
	require.Len(t, result.Candidates, 1)
	candidate := result.Candidates[0]
	require.Empty(t, candidate.URL)
	require.Nil(t, candidate.Metadata)
	require.Equal(t, "source-1", candidate.SourceAssetID)
	require.Equal(t, imageagent.DurableAssetIdentity{ObjectKey: published.Assets[0].ObjectKey, SHA256: published.Assets[0].SHA256}, candidate.DurableAsset)

	again, err := executor.BuildSlotResult(context.Background(), input, published)
	require.NoError(t, err)
	require.Equal(t, candidate.AssetID, again.Candidates[0].AssetID)

	published.Assets[0].SHA256 = strings.Repeat("b", 64)
	published.Assets[0].ObjectKey = "image-agent/public/tenant-1/run-1/1/scene-1/1/0-bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb.png"
	changed, err := executor.BuildSlotResult(context.Background(), input, published)
	require.NoError(t, err)
	require.NotEqual(t, candidate.AssetID, changed.Candidates[0].AssetID)
}

func TestProductImageV3MainSlotRequiresExactlyOneCandidate(t *testing.T) {
	executor := NewProductImageSlotExecutor(Dependencies{})
	input := slotInput("main-1", imageagent.SlotRoleMain)
	published := durablePublishedOutput(input, []imageagent.PublishedAssetRef{
		{ObjectKey: "image-agent/public/tenant-1/run-1/1/main-1/1/0-aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa.png", SHA256: strings.Repeat("a", 64), SizeBytes: 1, ContentType: "image/png", Width: 1, Height: 1, SourceAssetID: "source-1"},
		{ObjectKey: "image-agent/public/tenant-1/run-1/1/main-1/1/1-bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb.png", SHA256: strings.Repeat("b", 64), SizeBytes: 1, ContentType: "image/png", Width: 1, Height: 1, SourceAssetID: "source-1"},
	})

	_, err := executor.BuildSlotResult(context.Background(), input, published)

	require.ErrorIs(t, err, imageagent.ErrValidation)
}

func TestProductImageLegacyExecuteSlotRetainsV2Shape(t *testing.T) {
	publisher := &recordingAssetPublisher{}
	executor := NewProductImageSlotExecutor(Dependencies{
		SceneRenderer:  &recordingSceneRenderer{result: []productimage.ImageAsset{{URL: `C:\\worker\\generated.png`, Metadata: map[string]string{"local_path": `C:\\worker\\generated.png`}}}},
		AssetPublisher: publisher,
	})

	result, err := executor.ExecuteSlot(context.Background(), sceneSlotInput("scene-1"))

	require.NoError(t, err)
	require.Equal(t, 1, publisher.calls)
	encoded, err := json.Marshal(result)
	require.NoError(t, err)
	require.JSONEq(t, `{"SlotID":"scene-1","Attempt":1,"Candidates":[{"AssetID":"imageagent-candidate-3dffce1d6079689a244e904da7f05eb5a66c17f11f216c1dd81f380cecb4162a","URL":"https://cdn.example.test/generated.png","SourceAssetID":"source-1","Metadata":{"local_path":"C:\\\\worker\\\\generated.png"}}]}`, string(encoded))
}

func TestProductImageV3BuildResultRejectsNonPublicAndMismatchedPublishedKeys(t *testing.T) {
	executor := NewProductImageSlotExecutor(Dependencies{})
	input := sceneSlotInput("scene-1")
	hash := strings.Repeat("a", 64)
	for _, objectKey := range []string{
		"image-agent/staging/tenant-1/run-1/1/scene-1/1/0-" + hash + ".png",
		"private/provider-output/0-" + hash + ".png",
		"image-agent/public/tenant-2/run-1/1/scene-1/1/0-" + hash + ".png",
		"image-agent/public/tenant-1/run-1/01/scene-1/1/0-" + hash + ".png",
		"image-agent/public/tenant-1/run-1/1/scene-1/1/1-" + hash + ".png",
		"image-agent/public/tenant-1/run-1/1/scene-1/1/0-" + strings.Repeat("b", 64) + ".png",
		"image-agent/public/tenant-1/run-1/1/scene-1/1/0-" + hash + ".jpg",
	} {
		t.Run(objectKey, func(t *testing.T) {
			_, err := executor.BuildSlotResult(context.Background(), input, durablePublishedOutput(input, []imageagent.PublishedAssetRef{{
				ObjectKey: objectKey, SHA256: hash, SizeBytes: 1, ContentType: "image/png", Width: 1, Height: 1, SourceAssetID: "source-1",
			}}))
			require.ErrorIs(t, err, imageagent.ErrValidation)
		})
	}
}

func TestProductImageV3BuildResultRequiresNormalizedExecutionIdentity(t *testing.T) {
	executor := NewProductImageSlotExecutor(Dependencies{})
	for _, mutate := range []func(*imageagent.SlotExecutionInput){
		func(input *imageagent.SlotExecutionInput) { input.TenantID = " \t" },
		func(input *imageagent.SlotExecutionInput) { input.UserID = " \t" },
	} {
		input := sceneSlotInput("scene-1")
		mutate(&input)
		_, err := executor.BuildSlotResult(context.Background(), input, durablePublishedOutput(input, []imageagent.PublishedAssetRef{publicPublishedAsset()}))
		require.ErrorIs(t, err, imageagent.ErrValidation)
	}
}

func TestProductImageV3BuildResultAcceptsOpaqueAuthenticatedUserID(t *testing.T) {
	executor := NewProductImageSlotExecutor(Dependencies{})
	input := sceneSlotInput("scene-1")
	input.UserID = "auth0|opaque/user:1"

	result, err := executor.BuildSlotResult(context.Background(), input, durablePublishedOutput(input, []imageagent.PublishedAssetRef{publicPublishedAsset()}))

	require.NoError(t, err)
	require.Len(t, result.Candidates, 1)
}

func TestProductImageV3CandidateIdentityBindsEveryAttemptDomain(t *testing.T) {
	input := sceneSlotInput("scene-1")
	asset := publicPublishedAsset()
	baseline := durableCandidateAssetID(input, input.Slot, asset, 0)
	require.Equal(t, baseline, durableCandidateAssetID(input, input.Slot, asset, 0), "exact replay must be stable")

	for _, test := range []struct {
		name   string
		mutate func(*imageagent.SlotExecutionInput, *imageagent.PublishedAssetRef, *int)
	}{
		{name: "tenant", mutate: func(input *imageagent.SlotExecutionInput, _ *imageagent.PublishedAssetRef, _ *int) {
			input.TenantID = "tenant-2"
		}},
		{name: "opaque user", mutate: func(input *imageagent.SlotExecutionInput, _ *imageagent.PublishedAssetRef, _ *int) {
			input.UserID = "auth0|opaque/user:2"
		}},
		{name: "run", mutate: func(input *imageagent.SlotExecutionInput, _ *imageagent.PublishedAssetRef, _ *int) {
			input.RunID = "run-2"
		}},
		{name: "plan", mutate: func(input *imageagent.SlotExecutionInput, _ *imageagent.PublishedAssetRef, _ *int) {
			input.PlanRevision = 2
		}},
		{name: "slot", mutate: func(input *imageagent.SlotExecutionInput, _ *imageagent.PublishedAssetRef, _ *int) {
			input.Slot.ID = "scene-2"
		}},
		{name: "attempt", mutate: func(input *imageagent.SlotExecutionInput, _ *imageagent.PublishedAssetRef, _ *int) { input.Attempt = 2 }},
		{name: "index and final key", mutate: func(_ *imageagent.SlotExecutionInput, asset *imageagent.PublishedAssetRef, index *int) {
			*index = 1
			asset.ObjectKey = "image-agent/public/tenant-1/run-1/1/scene-1/1/1-aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa.png"
		}},
		{name: "final hash", mutate: func(_ *imageagent.SlotExecutionInput, asset *imageagent.PublishedAssetRef, _ *int) {
			asset.SHA256 = strings.Repeat("b", 64)
		}},
		{name: "source lineage", mutate: func(_ *imageagent.SlotExecutionInput, asset *imageagent.PublishedAssetRef, _ *int) {
			asset.SourceAssetID = "source-2"
		}},
	} {
		t.Run(test.name, func(t *testing.T) {
			candidateInput, candidateAsset, candidateIndex := input, asset, 0
			test.mutate(&candidateInput, &candidateAsset, &candidateIndex)
			require.NotEqual(t, baseline, durableCandidateAssetID(candidateInput, candidateInput.Slot, candidateAsset, candidateIndex))
		})
	}
}

func TestProductImageV3BuildResultPreservesPublishedOrder(t *testing.T) {
	executor := NewProductImageSlotExecutor(Dependencies{})
	input := sceneSlotInput("scene-1")
	first := publicPublishedAsset()
	second := first
	second.ObjectKey = "image-agent/public/tenant-1/run-1/1/scene-1/1/1-bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb.png"
	second.SHA256 = strings.Repeat("b", 64)

	result, err := executor.BuildSlotResult(context.Background(), input, durablePublishedOutput(input, []imageagent.PublishedAssetRef{first, second}))

	require.NoError(t, err)
	require.Equal(t, []string{first.ObjectKey, second.ObjectKey}, []string{result.Candidates[0].DurableAsset.ObjectKey, result.Candidates[1].DurableAsset.ObjectKey})
}

func TestProductImageV3BuildResultRejectsDuplicatePublishedReferences(t *testing.T) {
	executor := NewProductImageSlotExecutor(Dependencies{})
	input := sceneSlotInput("scene-1")
	asset := publicPublishedAsset()

	_, err := executor.BuildSlotResult(context.Background(), input, durablePublishedOutput(input, []imageagent.PublishedAssetRef{asset, asset}))

	require.ErrorIs(t, err, imageagent.ErrValidation)
}

func TestProductImageV3BuildResultRejectsZeroMainCandidate(t *testing.T) {
	executor := NewProductImageSlotExecutor(Dependencies{})
	input := slotInput("main-1", imageagent.SlotRoleMain)

	_, err := executor.BuildSlotResult(context.Background(), input, durablePublishedOutput(input, nil))

	require.ErrorIs(t, err, imageagent.ErrValidation)
}

func TestProductImageV3CandidateJSONExcludesTransientAndProviderSentinels(t *testing.T) {
	executor := NewProductImageSlotExecutor(Dependencies{})
	input := sceneSlotInput("scene-1")
	result, err := executor.BuildSlotResult(context.Background(), input, durablePublishedOutput(input, []imageagent.PublishedAssetRef{publicPublishedAsset()}))
	require.NoError(t, err)
	encoded, err := json.Marshal(result)
	require.NoError(t, err)
	for _, sentinel := range []string{"C:/worker", "file://", "http://", "https://", "authorization", "credential", "provider_receipt", "debug_prompt", "png bytes", "render_scene_model"} {
		require.NotContains(t, string(encoded), sentinel)
	}
}

func TestExecutorCallsSceneRendererOncePerSceneSlot(t *testing.T) {
	renderer := &recordingSceneRenderer{result: []productimage.ImageAsset{{URL: "https://generated.example/scene-1.jpg"}}}
	executor := NewProductImageSlotExecutor(Dependencies{SceneRenderer: renderer, SubjectExtractor: stubSubjectExtractor()})
	for _, id := range []string{"scene-1", "scene-2", "scene-3", "scene-4"} {
		_, err := executor.ExecuteSlot(context.Background(), sceneSlotInput(id))
		require.NoError(t, err)
	}
	require.Equal(t, 4, renderer.calls)
}

func TestExecutorRejectsSourceURLsAndReferencesOutsideWorkflowCatalog(t *testing.T) {
	executor := NewProductImageSlotExecutor(Dependencies{SceneRenderer: &recordingSceneRenderer{result: []productimage.ImageAsset{{URL: "https://generated.example/generated.jpg"}}}})
	input := sceneSlotInput("scene-1")
	input.Slot.SourceAssetIDs = []string{"https://attacker.example/source.png"}
	_, err := executor.ExecuteSlot(context.Background(), input)
	require.ErrorContains(t, err, "not authorized")
	input.Slot.SourceAssetIDs = []string{"source-not-authorized"}
	_, err = executor.ExecuteSlot(context.Background(), input)
	require.ErrorContains(t, err, "not authorized")
}

func TestExecutorResolvesAuthorizedSourceIDWhenCatalogDisplayURLIsOmitted(t *testing.T) {
	executor := NewProductImageSlotExecutor(Dependencies{
		SceneRenderer: &recordingSceneRenderer{result: []productimage.ImageAsset{{URL: "https://generated.example/generated.jpg"}}},
	})
	input := sceneSlotInput("scene-1")
	input.AssetCatalog.Assets[0].DisplayURL = ""
	input.AssetCatalog.Assets[0].URL = "https://example.test/source.jpg"
	input.AssetCatalog.Assets[0].SourceURL = "https://origin.example/source.jpg"
	result, err := executor.ExecuteSlot(context.Background(), input)
	require.NoError(t, err)
	require.Len(t, result.Candidates, 1)
}

func TestExecutorUsesCanonicalCatalogURLInsteadOfDisplayAlias(t *testing.T) {
	renderer := &recordingSceneRenderer{result: []productimage.ImageAsset{{URL: "https://example.test/generated.jpg"}}}
	executor := NewProductImageSlotExecutor(Dependencies{
		SceneRenderer: renderer,
	})
	input := sceneSlotInput("scene-1")
	input.AssetCatalog.Assets[0].URL = "https://trusted.example/source.jpg"
	input.AssetCatalog.Assets[0].SourceURL = "https://trusted.example/source.jpg"
	input.AssetCatalog.Assets[0].DisplayURL = "https://attacker.example/display-alias.jpg"

	_, err := executor.ExecuteSlot(context.Background(), input)

	require.NoError(t, err)
	require.NotNil(t, renderer.asset)
	require.Equal(t, "https://trusted.example/source.jpg", renderer.asset.URL)
}

func TestExecutorUsesSubjectExtractionAndWhiteBackgroundForMainSlot(t *testing.T) {
	extractor := &recordingSubjectExtractor{result: &productimage.ImageAsset{URL: "https://generated.example/subject.png"}}
	whiteBackground := &recordingWhiteBackgroundRenderer{result: &productimage.ImageAsset{URL: "https://generated.example/main.png"}}
	executor := NewProductImageSlotExecutor(Dependencies{SubjectExtractor: extractor, WhiteBackgroundRenderer: whiteBackground})

	result, err := executor.ExecuteSlot(context.Background(), slotInput(" main-1 ", imageagent.SlotRoleMain))

	require.NoError(t, err)
	require.Equal(t, 1, extractor.calls)
	require.Equal(t, 1, whiteBackground.calls)
	require.Equal(t, "main-1", result.SlotID)
	require.Equal(t, 1, result.Attempt)
	require.Len(t, result.Candidates, 1)
	require.NotEmpty(t, result.Candidates[0].AssetID)
	require.Equal(t, "https://generated.example/main.png", result.Candidates[0].URL)
	require.Equal(t, "source-1", result.Candidates[0].SourceAssetID)
}

func TestExecutorMapsSceneRoleBriefAndAuthorizedStyleReferencesToSceneContext(t *testing.T) {
	renderer := &recordingSceneRenderer{result: []productimage.ImageAsset{{URL: "https://generated.example/scene.jpg"}}}
	executor := NewProductImageSlotExecutor(Dependencies{
		SceneRenderer:  renderer,
		ProductContext: &productimage.ProductContext{Title: "Example product"},
	})
	input := sceneSlotInput("detail-1")
	input.Slot.Role = imageagent.SlotRoleDetail
	input.Slot.Brief = "  show material texture  "
	input.Slot.StyleReferenceIDs = []string{" style-1 ", "unapproved-style"}
	input.Slot.StyleReferenceIDs = []string{" style-1 "}
	input.AssetCatalog.Assets = append(input.AssetCatalog.Assets, imageagent.AuthorizedAsset{ID: "style-1", Type: imageagent.AuthorizedAssetStyle})

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
		assets[index] = productimage.ImageAsset{URL: "https://generated.example/candidate-" + string(rune('a'+index)) + ".jpg"}
	}
	renderer := &recordingSceneRenderer{result: assets}
	executor := NewProductImageSlotExecutor(Dependencies{SceneRenderer: renderer})

	result, err := executor.ExecuteSlot(context.Background(), sceneSlotInput("scene-1"))

	require.NoError(t, err)
	require.Equal(t, "scene-1", result.SlotID)
	require.Len(t, result.Candidates, 11)
	require.NotEmpty(t, result.Candidates[0].AssetID)
	require.NotEqual(t, result.Candidates[0].AssetID, result.Candidates[10].AssetID)
	for _, candidate := range result.Candidates {
		require.Equal(t, "source-1", candidate.SourceAssetID)
	}
}

func TestExecutorPublishesModelLocalOutputsBeforeCandidateProjection(t *testing.T) {
	publisher := &recordingAssetPublisher{}
	executor := NewProductImageSlotExecutor(Dependencies{
		SceneRenderer:  &recordingSceneRenderer{result: []productimage.ImageAsset{{URL: `C:\work\generated.png`, Metadata: map[string]string{"local_path": `C:\work\generated.png`}}}},
		AssetPublisher: publisher,
	})

	result, err := executor.ExecuteSlot(context.Background(), sceneSlotInput("scene-1"))

	require.NoError(t, err)
	require.Equal(t, 1, publisher.calls)
	require.Len(t, result.Candidates, 1)
	require.Equal(t, "https://cdn.example.test/generated.png", result.Candidates[0].URL)
}

func TestExecutorCandidateIdentityBindsAttemptAndOriginalProviderIndex(t *testing.T) {
	source := productimage.ImageAsset{URL: "https://example.test/source.jpg", SourceURL: "https://example.test/source.jpg"}
	generated := productimage.ImageAsset{URL: "https://example.test/generated.jpg"}
	input := sceneSlotInput("scene-1")
	executor := NewProductImageSlotExecutor(Dependencies{SceneRenderer: &recordingSceneRenderer{
		result: []productimage.ImageAsset{source, generated},
	}})
	input.AssetCatalog.Assets[0].URL = source.URL
	input.AssetCatalog.Assets[0].SourceURL = source.SourceURL

	first, err := executor.ExecuteSlot(context.Background(), input)
	require.NoError(t, err)
	require.Len(t, first.Candidates, 1)

	input.Attempt = 2
	second, err := executor.ExecuteSlot(context.Background(), input)
	require.NoError(t, err)
	require.Len(t, second.Candidates, 1)
	require.NotEqual(t, first.Candidates[0].AssetID, second.Candidates[0].AssetID)

	executor = NewProductImageSlotExecutor(Dependencies{SceneRenderer: &recordingSceneRenderer{
		result: []productimage.ImageAsset{source, source, generated},
	}})
	withTwoRejected, err := executor.ExecuteSlot(context.Background(), sceneSlotInput("scene-1"))
	require.NoError(t, err)
	require.Len(t, withTwoRejected.Candidates, 1)
	require.NotEqual(t, first.Candidates[0].AssetID, withTwoRejected.Candidates[0].AssetID)
}

func TestCandidateIdentityBindsStableExecutionIdentity(t *testing.T) {
	input := sceneSlotInput("scene-1")
	slot := input.Slot
	baseline := candidateAssetID(input, slot, 2)
	require.Equal(t, baseline, candidateAssetID(input, slot, 2))

	variants := []struct {
		name  string
		input imageagent.SlotExecutionInput
		slot  imageagent.Slot
		index int
	}{
		{name: "run", input: func() imageagent.SlotExecutionInput { cloned := input; cloned.RunID = "run-2"; return cloned }(), slot: slot, index: 2},
		{name: "plan revision", input: func() imageagent.SlotExecutionInput { cloned := input; cloned.PlanRevision = 2; return cloned }(), slot: slot, index: 2},
		{name: "slot id", input: input, slot: func() imageagent.Slot { cloned := slot; cloned.ID = "scene-2"; return cloned }(), index: 2},
		{name: "slot idempotency key", input: input, slot: func() imageagent.Slot { cloned := slot; cloned.IdempotencyKey = "slot-2"; return cloned }(), index: 2},
		{name: "input idempotency key", input: func() imageagent.SlotExecutionInput {
			cloned := input
			cloned.IdempotencyKey = "attempt-2"
			return cloned
		}(), slot: slot, index: 2},
		{name: "attempt", input: func() imageagent.SlotExecutionInput { cloned := input; cloned.Attempt = 2; return cloned }(), slot: slot, index: 2},
		{name: "provider output index", input: input, slot: slot, index: 3},
	}
	for _, tt := range variants {
		t.Run(tt.name, func(t *testing.T) {
			require.NotEqual(t, baseline, candidateAssetID(tt.input, tt.slot, tt.index))
		})
	}
}

func TestExecutorRejectsSourceEquivalentURLs(t *testing.T) {
	tests := []struct {
		name      string
		sourceURL string
		outputURL string
	}{
		{name: "local path", sourceURL: `C:\\work\\source.png`, outputURL: `C:\\work\\source.png`},
		{name: "asset URL", sourceURL: "asset://source-1/image", outputURL: "asset://source-1/image"},
		{name: "HTTP root slash", sourceURL: "https://example.test", outputURL: "https://example.test/"},
		{name: "HTTP query ordering and fragments", sourceURL: "https://example.test/source.jpg?b=2&a=1#source", outputURL: "HTTPS://EXAMPLE.TEST:443/source.jpg?a=1&b=2#generated"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			executor := NewProductImageSlotExecutor(Dependencies{SceneRenderer: &recordingSceneRenderer{result: []productimage.ImageAsset{{URL: tt.outputURL}}}})
			input := sceneSlotInput("scene-1")
			input.AssetCatalog.Assets[0].DisplayURL = tt.sourceURL
			input.AssetCatalog.Assets[0].URL = tt.sourceURL
			input.AssetCatalog.Assets[0].SourceURL = tt.sourceURL
			result, err := executor.ExecuteSlot(context.Background(), input)
			require.Error(t, err)
			require.Empty(t, result.Candidates)
		})
	}
}

func TestExecutorAcceptsUnrelatedMetadataDescriptions(t *testing.T) {
	executor := NewProductImageSlotExecutor(Dependencies{SceneRenderer: &recordingSceneRenderer{result: []productimage.ImageAsset{{
		URL: "https://example.test/generated.jpg", Type: productimage.AssetTypeGalleryImage,
		Metadata: map[string]string{"model_note": "the prompt says placeholder and pass_through only as prohibited examples"},
	}}}})

	result, err := executor.ExecuteSlot(context.Background(), sceneSlotInput("scene-1"))

	require.NoError(t, err)
	require.Len(t, result.Candidates, 1)
}

func TestExecutorRejectsDirectURLSourceIDsEvenWhenReadable(t *testing.T) {
	renderer := &recordingSceneRenderer{result: []productimage.ImageAsset{{URL: "https://example.test/generated.jpg"}}}
	executor := NewProductImageSlotExecutor(Dependencies{SceneRenderer: renderer})

	input := sceneSlotInput("scene-1")
	input.Slot.SourceAssetIDs = []string{"https://example.test/source.jpg"}
	result, err := executor.ExecuteSlot(context.Background(), input)
	require.ErrorContains(t, err, "not authorized")
	require.Empty(t, result.Candidates)
}

func TestExecutorRejectsSemanticFallbacksAndAcceptsModelLocalOutput(t *testing.T) {
	source := productimage.ImageAsset{URL: "https://example.test/source.jpg", SourceURL: "https://example.test/source.jpg"}
	tests := []struct {
		name  string
		asset productimage.ImageAsset
		valid bool
	}{
		{name: "source type", asset: productimage.ImageAsset{URL: "https://example.test/other.jpg", Type: productimage.AssetTypeSourceImage}},
		{name: "pass through operation", asset: productimage.ImageAsset{URL: "https://example.test/other.jpg", Operations: []string{"pass_through_gallery"}}},
		{name: "placeholder metadata", asset: productimage.ImageAsset{URL: "https://example.test/other.jpg", Metadata: map[string]string{"mode": "placeholder"}}},
		{name: "tenant fallback metadata", asset: productimage.ImageAsset{URL: "https://example.test/other.jpg", Metadata: map[string]string{"tenant_model_gate": "true"}}},
		{name: "canonical source url", asset: productimage.ImageAsset{URL: "HTTPS://EXAMPLE.TEST:443/assets/../source.jpg#fragment"}},
		{name: "model local output", asset: productimage.ImageAsset{URL: `C:\\work\\generated.png`, Type: productimage.AssetTypeGalleryImage, Metadata: map[string]string{"scene_mode": "model", "local_path": `C:\\work\\generated.png`}}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			executor := NewProductImageSlotExecutor(Dependencies{SceneRenderer: &recordingSceneRenderer{result: []productimage.ImageAsset{tt.asset}}})
			input := sceneSlotInput("scene-1")
			input.AssetCatalog.Assets[0].URL = source.URL
			input.AssetCatalog.Assets[0].SourceURL = source.SourceURL
			result, err := executor.ExecuteSlot(context.Background(), input)
			if tt.valid {
				require.NoError(t, err)
				require.Len(t, result.Candidates, 1)
				return
			}
			require.Error(t, err)
			require.Empty(t, result.Candidates)
		})
	}
}

func TestExecutorDoesNotMutateInputsOrCatalogAssets(t *testing.T) {
	input := sceneSlotInput(" scene-1 ")
	input.Slot.SourceAssetIDs = []string{" source-1 ", " source-2 "}
	input.Slot.StyleReferenceIDs = []string{" style-1 "}
	input.AssetCatalog.Assets = append(input.AssetCatalog.Assets, imageagent.AuthorizedAsset{ID: "style-1", Type: imageagent.AuthorizedAssetStyle})
	before := input
	before.Slot.SourceAssetIDs = append([]string(nil), input.Slot.SourceAssetIDs...)
	before.Slot.StyleReferenceIDs = append([]string(nil), input.Slot.StyleReferenceIDs...)
	input.AssetCatalog.Assets[0].Metadata = map[string]string{"owner": "test"}
	before.AssetCatalog.Assets = append([]imageagent.AuthorizedAsset(nil), input.AssetCatalog.Assets...)
	before.AssetCatalog.Assets[0].Metadata = map[string]string{"owner": "test"}
	executor := NewProductImageSlotExecutor(Dependencies{SceneRenderer: mutatingSceneRenderer{}})

	_, err := executor.ExecuteSlot(context.Background(), input)

	require.NoError(t, err)
	require.True(t, reflect.DeepEqual(before, input), "slot execution mutated caller input: before=%+v after=%+v", before, input)
	require.Equal(t, map[string]string{"owner": "test"}, input.AssetCatalog.Assets[0].Metadata)
}

func TestExecutorConcurrentReuseDoesNotShareMutableInputs(t *testing.T) {
	input := sceneSlotInput("scene-1")
	input.AssetCatalog.Assets[0].Metadata = map[string]string{"owner": "test"}
	executor := NewProductImageSlotExecutor(Dependencies{SceneRenderer: mutatingSceneRenderer{}})
	var group sync.WaitGroup
	errors := make(chan error, 32)
	for range 32 {
		group.Add(1)
		go func() {
			defer group.Done()
			_, err := executor.ExecuteSlot(context.Background(), input)
			errors <- err
		}()
	}
	group.Wait()
	close(errors)
	for err := range errors {
		require.NoError(t, err)
	}
	require.Equal(t, map[string]string{"owner": "test"}, input.AssetCatalog.Assets[0].Metadata)
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
			})
			result, err := executor.ExecuteSlot(context.Background(), sceneSlotInput("scene-1"))
			require.Error(t, err)
			require.Empty(t, result.Candidates)
		})
	}
}

func TestExecutorSizeSlotRequiresReliableDimensions(t *testing.T) {
	renderer := &recordingSceneRenderer{result: []productimage.ImageAsset{{URL: "https://generated.example/size.jpg"}}}
	executor := NewProductImageSlotExecutor(Dependencies{SceneRenderer: renderer})
	input := slotInput("size-1", imageagent.SlotRoleSize)

	_, err := executor.ExecuteSlot(context.Background(), input)
	require.ErrorContains(t, err, "reliable dimensions")
	require.Zero(t, renderer.calls)

	executor = NewProductImageSlotExecutor(Dependencies{
		SceneRenderer: renderer,
	})
	input.AssetCatalog.Assets[0].Width = 1200
	input.AssetCatalog.Assets[0].Height = 900
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
		Slot:         imageagent.Slot{ID: id, Role: role, SourceAssetIDs: []string{"source-1"}, IdempotencyKey: "slot-1"},
		AssetCatalog: imageagent.AssetCatalog{Assets: []imageagent.AuthorizedAsset{{ID: "source-1", Type: imageagent.AuthorizedAssetSource, URL: "https://example.test/source.jpg", SourceURL: "https://example.test/source.jpg", DisplayURL: "https://example.test/source.jpg", Metadata: map[string]string{}}}},
	}
}

func durablePublishedOutput(input imageagent.SlotExecutionInput, assets []imageagent.PublishedAssetRef) imageagent.PublishedSlotOutput {
	return imageagent.PublishedSlotOutput{SlotID: input.Slot.ID, Attempt: input.Attempt, Assets: assets}
}

func publicPublishedAsset() imageagent.PublishedAssetRef {
	return imageagent.PublishedAssetRef{
		ObjectKey:     "image-agent/public/tenant-1/run-1/1/scene-1/1/0-aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa.png",
		SHA256:        strings.Repeat("a", 64),
		SizeBytes:     1,
		ContentType:   "image/png",
		Width:         1,
		Height:        1,
		SourceAssetID: "source-1",
	}
}

type recordingSceneRenderer struct {
	calls   int
	result  []productimage.ImageAsset
	err     error
	asset   *productimage.ImageAsset
	context *productimage.ProductContext
}

type recordingAssetPublisher struct{ calls int }

func (p *recordingAssetPublisher) Publish(_ context.Context, _ *productimage.ImageProcessRequest, result *productimage.ImageProcessResult) error {
	p.calls++
	for index := range result.GalleryImages {
		result.GalleryImages[index].URL = "https://cdn.example.test/generated.png"
	}
	return nil
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

type mutatingSceneRenderer struct{}

func (mutatingSceneRenderer) Render(_ context.Context, asset *productimage.ImageAsset, _ *productimage.ProductContext) ([]productimage.ImageAsset, error) {
	asset.Operations = append(asset.Operations, "renderer_mutation")
	asset.Metadata["renderer_mutation"] = "true"
	return []productimage.ImageAsset{{URL: "https://example.test/generated.jpg", Type: productimage.AssetTypeGalleryImage}}, nil
}

func (r *recordingWhiteBackgroundRenderer) Render(_ context.Context, _ *productimage.ImageAsset, _ *productimage.ProductContext) (*productimage.ImageAsset, error) {
	r.calls++
	return r.result, nil
}

func stubSubjectExtractor() productimage.SubjectExtractor {
	return &recordingSubjectExtractor{result: &productimage.ImageAsset{URL: "https://generated.example/subject.png"}}
}

func candidateIDs(candidates []imageagent.AssetCandidate) []string {
	ids := make([]string, 0, len(candidates))
	for _, candidate := range candidates {
		ids = append(ids, candidate.AssetID)
	}
	return ids
}
