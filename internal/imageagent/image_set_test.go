package imageagent

import (
	"encoding/json"
	"math"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func setPlanFixture(t *testing.T) Plan {
	t.Helper()
	hash := strings.Repeat("a", 64)
	raw := `{"Revision":1,"IdempotencyKey":"set-plan","SourceAssetIDs":["source-1"],"CreatedBy":"actor","Set":{"Schema":"product-image-set-v1","Source":{"ProductID":"product-1","OperationID":"source-operation","OriginalPublicationID":"source-product","OriginalVersion":1,"EffectiveVersion":1,"CatalogHash":"catalog-v2:HASH"},"Target":{"Platform":"product"},"Configuration":{"Kind":"agent-configuration-v1","ID":"9e7afaa9-a9f9-48ba-a11a-b5bb377f08e9","Digest":"HASH"},"ConfigurationEpoch":"1","ParametersDigest":"HASH","InputDigest":"HASH","QuoteDigest":"HASH","MaxPoints":20},"Slots":[{"ID":"overview","Role":"detail","SourceAssetIDs":["source-1"],"IdempotencyKey":"overview-1","Status":"pending","Recipe":{"Purpose":"product_overview","Background":"白色","Language":"zh","Placement":{"Group":"detail","Order":1},"PromptVersion":"product-image-set-v1","Prompt":"根据原始商品图片表现商品全貌，不编造事实","References":[{"AssetID":"source-1","SHA256":"HASH","MediaType":"image/png","Bytes":10,"Width":1024,"Height":1024}],"Quote":{"Provider":"grsai","Model":"gpt-image-2.5","Protocol":"grsai-json-sync-v1","Resolution":"1024x1024","Quality":"auto","PriceVersion":"price-1","Points":20,"RouteReference":"route-1","CredentialReference":"credential-1","ConfigurationVersion":"config-1"}}}]} `
	var plan Plan
	require.NoError(t, json.Unmarshal([]byte(strings.ReplaceAll(raw, "HASH", hash)), &plan))
	if plan.Set != nil {
		var err error
		plan.Set.QuoteDigest, err = ImageSetQuoteDigest(plan)
		require.NoError(t, err)
	}
	return plan
}

func TestSetPlanPermitsDetailOnlyWithoutWeakeningSingleImagePlan(t *testing.T) {
	plan := setPlanFixture(t)
	require.NoError(t, ValidateInitialSubmittedPlan(plan), "a detail-only set must not require a fabricated main image")
	// Re-encode without the set protocol: the original plan contract still
	// requires one Main, regardless of a caller's requested content role.
	raw, err := json.Marshal(plan)
	require.NoError(t, err)
	var object map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(raw, &object))
	delete(object, "Set")
	raw, err = json.Marshal(object)
	require.NoError(t, err)
	var original Plan
	require.NoError(t, json.Unmarshal(raw, &original))
	require.Error(t, ValidateInitialSubmittedPlan(original))
}

func setResultFixture(t *testing.T) (Plan, []SlotProjection) {
	plan := setPlanFixture(t)
	failed := plan.Slots[0]
	failed.ID, failed.IdempotencyKey = "details", "details-1"
	failed.Recipe = CloneImageSlotRecipe(failed.Recipe)
	failed.Recipe.Placement.Order = 2
	plan.Slots = append(plan.Slots, failed)
	plan.Set.MaxPoints = 40
	var err error
	plan.Set.QuoteDigest, err = ImageSetQuoteDigest(plan)
	require.NoError(t, err)
	hash := strings.Repeat("a", 64)
	proof := &ImageGenerationProof{IntentID: hash, Fingerprint: hash, SettlementProofDigest: hash, Points: 20}
	accepted := plan.Slots[0]
	accepted.Status = SlotStatusAccepted
	failed.Status = SlotStatusBlocked
	return plan, []SlotProjection{
		{Slot: accepted, Attempt: 1, Closure: &ImageSlotClosure{Kind: "settled", IntentID: hash, Fingerprint: hash, SettlementProofDigest: hash, Points: 20}, Candidates: []AssetCandidate{{AssetID: "candidate-1", SourceAssetID: "source-1", Width: 1024, Height: 1024, Operations: []string{"render_source_edit"}, DurableAsset: DurableAssetIdentity{ObjectKey: "image-agent/public/tenant-a/run-1/1/overview/1/0-" + hash + ".png", SHA256: hash}, GenerationProof: proof}}},
		{Slot: failed, ErrorCode: "budget_exceeded", Closure: &ImageSlotClosure{Kind: "not_dispatched"}},
	}
}

func TestSetResultDigestAllowsKnownPartialFailureButRejectsUnknownOrUnsettled(t *testing.T) {
	plan, slots := setResultFixture(t)
	digest, err := ImageSetResultDigest(plan, slots, nil)
	require.NoError(t, err)
	require.NotEmpty(t, digest)
	_, err = ResultDigestV3(plan, slots)
	require.Error(t, err, "the original all-accepted digest must retain its original contract")
	_, err = ImageSetResultDigest(plan, slots, []RecoverableEffect{{SlotID: "details", Attempt: 1, Code: "provider_outcome_unknown"}})
	require.Error(t, err)
	slots[1].Closure = nil
	_, err = ImageSetResultDigest(plan, slots, nil)
	require.Error(t, err, "absence of a recoverable-effect list is not proof of settlement")
	plan, slots = setResultFixture(t)
	slots[0].Candidates[0].GenerationProof = nil
	_, err = ImageSetResultDigest(plan, slots, nil)
	require.Error(t, err)
	plan, slots = setResultFixture(t)
	slots[1].ErrorCode = "configuration_changed"
	changed, err := ImageSetResultDigest(plan, slots, nil)
	require.NoError(t, err)
	require.NotEqual(t, digest, changed, "known failures and skipped work remain in the exact result")
	raw, err := json.Marshal(slots)
	require.NoError(t, err)
	var restored []SlotProjection
	require.NoError(t, json.Unmarshal(raw, &restored))
	require.Equal(t, slots, restored, "restart must preserve closure/provenance references")
}

func TestSetPlanChecksSourceBoundsQuoteChangesAndPointSumOverflow(t *testing.T) {
	plan := setPlanFixture(t)
	plan.Slots[0].Recipe.References[0].SHA256 = "missing"
	require.Error(t, ValidateSubmittedPlan(plan))
	plan = setPlanFixture(t)
	plan.Slots[0].Recipe.Quote.PriceVersion = "price-2"
	require.Error(t, ValidateSubmittedPlan(plan), "the frozen quote digest must reject a price change")
	plan = setPlanFixture(t)
	plan.Slots[0].Recipe.References[0].Bytes = 16 << 20
	plan.Slots[0].SourceAssetIDs = append(plan.Slots[0].SourceAssetIDs, "source-2")
	plan.SourceAssetIDs = append(plan.SourceAssetIDs, "source-2")
	plan.Slots[0].Recipe.References = append(plan.Slots[0].Recipe.References, ImageSourceObservation{AssetID: "source-2", SHA256: strings.Repeat("b", 64), MediaType: "image/png", Bytes: 1, Width: 1, Height: 1})
	require.Error(t, ValidateSubmittedPlan(plan), "an aggregate source bundle must remain bounded")
	plan = setPlanFixture(t)
	plan.Slots[0].Recipe.Quote.Points = math.MaxInt64
	second := plan.Slots[0]
	second.Recipe = CloneImageSlotRecipe(second.Recipe)
	second.Recipe.Quote.Points = 1
	plan.Slots = append(plan.Slots, second)
	_, err := ImageSetPoints(plan)
	require.Error(t, err, "point overflow cannot become a small or negative budget")
}
