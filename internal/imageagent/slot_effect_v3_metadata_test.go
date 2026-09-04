package imageagent

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNewSlotEffectV3PublishedResultPreservesArtifactMetadata(t *testing.T) {
	result, err := NewSlotEffectV3PublishedResult(SlotExecutionResult{
		SlotID: "scene-1", Attempt: 1, Candidates: []AssetCandidate{{
			AssetID: "candidate-1", SourceAssetID: "source-1", Width: 1200, Height: 900,
			Operations: []string{"render_scene", "review"}, DurableAsset: DurableAssetIdentity{
				ObjectKey: "image-agent/public/tenant-a/run-1/1/scene-1/1/0-aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa.png",
				SHA256:    "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
			},
		}},
	})

	require.NoError(t, err)
	require.Equal(t, 1200, result.Candidates[0].Width)
	require.Equal(t, 900, result.Candidates[0].Height)
	require.Equal(t, []string{"render_scene", "review"}, result.Candidates[0].Operations)
}
