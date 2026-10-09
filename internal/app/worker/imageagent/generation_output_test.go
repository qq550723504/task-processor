package imageagentworker

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"image"
	"image/png"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"task-processor/internal/agent"
	"task-processor/internal/imageagent"
)

func TestGenerationOutputRecoveryOnlyFetchesBoundOriginalResult(t *testing.T) {
	var encoded bytes.Buffer
	require.NoError(t, png.Encode(&encoded, image.NewRGBA(image.Rect(0, 0, 2, 2))))
	var setEncoded bytes.Buffer
	require.NoError(t, png.Encode(&setEncoded, image.NewRGBA(image.Rect(0, 0, 1024, 1024))))
	input := imageagent.SlotExecutionInput{RunID: "run-1", TenantID: "org-1", UserID: "actor-1", PlanRevision: 1, Attempt: 1, OrganizationIdentity: imageagent.ExecutionIdentity{ScopeProtocol: imageagent.OrganizationScopeProtocol, MemberID: "member-1"}, Slot: imageagent.Slot{ID: "main", Role: imageagent.SlotRoleMain, SourceAssetIDs: []string{"source-1"}}, AssetCatalog: imageagent.AssetCatalog{Assets: []imageagent.AuthorizedAsset{{ID: "source-1", Type: imageagent.AuthorizedAssetSource, URL: "https://source.example/original.png"}}}}
	catalog, err := imageagent.NormalizeAssetCatalog(input.AssetCatalog)
	require.NoError(t, err)
	input.AssetCatalog = catalog
	intent := imageagent.GenerationIntent{Identity: imageagent.SlotExternalEffectIdentity{RunScope: imageagent.RunScope{TenantID: input.TenantID, OwnerUserID: input.UserID, RunID: input.RunID}, PlanRevision: 1, SlotID: "main", Attempt: 1}, MemberID: "member-1", CatalogHash: catalog.Manifest.Hash, SourceDigest: strings.Repeat("b", 64), PromptVersion: "p1", RouteReference: "r1", CredentialReference: "c1", ConfigurationVersion: "v1", Provider: "grsai", Model: "gpt-image-2.5", Protocol: "grsai-json-sync-v1", Resolution: "1024x1024", Quality: "auto", PriceVersion: "old-price", Points: 12, LimitVersion: 1, MonthStart: time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)}
	fact, err := imageagent.NewGenerationFact(intent)
	require.NoError(t, err)
	fact, err = fact.BindReservation(imageagent.GenerationReservationReceipt{IntentID: fact.IntentID, Fingerprint: fact.Fingerprint, OrganizationID: input.TenantID, MemberID: intent.MemberID, OperationID: "image-reserve:" + fact.IntentID, ReservationID: "reserved-1", ResourceType: "ai_point", Points: 12, PriceVersion: intent.PriceVersion, LimitVersion: 1, MonthStart: intent.MonthStart})
	require.NoError(t, err)
	fact, _, err = fact.BeginDispatch()
	require.NoError(t, err)
	started := fact
	proof := imageagent.GenerationSuccess{ResponseID: "generated-1", ResultDigest: strings.Repeat("c", 64)}.WithResultLocator("https://output.example/image.png?signature=private", "")
	fact, err = fact.RecordSuccess(proof)
	require.NoError(t, err)
	for _, mode := range []string{"valid", "image_set", "changed_set", "bad_set", "wrong_size", "empty_set", "fetch_failure_set", "private", "oversized_locator", "member", "catalog", "unknown", "bad_image", "fetch_failure"} {
		t.Run(mode, func(t *testing.T) {
			calls := 0
			in, current := input, fact
			switch mode {
			case "image_set", "changed_set", "bad_set", "wrong_size", "empty_set", "fetch_failure_set":
				hash := strings.Repeat("a", 64)
				in.ImageSet = &imageagent.ImageSetPlan{Schema: imageagent.ImageSetSchema, Source: imageagent.ImageSourceBinding{ContextKind: imageagent.ImageSourceAcquisition, ProductID: "product-1", OperationID: "source-operation", OriginalPublicationID: "publication-1", OriginalVersion: 1, EffectiveVersion: 1, CatalogHash: catalog.Manifest.Hash}, Target: imageagent.ImageTarget{Platform: "product"}, Configuration: agent.ConfigurationSnapshotRef{Kind: "agent-configuration-v1", ID: "9e7afaa9-a9f9-48ba-a11a-b5bb377f08e9", Digest: hash}, ConfigurationEpoch: "1", ParametersDigest: hash, InputDigest: hash, MaxPoints: 12}
				in.TargetPlatform = "product"
				in.Slot.Role = imageagent.SlotRoleDetail
				in.Slot.Recipe = &imageagent.ImageSlotRecipe{Purpose: "product_overview", Background: "white", Language: "en", Placement: imageagent.ImagePlacement{Group: "detail", Order: 1}, PromptVersion: imageagent.ImageSetSchema, Prompt: "approved overview", References: []imageagent.ImageSourceObservation{{AssetID: "source-1", SHA256: hash, MediaType: "image/png", Bytes: 10, Width: 2, Height: 2}}, Quote: imageagent.ImageGenerationQuote{Provider: "grsai", Model: "gpt-image-2.5", Protocol: "grsai-json-sync-v1", Resolution: "1024x1024", Quality: "auto", PriceVersion: "old-price", Points: 12, RouteReference: "r1", CredentialReference: "c1", ConfigurationVersion: "v1"}}
				in.ImageSet.QuoteDigest, err = imageagent.ImageSetQuoteDigest(imageagent.Plan{Set: in.ImageSet, Slots: []imageagent.Slot{in.Slot}})
				require.NoError(t, err)
				i := intent
				i.InputProtocol = imageagent.ImageSetSchema
				i.InputDigest, err = imageagent.ImageSlotGenerationInputDigest(in)
				require.NoError(t, err)
				i.SourceDigest = imageagent.ImageSourceBundleDigest(in.Slot.Recipe.References)
				i.PromptVersion = imageagent.ImageSetSchema
				current, err = imageagent.NewGenerationFact(i)
				require.NoError(t, err)
				reservation := fact.Reservation
				reservation.Fingerprint = current.Fingerprint
				current, err = current.BindReservation(reservation)
				require.NoError(t, err)
				current, _, err = current.BeginDispatch()
				require.NoError(t, err)
				current, err = current.RecordSuccess(proof)
				require.NoError(t, err)
				if mode == "changed_set" {
					in.Slot.Recipe.Prompt = "changed purpose"
				}
			case "private":
				current, _ = started.RecordSuccess(proof.WithResultLocator("https://127.0.0.1/secret", ""))
			case "oversized_locator":
				current, _ = started.RecordSuccess(proof.WithResultLocator("https://output.example/"+strings.Repeat("x", 4096), ""))
			case "member":
				in.OrganizationIdentity.MemberID = "other"
			case "catalog":
				in.AssetCatalog.Manifest.Hash = "catalog-v1:" + strings.Repeat("d", 64)
			case "unknown":
				current, _ = started.MarkUnknown()
			}
			materialize := generationOutputRecovery(func(_ context.Context, raw string) ([]byte, error) {
				calls++
				require.Equal(t, proof.ResultURL, raw)
				if mode == "fetch_failure" || mode == "fetch_failure_set" {
					return nil, errors.New(raw)
				}
				if mode == "bad_image" || mode == "bad_set" {
					return []byte("not image"), nil
				}
				if mode == "empty_set" {
					return nil, nil
				}
				if mode == "image_set" {
					return setEncoded.Bytes(), nil
				}
				return encoded.Bytes(), nil
			})
			got, err := materialize(context.Background(), in, current)
			if mode == "valid" || mode == "image_set" {
				require.NoError(t, err)
				if mode == "image_set" {
					require.Equal(t, setEncoded.Bytes(), got.Assets[0].Bytes)
				} else {
					require.Equal(t, encoded.Bytes(), got.Assets[0].Bytes)
				}
				require.Equal(t, "https://source.example/original.png", got.Assets[0].SourceURL)
				wire, e := json.Marshal(got)
				require.NoError(t, e)
				require.NotContains(t, string(wire), "signature=private")
			} else {
				require.Error(t, err)
				require.NotContains(t, err.Error(), "signature")
				require.Empty(t, got.Assets)
			}
			if mode == "bad_set" || mode == "wrong_size" || mode == "empty_set" {
				require.ErrorIs(t, err, imageagent.ErrInvalidGeneratedOutput)
			} else if mode != "bad_image" {
				require.NotErrorIs(t, err, imageagent.ErrInvalidGeneratedOutput)
			}
			if mode == "valid" || mode == "image_set" || mode == "bad_image" || mode == "fetch_failure" || mode == "bad_set" || mode == "wrong_size" || mode == "empty_set" || mode == "fetch_failure_set" {
				require.Equal(t, 1, calls)
			} else {
				require.Zero(t, calls)
			}
		})
	}
}
