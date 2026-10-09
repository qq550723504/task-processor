package assetpublication

import (
	"bytes"
	"context"
	"encoding/json"
	"image"
	"image/png"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"task-processor/internal/ai"
	"task-processor/internal/imageagent"
	"task-processor/internal/imageagent/objectstore"
	"task-processor/internal/imageagent/tools"
	"task-processor/internal/integration/grsai"
	openai "task-processor/internal/integration/openai"
	productimage "task-processor/internal/product/image"
)

// Controlled bytes/storage only; generation, materialization and selection use
// their actual production adapters. No provider is called during selection.
func TestSynchronousImageSetOutputCanBeSelectedWithDistinctOrMissingRequestID(t *testing.T) {
	for _, requestID := range []string{"http-request-1", ""} {
		t.Run(requestID, func(t *testing.T) {
			ctx := context.Background()
			var pngData bytes.Buffer
			require.NoError(t, png.Encode(&pngData, image.NewRGBA(image.Rect(0, 0, 1024, 1024))))
			projection, facts, source, choice := imageSetCandidateFixtureWithSource(t, pngData.Bytes())
			input, err := imageagent.ImageSetSlotExecutionInput(projection, choice.SlotID, choice.Attempt)
			require.NoError(t, err)
			input.SourceReferences = [][]byte{pngData.Bytes()}
			fact, err := imageagent.NewGenerationFact(facts.fact.Intent)
			require.NoError(t, err)
			fact, err = fact.BindReservation(facts.fact.Reservation)
			require.NoError(t, err)
			fact, _, err = fact.BeginDispatch()
			require.NoError(t, err)
			submits := 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				submits++
				require.Equal(t, http.MethodPost, r.Method)
				w.Header().Set("X-Request-Id", requestID)
				require.NoError(t, json.NewEncoder(w).Encode(map[string]any{"id": "generation-job-1", "status": "succeeded", "results": []map[string]string{{"url": "https://output.example.org/result.png"}}}))
			}))
			defer server.Close()
			client := grsai.NewClient(grsai.Config{Model: "gpt-image-2.5", SubmitURL: server.URL, HTTPClient: server.Client()})
			provider, err := grsai.NewSynchronousProductImageAdapter(grsai.ProductImageAdapterConfig{Client: client, ImageModel: "gpt-image-2.5", RouteReference: "route", CredentialReference: "credential", ConfigurationVersion: "config", Build: func(bound ai.RouteBoundImageGenerator) (grsai.ProductImageProvider, error) {
				return openai.NewProductImageAdapter(openai.ProductImageAdapterConfig{ImageClient: bound, Provider: "grsai", ImageModel: "gpt-image-2.5", RouteReference: "route", CredentialReference: "credential", ConfigurationVersion: "config", PricingVersion: "controlled-unpriced", Prompts: openai.DefaultProductImagePrompts(), MaximumSceneOutputs: 1, GeneratedImageFetcher: func(context.Context, string) ([]byte, error) { return pngData.Bytes(), nil }})
			}}, func(_ context.Context, observation grsai.GenerationObservation) error {
				require.Equal(t, "generation-job-1", observation.ResponseID)
				require.Equal(t, requestID, observation.RequestID)
				var err error
				fact, err = fact.RecordSuccess(imageagent.GenerationSuccess{ResponseID: observation.ResponseID, RequestID: observation.RequestID, ResultDigest: observation.ResultDigest, ResultURL: observation.ResultURL})
				return err
			})
			require.NoError(t, err)
			editor, err := productimage.NewSourceEditCapability(provider.(productimage.SourceEditor))
			require.NoError(t, err)
			executor := tools.NewProductImageSlotExecutor(tools.Dependencies{SourceEditor: editor, UsageQuoter: provider})
			quote, err := executor.QuoteSlot(ctx, input, imageagent.BudgetPolicy{})
			require.NoError(t, err)
			output, err := executor.GenerateQuotedSlot(ctx, input, quote)
			require.NoError(t, err)
			require.Equal(t, requestID, output.Assets[0].ProviderReceiptID)
			require.Equal(t, 1, submits)
			fact, err = fact.BindSettlement(imageagent.GenerationSettlementReceipt{IntentID: fact.IntentID, Fingerprint: fact.Fingerprint, OperationID: "image-finalize:" + fact.IntentID, ReservationID: fact.Reservation.ReservationID, State: "committed", Points: fact.Intent.Points, ProofDigest: fact.TerminalProofDigest()})
			require.NoError(t, err)
			storage := &adapterObjectFixture{objects: map[string]objectstore.ImmutableObjectPut{}}
			artifacts, err := objectstore.NewDurableArtifactStore(storage, objectstore.DurableArtifactStoreConfig{MaxArtifactBytes: 16 << 20, OperationTimeout: time.Second})
			require.NoError(t, err)
			generated := output.Assets[0]
			prepared, err := artifacts.PrepareSlotArtifacts(objectstore.PrepareSlotArtifactsInput{Identity: fact.Intent.Identity, Assets: []objectstore.ArtifactInput{{Bytes: generated.Bytes, ContentType: generated.ContentType, Width: generated.Width, Height: generated.Height, SourceAssetID: output.SourceAssetID, Operations: generated.Operations, ProviderReceiptID: generated.ProviderReceiptID}}})
			require.NoError(t, err)
			require.NoError(t, artifacts.EnsureStaged(ctx, prepared))
			manifest, err := artifacts.Finalize(ctx, prepared.Manifest)
			require.NoError(t, err)
			built, err := tools.NewImageSetResultBuilder().BuildSlotResult(ctx, input, imageagent.PublishedSlotOutput{SlotID: choice.SlotID, Attempt: choice.Attempt, Assets: manifest.Assets})
			require.NoError(t, err)
			proof := &imageagent.ImageGenerationProof{IntentID: fact.IntentID, Fingerprint: fact.Fingerprint, SettlementProofDigest: fact.TerminalProofDigest(), Points: fact.Intent.Points}
			built.Candidates[0].GenerationProof = proof
			projection.Slots[0].Candidates = built.Candidates
			projection.Slots[0].Closure, err = imageagent.ImageGenerationClosure(fact)
			require.NoError(t, err)
			projection.ResultDigest, err = imageagent.ImageSetResultDigest(projection.Plan, projection.Slots, nil)
			require.NoError(t, err)
			published, err := imageagent.NewSlotEffectV3PublishedResult(built)
			require.NoError(t, err)
			fingerprint, err := imageagent.SlotEffectV3PublishedResultFingerprint(published)
			require.NoError(t, err)
			facts.fact = fact
			facts.effect.FinalManifest = manifest
			facts.effect.Published = published
			facts.effect.ResultFingerprint = fingerprint
			choice.AssetID = built.Candidates[0].AssetID
			choice.ResultDigest = projection.ResultDigest
			reader, err := NewImageSetCandidateReader(staticProjectionSource{projection: projection}, facts, facts, artifacts)
			require.NoError(t, err)
			selected, err := reader.ReadImageSetCandidate(ctx, source, choice)
			require.NoError(t, err)
			require.Equal(t, built.Candidates[0].AssetID, selected.Asset.ID)
			require.Equal(t, 1, submits, "selection must never submit again")
		})
	}
}

type adapterObjectFixture struct {
	objects map[string]objectstore.ImmutableObjectPut
}

func (s *adapterObjectFixture) PublicURL(key string) string {
	return "https://assets.example.org/" + key
}
func (s *adapterObjectFixture) InspectObject(_ context.Context, key string) (objectstore.ObjectInspection, error) {
	value, ok := s.objects[key]
	if !ok {
		return objectstore.ObjectInspection{}, nil
	}
	return objectstore.ObjectInspection{Exists: true, ContentLength: value.SizeBytes, ContentType: value.ContentType, Metadata: map[string]string{"sha256": value.SHA256, "size-bytes": strconv.FormatInt(value.SizeBytes, 10)}}, nil
}
func (s *adapterObjectFixture) ReadObject(ctx context.Context, key string, _ int64) ([]byte, objectstore.ObjectInspection, error) {
	info, err := s.InspectObject(ctx, key)
	return s.objects[key].Data, info, err
}
func (s *adapterObjectFixture) PutImmutable(_ context.Context, value objectstore.ImmutableObjectPut) error {
	s.objects[value.Key] = value
	return nil
}
func (s *adapterObjectFixture) CopyImmutable(_ context.Context, value objectstore.ImmutableObjectCopy) error {
	copy := value.Destination
	copy.Data = s.objects[value.SourceKey].Data
	s.objects[copy.Key] = copy
	return nil
}
