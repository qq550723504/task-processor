package imageagentpolicy

import (
	"testing"

	"github.com/stretchr/testify/require"
	"task-processor/internal/marketplace/imagepolicy"
)

func TestAcquisitionMainImageUsesApprovedProviderNeutralWhiteBackgroundPolicy(t *testing.T) {
	resolver, err := LoadEmbeddedResolver()
	require.NoError(t, err)
	profile, err := resolver.Resolve(imagepolicy.ProfileInput{
		Marketplace: "product", Country: "zz", Family: "default", SceneCategory: "general",
	})
	require.NoError(t, err)
	require.Equal(t, 0.70, profile.Thresholds.WhiteBackgroundReview)
	require.Equal(t, 0.65, profile.Thresholds.MainReview)
	require.Equal(t, 0.10, profile.Thresholds.WhiteCanvasPenalty)
	require.Equal(t, "general", profile.SceneDefaults.SceneCategory)
}
