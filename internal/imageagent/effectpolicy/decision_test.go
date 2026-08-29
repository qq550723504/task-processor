package effectpolicy

import (
	"testing"

	"task-processor/internal/imageagent"
)

func TestEffectDecisionDoesNotAliasAttemptSlices(t *testing.T) {
	attempt := imageagent.SlotEffectV3Attempt{
		StagingManifest: imageagent.StagingManifest{
			Assets:           []imageagent.StagedAssetRef{{Operations: []string{"resize"}}},
			ProviderMetadata: map[string]string{"source": "provider-a"},
		},
		FinalManifest: imageagent.FinalManifest{
			Assets: []imageagent.PublishedAssetRef{{Operations: []string{"cleanup_image"}}},
		},
		Published: imageagent.SlotEffectV3PublishedResult{
			Candidates: []imageagent.SlotEffectV3AssetCandidate{{AssetID: "candidate-a"}},
		},
	}

	cloned := cloneSlotEffectV3Attempt(attempt)
	cloned.StagingManifest.Assets[0].Operations[0] = "render_white_bg"
	cloned.StagingManifest.ProviderMetadata["source"] = "provider-b"
	cloned.FinalManifest.Assets[0].Operations[0] = "resize"
	cloned.Published.Candidates[0].AssetID = "candidate-b"

	if got := attempt.StagingManifest.Assets[0].Operations[0]; got != "resize" {
		t.Errorf("staging operations aliased input: got %q", got)
	}
	if got := attempt.StagingManifest.ProviderMetadata["source"]; got != "provider-a" {
		t.Errorf("staging provider metadata aliased input: got %q", got)
	}
	if got := attempt.FinalManifest.Assets[0].Operations[0]; got != "cleanup_image" {
		t.Errorf("final operations aliased input: got %q", got)
	}
	if got := attempt.Published.Candidates[0].AssetID; got != "candidate-a" {
		t.Errorf("published candidates aliased input: got %q", got)
	}
}
