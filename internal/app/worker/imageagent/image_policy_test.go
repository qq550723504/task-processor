package imageagentworker

import (
	"testing"

	policycatalog "task-processor/internal/integration/policy/productimage"
	imagepolicy "task-processor/internal/marketplace/imagepolicy"
	productimage "task-processor/internal/product/image"

	"github.com/stretchr/testify/require"
)

func TestNewImagePolicyResolverMapsInfrastructureCatalogAtAppBoundary(t *testing.T) {
	t.Parallel()

	resolver, err := newImagePolicyResolver(policycatalog.Catalog{
		Version: "product-image-policy/v1",
		Policies: []policycatalog.Policy{{
			Key:        policycatalog.PolicyKey{Marketplace: "marketplace-a", Country: "xx", Family: "family-a", SceneCategory: "category-a"},
			Thresholds: policycatalog.Thresholds{MainReview: 0.61, WhiteBackgroundReview: 0.72, WhiteCanvasPenalty: 0.08},
			SceneDefaults: productimage.SceneOptions{
				SceneCategory: "category-a", SceneStyle: "studio", BackgroundTone: "neutral", Composition: "centered",
				PropsLevel: "none", AudienceHint: "general", StyleReferenceIDs: []string{"style-a"},
			},
		}},
	})

	require.NoError(t, err)
	profile, err := resolver.Resolve(imagepolicy.ProfileInput{Marketplace: "marketplace-a", Country: "xx", Family: "family-a", SceneCategory: "category-a"})
	require.NoError(t, err)
	require.Equal(t, imagepolicy.Thresholds{MainReview: 0.61, WhiteBackgroundReview: 0.72, WhiteCanvasPenalty: 0.08}, profile.Thresholds)
	require.Equal(t, []string{"style-a"}, profile.SceneDefaults.StyleReferenceIDs)
}

func TestLoadEmbeddedImagePolicyResolverFailsClosedAtAppComposition(t *testing.T) {
	t.Parallel()

	resolver, err := loadEmbeddedImagePolicyResolver()

	require.NoError(t, err)
	require.NotNil(t, resolver)
}
