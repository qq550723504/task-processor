package tools

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"github.com/stretchr/testify/require"
	"strings"
	"task-processor/internal/agent"
	"task-processor/internal/imageagent"
	productimage "task-processor/internal/product/image"
	"testing"
)

type recordingSourceEditor struct {
	calls     int
	request   productimage.SourceEditRequest
	candidate productimage.Candidate
}

func (e *recordingSourceEditor) EditSources(_ context.Context, r productimage.SourceEditRequest) (productimage.Candidate, error) {
	e.calls++
	e.request = r
	return e.candidate, nil
}

func setExecutionInput(t *testing.T) imageagent.SlotExecutionInput {
	input := testProductImageExecutionInput()
	input.TargetPlatform = "product"
	input.ImagePolicyContext = nil
	input.Slot.Role = imageagent.SlotRoleDetail
	input.Slot.Brief = ""
	input.Slot.SourceAssetIDs = []string{"source-1", "source-2"}
	second := input.AssetCatalog.Assets[0]
	second.ID = "source-2"
	second.URL = "https://source.example/two.png"
	second.SourceURL = second.URL
	input.AssetCatalog.Assets = append(input.AssetCatalog.Assets, second)
	input.SourceReferences = [][]byte{testTinyPNG(t), []byte("second exact source")}
	hash := strings.Repeat("a", 64)
	input.ImageSet = &imageagent.ImageSetPlan{Schema: imageagent.ImageSetSchema, Source: imageagent.ImageSourceBinding{ProductID: "product-1", OperationID: "operation-1", OriginalPublicationID: "publication-1", OriginalVersion: 1, EffectiveVersion: 1, CatalogHash: hash}, Target: imageagent.ImageTarget{Platform: "product"}, Configuration: agent.ConfigurationSnapshotRef{Kind: "agent-configuration-v1", ID: "9e7afaa9-a9f9-48ba-a11a-b5bb377f08e9", Digest: hash}, ConfigurationEpoch: "1", ParametersDigest: hash, InputDigest: hash, MaxPoints: 12}
	input.Slot.Recipe = &imageagent.ImageSlotRecipe{Purpose: "product_overview", Background: "white", Language: "en", Placement: imageagent.ImagePlacement{Group: "detail", Order: 1}, PromptVersion: imageagent.ImageSetSchema, Prompt: "approved product overview", Quote: imageagent.ImageGenerationQuote{Provider: "grsai", Model: "gpt-image-2.5", Protocol: "grsai-json-sync-v1", Resolution: "1024x1024", Quality: "auto", PriceVersion: "price", Points: 12, RouteReference: "route", CredentialReference: "credential", ConfigurationVersion: "config"}}
	for i, data := range input.SourceReferences {
		sum := sha256.Sum256(data)
		input.Slot.Recipe.References = append(input.Slot.Recipe.References, imageagent.ImageSourceObservation{AssetID: input.Slot.SourceAssetIDs[i], SHA256: hex.EncodeToString(sum[:]), MediaType: "image/png", Bytes: int64(len(data)), Width: 1200, Height: 1200})
	}
	var err error
	input.ImageSet.QuoteDigest, err = imageagent.ImageSetQuoteDigest(imageagent.Plan{Set: input.ImageSet, Slots: []imageagent.Slot{input.Slot}})
	require.NoError(t, err)
	return input
}

func TestImageSetExecutorUsesSingleEditForDetailAndBindsEverySource(t *testing.T) {
	input := setExecutionInput(t)
	editor := &recordingSourceEditor{candidate: testSceneCandidate(t, "https://source.example/item.png")}
	editor.candidate.Asset.Operations = []string{productimage.SourceEditOperation}
	executor := NewProductImageSlotExecutor(Dependencies{SourceEditor: editor, UsageQuoter: testProductUsageQuoter{}})
	quote, err := executor.QuoteSlot(context.Background(), input, imageagent.BudgetPolicy{})
	require.NoError(t, err)
	require.Len(t, quote.Operations, 1)
	require.Equal(t, productimage.SourceEditOperation, quote.Operations[0].Name)
	output, err := executor.GenerateQuotedSlot(context.Background(), input, quote)
	require.NoError(t, err)
	require.Len(t, output.Assets, 1)
	require.Equal(t, input.SourceReferences, editor.request.ReferenceBytes)
	require.Len(t, editor.request.Sources, 2)
	require.Equal(t, input.Slot.Recipe.Prompt, editor.request.Prompt)
	require.EqualValues(t, 1, output.UsageReceipt.Actual.ModelCalls)
	changed := input
	changed.SourceReferences = [][]byte{input.SourceReferences[0], []byte("changed")}
	_, err = executor.GenerateQuotedSlot(context.Background(), changed, quote)
	require.ErrorIs(t, err, imageagent.ErrRevisionConflict)
	require.Equal(t, 1, editor.calls)
	changed = input
	changed.Slot.Recipe = imageagent.CloneImageSlotRecipe(input.Slot.Recipe)
	changed.Slot.Recipe.Prompt = "different purpose"
	_, err = executor.GenerateQuotedSlot(context.Background(), changed, quote)
	require.ErrorIs(t, err, imageagent.ErrRevisionConflict)
	require.Equal(t, 1, editor.calls)
	_, err = executor.QuoteStagedReview(context.Background(), input, imageagent.BudgetPolicy{})
	require.Error(t, err, "no hidden reviewer on set generation or recovery")
}
