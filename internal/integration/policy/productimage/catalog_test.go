package productimage

import (
	"strings"
	"testing"

	imagepolicy "task-processor/internal/marketplace/imagepolicy"
	productimage "task-processor/internal/product/image"

	"github.com/stretchr/testify/require"
)

func TestDecodeReturnsTypedPolicySetFromStrictDocument(t *testing.T) {
	t.Parallel()

	set, err := Decode(strings.NewReader(validCatalogFixture))

	require.NoError(t, err)
	require.Equal(t, imagepolicy.PolicySet{
		Version: "product-image-policy/v1",
		Policies: []imagepolicy.Policy{{
			Key: imagepolicy.PolicyKey{
				Marketplace:   "marketplace-a",
				Country:       "xx",
				Family:        "family-a",
				SceneCategory: "category-a",
			},
			Thresholds: imagepolicy.Thresholds{
				MainReview:            0.61,
				WhiteBackgroundReview: 0.72,
				WhiteCanvasPenalty:    0.08,
			},
			SceneDefaults: productimage.SceneOptions{
				SceneCategory:     "category-a",
				SceneStyle:        "studio",
				BackgroundTone:    "neutral",
				Composition:       "centered",
				PropsLevel:        "none",
				AudienceHint:      "general",
				CustomSceneHint:   "soft shadow",
				SlotRole:          "scene",
				SlotBrief:         "show product clearly",
				StyleReferenceIDs: []string{"style-a"},
			},
		}},
	}, set)
}

func TestDecodeRejectsUnknownFieldsAndAdditionalDocuments(t *testing.T) {
	t.Parallel()

	unknownField := strings.Replace(validCatalogFixture, "schema:", "unexpected: true\nschema:", 1)
	set, err := Decode(strings.NewReader(unknownField))
	require.Empty(t, set)
	require.ErrorIs(t, err, ErrInvalidCatalog)

	set, err = Decode(strings.NewReader(validCatalogFixture + "\n---\n" + validCatalogFixture))
	require.Empty(t, set)
	require.ErrorIs(t, err, ErrInvalidCatalog)
}

func TestDecodeRejectsUnsupportedSchemaAndMissingRequiredThreshold(t *testing.T) {
	t.Parallel()

	unsupported := strings.Replace(validCatalogFixture, "product-image-policy/v1", "product-image-policy/v2", 1)
	set, err := Decode(strings.NewReader(unsupported))
	require.Empty(t, set)
	require.ErrorIs(t, err, ErrInvalidCatalog)

	missingThreshold := strings.Replace(validCatalogFixture, "      white_canvas_penalty: 0.08\n", "", 1)
	set, err = Decode(strings.NewReader(missingThreshold))
	require.Empty(t, set)
	require.ErrorIs(t, err, ErrInvalidCatalog)
}

func TestDecodeRejectsDuplicatePoliciesThroughResolverContract(t *testing.T) {
	t.Parallel()

	duplicate := strings.Replace(validCatalogFixture, "policies:\n", "policies:\n"+strings.TrimPrefix(validCatalogPolicy, "policies:\n"), 1)
	set, err := Decode(strings.NewReader(duplicate))

	require.Empty(t, set)
	require.ErrorIs(t, err, ErrInvalidCatalog)
}

func TestDecodeRejectsCatalogAboveRawResourceLimit(t *testing.T) {
	t.Parallel()

	set, err := Decode(strings.NewReader(validCatalogFixture + strings.Repeat(" ", maxCatalogDocumentBytes)))

	require.Empty(t, set)
	require.ErrorIs(t, err, ErrInvalidCatalog)
}

func TestLoadEmbeddedReturnsResolverReadyCatalogWithoutRuntimePath(t *testing.T) {
	t.Parallel()

	first, err := LoadEmbedded()
	require.NoError(t, err)
	require.NotEmpty(t, first.Version)
	require.NotEmpty(t, first.Policies)
	resolver, err := imagepolicy.NewResolver(first)
	require.NoError(t, err)
	require.NotNil(t, resolver)

	first.Policies[0].Key.Marketplace = "mutated"
	second, err := LoadEmbedded()
	require.NoError(t, err)
	require.NotEqual(t, "mutated", second.Policies[0].Key.Marketplace)
}

const validCatalogFixture = `schema: product-image-policy/v1
policies:
  - marketplace: marketplace-a
    country: xx
    family: family-a
    scene_category: category-a
    thresholds:
      main_review: 0.61
      white_background_review: 0.72
      white_canvas_penalty: 0.08
    scene_defaults:
      scene_category: category-a
      scene_style: studio
      background_tone: neutral
      composition: centered
      props_level: none
      audience_hint: general
      custom_scene_hint: soft shadow
      slot_role: scene
      slot_brief: show product clearly
      style_reference_ids: [style-a]
`

const validCatalogPolicy = `policies:
  - marketplace: marketplace-a
    country: xx
    family: family-a
    scene_category: category-a
    thresholds:
      main_review: 0.61
      white_background_review: 0.72
      white_canvas_penalty: 0.08
    scene_defaults:
      scene_category: category-a
`
