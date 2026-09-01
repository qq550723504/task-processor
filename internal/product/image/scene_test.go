package image

import (
	"encoding/json"
	"math"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestMergeSceneOptionsNormalizesReferencesWithoutAliasingInputs(t *testing.T) {
	t.Parallel()

	base := &SceneOptions{
		SceneStyle:        " studio ",
		BackgroundTone:    " bright ",
		StyleReferenceIDs: []string{"style-1"},
	}
	override := &SceneOptions{
		SceneStyle:        " lifestyle ",
		StyleReferenceIDs: []string{" style-2 ", "style-2", "", "style-3"},
	}

	got, err := MergeSceneOptions(base, override)
	require.NoError(t, err)
	require.Equal(t, "lifestyle", got.SceneStyle)
	require.Equal(t, "bright", got.BackgroundTone)
	require.Equal(t, []string{"style-2", "style-3"}, got.StyleReferenceIDs)

	got.StyleReferenceIDs[0] = "mutated"
	require.Equal(t, []string{"style-1"}, base.StyleReferenceIDs)
	require.Equal(t, " style-2 ", override.StyleReferenceIDs[0])
}

func TestMergeSceneOptionsRejectsReferenceResourceLimitViolations(t *testing.T) {
	t.Parallel()

	tooMany := make([]string, 17)
	for index := range tooMany {
		tooMany[index] = string(rune(0x4000 + index))
	}
	for name, references := range map[string][]string{
		"too many": tooMany,
		"too long": {strings.Repeat("x", 8193)},
	} {
		t.Run(name, func(t *testing.T) {
			_, err := MergeSceneOptions(nil, &SceneOptions{StyleReferenceIDs: references})
			require.ErrorIs(t, err, ErrInputInvalid)
		})
	}
}

func TestResolveSceneProfileReturnsIndependentCopiesAndRejectsUnknownProfiles(t *testing.T) {
	t.Parallel()

	first, err := ResolveSceneProfile("shein_selling_point")
	require.NoError(t, err)
	require.Equal(t, "selling_point", first.VisualMode)
	require.NotEmpty(t, first.CopySlots)
	first.CopySlots[0] = "mutated"

	second, err := ResolveSceneProfile("shein_selling_point")
	require.NoError(t, err)
	require.NotEqual(t, "mutated", second.CopySlots[0])

	defaultProfile, err := ResolveSceneProfile("")
	require.NoError(t, err)
	require.Equal(t, "local_canvas_default", defaultProfile.Name)

	_, err = ResolveSceneProfile("unknown-profile")
	require.ErrorIs(t, err, ErrCapabilityUnsupported)
}

func TestNormalizeSceneProfileRejectsNonFiniteNumbers(t *testing.T) {
	t.Parallel()

	valid := sceneProfileYAML{
		Name: "test", SubjectScale: 0.5, BlurRadius: 1,
		BackgroundBrightness: 1, BackgroundContrast: 1,
		LayoutVariant: "center", VisualMode: "catalog",
	}
	for name, mutate := range map[string]func(*sceneProfileYAML){
		"blur NaN":               func(profile *sceneProfileYAML) { profile.BlurRadius = math.NaN() },
		"blur infinity":          func(profile *sceneProfileYAML) { profile.BlurRadius = math.Inf(1) },
		"brightness NaN":         func(profile *sceneProfileYAML) { profile.BackgroundBrightness = math.NaN() },
		"brightness infinity":    func(profile *sceneProfileYAML) { profile.BackgroundBrightness = math.Inf(-1) },
		"contrast NaN":           func(profile *sceneProfileYAML) { profile.BackgroundContrast = math.NaN() },
		"contrast infinity":      func(profile *sceneProfileYAML) { profile.BackgroundContrast = math.Inf(1) },
		"subject scale NaN":      func(profile *sceneProfileYAML) { profile.SubjectScale = math.NaN() },
		"subject scale infinity": func(profile *sceneProfileYAML) { profile.SubjectScale = math.Inf(1) },
	} {
		t.Run(name, func(t *testing.T) {
			profile := valid
			mutate(&profile)
			_, err := normalizeSceneProfile(profile)
			require.ErrorIs(t, err, ErrInputInvalid)
		})
	}
}

func TestBuildScenePlanProducesDeterministicTypedSellingPointLayers(t *testing.T) {
	t.Parallel()

	firstProduct := validProductContext()
	firstProduct.Title = "Portable Speaker"
	firstProduct.ProductType = "Bluetooth Speaker"
	firstProduct.Attributes = map[string]string{
		"Size":     "12 x 8 x 3 cm",
		"Feature":  "Water Resistant",
		"Material": "ABS",
		"Color":    "Black",
	}
	secondProduct := firstProduct
	secondProduct.Attributes = map[string]string{
		"Color":    "Black",
		"Material": "ABS",
		"Feature":  "Water Resistant",
		"Size":     "12 x 8 x 3 cm",
	}

	first, err := BuildScenePlan(ScenePlanRequest{ProfileName: "shein_selling_point", Product: firstProduct})
	require.NoError(t, err)
	second, err := BuildScenePlan(ScenePlanRequest{ProfileName: "shein_selling_point", Product: secondProduct})
	require.NoError(t, err)
	require.Equal(t, first, second)
	require.Equal(t, "selling_point", first.VisualMode)
	require.NotEmpty(t, first.Content)
	require.NotEmpty(t, first.Layers)

	encoded, err := json.Marshal(first)
	require.NoError(t, err)
	require.Contains(t, string(encoded), "Portable Speaker")
	require.Contains(t, string(encoded), "Size: 12 x 8 x 3 cm")
	require.Contains(t, string(encoded), "Color: Black")
	require.NotContains(t, string(encoded), "<svg")

	for _, layer := range first.Layers {
		require.Greater(t, layer.Bounds.Width, 0.0)
		require.Greater(t, layer.Bounds.Height, 0.0)
	}
}

func TestBuildScenePlanInfersOnlyProductCategoryAndDoesNotInventMarketplacePolicy(t *testing.T) {
	t.Parallel()

	product := validProductContext()
	product.ProductType = "running sneaker"
	plan, err := BuildScenePlan(ScenePlanRequest{
		ProfileName: "local_canvas_default",
		Product:     product,
		Options:     SceneOptions{SceneStyle: "studio"},
	})
	require.NoError(t, err)
	require.Equal(t, "shoes", plan.Options.SceneCategory)
	require.Equal(t, "studio", plan.Options.SceneStyle)
	require.Empty(t, plan.Options.BackgroundTone, "marketplace defaults must be injected outside product/image")
}

func TestBuildScenePlanRejectsAbnormalProductInput(t *testing.T) {
	t.Parallel()

	for name, mutate := range map[string]func(*ScenePlanRequest){
		"missing product key":      func(request *ScenePlanRequest) { request.Product.ProductKey = "" },
		"noncanonical product key": func(request *ScenePlanRequest) { request.Product.ProductKey = " product-1 " },
		"too many attributes": func(request *ScenePlanRequest) {
			request.Product.Attributes = make(map[string]string, 257)
			for i := 0; i < 257; i++ {
				request.Product.Attributes[string(rune(0x3000+i))] = "value"
			}
		},
	} {
		t.Run(name, func(t *testing.T) {
			request := ScenePlanRequest{ProfileName: "local_canvas_default", Product: validProductContext()}
			mutate(&request)
			_, err := BuildScenePlan(request)
			require.ErrorIs(t, err, ErrInputInvalid)
		})
	}
}

func TestScenePlanResultDoesNotAliasRequest(t *testing.T) {
	t.Parallel()

	request := ScenePlanRequest{
		ProfileName: "shein_selling_point",
		Product:     validProductContext(),
		Options:     SceneOptions{StyleReferenceIDs: []string{"style-1"}},
	}
	plan, err := BuildScenePlan(request)
	require.NoError(t, err)

	request.Product.Attributes["material"] = "mutated"
	request.Options.StyleReferenceIDs[0] = "mutated"
	require.Equal(t, "style-1", plan.Options.StyleReferenceIDs[0])
	for _, content := range plan.Content {
		require.NotContains(t, content.Text, "mutated")
	}
}
