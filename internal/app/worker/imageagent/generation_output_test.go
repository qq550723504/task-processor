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

	"task-processor/internal/imageagent"
)

func TestGenerationOutputRecoveryOnlyFetchesBoundOriginalResult(t *testing.T) {
	var encoded bytes.Buffer
	require.NoError(t, png.Encode(&encoded, image.NewRGBA(image.Rect(0, 0, 2, 2))))
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
	for _, mode := range []string{"valid", "private", "oversized_locator", "member", "catalog", "unknown", "bad_image", "fetch_failure"} {
		t.Run(mode, func(t *testing.T) {
			calls := 0
			in, current := input, fact
			switch mode {
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
				if mode == "fetch_failure" {
					return nil, errors.New(raw)
				}
				if mode == "bad_image" {
					return []byte("not image"), nil
				}
				return encoded.Bytes(), nil
			})
			got, err := materialize(context.Background(), in, current)
			if mode == "valid" {
				require.NoError(t, err)
				require.Equal(t, encoded.Bytes(), got.Assets[0].Bytes)
				require.Equal(t, "https://source.example/original.png", got.Assets[0].SourceURL)
				wire, e := json.Marshal(got)
				require.NoError(t, e)
				require.NotContains(t, string(wire), "signature=private")
			} else {
				require.Error(t, err)
				require.NotContains(t, err.Error(), "signature")
				require.Empty(t, got.Assets)
			}
			if mode == "valid" || mode == "bad_image" || mode == "fetch_failure" {
				require.Equal(t, 1, calls)
			} else {
				require.Zero(t, calls)
			}
		})
	}
}
