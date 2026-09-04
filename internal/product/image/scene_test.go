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

func TestMergeSceneOptionsPreflightsRawScalarLimitsBeforeNormalization(t *testing.T) {
	t.Parallel()

	const designMaxImageStringBytes = 8 << 10
	const scalarBytes = 8 << 10
	for name, options := range map[string]SceneOptions{
		"overlong whitespace": {SceneStyle: strings.Repeat(" ", designMaxImageStringBytes+1)},
		"aggregate scalar bytes": {
			SceneCategory: strings.Repeat("a", scalarBytes), SceneStyle: strings.Repeat("b", scalarBytes),
			BackgroundTone: strings.Repeat("c", scalarBytes), Composition: strings.Repeat("d", scalarBytes),
			PropsLevel: strings.Repeat("e", scalarBytes), AudienceHint: strings.Repeat("f", scalarBytes),
			CustomSceneHint: strings.Repeat("g", scalarBytes), SlotRole: strings.Repeat("h", scalarBytes),
			SlotBrief: strings.Repeat("i", scalarBytes),
		},
	} {
		t.Run(name, func(t *testing.T) {
			got, err := MergeSceneOptions(nil, &options)
			require.ErrorIs(t, err, ErrInputInvalid)
			require.Nil(t, got)
		})
	}
}

func TestMergeSceneOptionsRejectsAggregateCreatedAcrossBaseAndOverride(t *testing.T) {
	t.Parallel()

	const scalarBytes = 8 << 10
	base := &SceneOptions{
		SceneCategory: strings.Repeat("a", scalarBytes), SceneStyle: strings.Repeat("b", scalarBytes),
		BackgroundTone: strings.Repeat("c", scalarBytes), Composition: strings.Repeat("d", scalarBytes),
	}
	override := &SceneOptions{
		PropsLevel: strings.Repeat("e", scalarBytes), AudienceHint: strings.Repeat("f", scalarBytes),
		CustomSceneHint: strings.Repeat("g", scalarBytes), SlotRole: strings.Repeat("h", scalarBytes),
		SlotBrief: strings.Repeat("i", scalarBytes),
	}

	got, err := MergeSceneOptions(base, override)
	require.ErrorIs(t, err, ErrInputInvalid)
	require.Nil(t, got)
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

func TestResolveSceneProfileRejectsOverlongRawNameBeforeNormalization(t *testing.T) {
	t.Parallel()

	_, err := ResolveSceneProfile(strings.Repeat(" ", (8<<10)+1))
	require.ErrorIs(t, err, ErrInputInvalid)
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

func TestBuildSceneLayoutPreservesEditorialAndSpecGeometry(t *testing.T) {
	t.Parallel()

	geometry := SceneLayoutInput{CanvasSize: 1600, SubjectWidth: 800, SubjectHeight: 800}
	editorial := SceneProfile{
		Group: "editorial/model", LayoutVariant: "hero_center", VisualMode: "editorial",
		MaxCopyLines: 2, MaxBadges: 1, MeasurementMode: "single_axis", DetailAnchorMode: "single_anchor",
	}
	spec := SceneProfile{
		Group: "selling_point/size/spec/detail", LayoutVariant: "right_info_panel", VisualMode: "spec_support",
		MaxCopyLines: 4, MaxBadges: 3, MeasurementMode: "dual_axis", DetailAnchorMode: "dual_anchor",
	}

	editorialLayout, err := BuildSceneLayout(editorial, geometry)
	require.NoError(t, err)
	require.Equal(t, SceneLayout{
		CardWidth: 960, CardHeight: 880, CardPoint: ScenePoint{X: 320, Y: 360},
		SubjectPoint:  ScenePoint{X: 400, Y: 400},
		SubjectBounds: ScenePixelBounds{X: 400, Y: 400, Width: 800, Height: 800},
		CardOpacity:   0.82, Engine: "preset_layout_v1", QualityGrade: "ideal",
	}, editorialLayout)

	specLayout, err := BuildSceneLayout(spec, geometry)
	require.NoError(t, err)
	require.Equal(t, SceneLayout{
		CardWidth: 1034, CardHeight: 1159, CardPoint: ScenePoint{X: 124, Y: 187},
		SubjectPoint:  ScenePoint{X: 144, Y: 333},
		SubjectBounds: ScenePixelBounds{X: 144, Y: 333, Width: 800, Height: 800},
		CardOpacity:   0.91, Engine: "preset_layout_v1", QualityGrade: "ideal",
	}, specLayout)
	require.Greater(t, specLayout.CardWidth, editorialLayout.CardWidth)
	require.Less(t, specLayout.SubjectPoint.X, editorialLayout.SubjectPoint.X)
}

func TestBuildSceneLayoutPreservesDualAxisUpwardShift(t *testing.T) {
	t.Parallel()

	profile := SceneProfile{
		Group: "selling_point/size/spec/detail", LayoutVariant: "spec_sheet", VisualMode: "spec_support",
		MaxCopyLines: 2, MaxBadges: 1, MeasurementMode: "single_axis", DetailAnchorMode: "single_anchor",
	}
	geometry := SceneLayoutInput{CanvasSize: 1600, SubjectWidth: 700, SubjectHeight: 900}
	singleAxis, err := BuildSceneLayout(profile, geometry)
	require.NoError(t, err)
	profile.MeasurementMode = "dual_axis"
	dualAxis, err := BuildSceneLayout(profile, geometry)
	require.NoError(t, err)

	require.Equal(t, 350, singleAxis.SubjectPoint.Y)
	require.Equal(t, 217, dualAxis.SubjectPoint.Y)
	require.Less(t, dualAxis.SubjectPoint.Y, singleAxis.SubjectPoint.Y)
}

func TestBuildSceneLayoutUsesIndependentSellingPointBranch(t *testing.T) {
	t.Parallel()

	geometry := SceneLayoutInput{CanvasSize: 1600, SubjectWidth: 720, SubjectHeight: 720}
	profile := SceneProfile{
		Group: "selling_point/size/spec/detail", LayoutVariant: "selling_point_grid", VisualMode: "selling_point",
		MaxCopyLines: 4, MaxBadges: 3, MeasurementMode: "dual_axis", DetailAnchorMode: "dual_anchor",
	}
	sellingPoint, err := BuildSceneLayout(profile, geometry)
	require.NoError(t, err)
	profile.VisualMode = "scene"
	scene, err := BuildSceneLayout(profile, geometry)
	require.NoError(t, err)

	require.Equal(t, "selling_point_layout_v1", sellingPoint.Engine)
	require.Equal(t, "preset_layout_v1", scene.Engine)
	require.Equal(t, 0.93, sellingPoint.CardOpacity)
	require.Less(t, sellingPoint.CardPoint.X, scene.CardPoint.X)
	require.Less(t, sellingPoint.SubjectPoint.X, scene.SubjectPoint.X)
}

func TestBuildSceneLayoutUsesResolvedPresetGroupsForOpacity(t *testing.T) {
	t.Parallel()

	for name, expected := range map[string]float64{
		"shein_model_editorial":   0.82,
		"shein_lifestyle_gallery": 0.88,
		"walmart_spec_support":    0.91,
		"shein_selling_point":     0.93,
	} {
		t.Run(name, func(t *testing.T) {
			profile, err := ResolveSceneProfile(name)
			require.NoError(t, err)
			layout, err := BuildSceneLayout(profile, SceneLayoutInput{CanvasSize: 1600, SubjectWidth: 720, SubjectHeight: 720})
			require.NoError(t, err)
			require.Equal(t, expected, layout.CardOpacity)
		})
	}
}

func TestBuildSceneLayoutRejectsUnsafeGeometryAndArithmeticInputs(t *testing.T) {
	t.Parallel()

	profile, err := ResolveSceneProfile("local_canvas_default")
	require.NoError(t, err)
	for name, mutate := range map[string]func(*SceneProfile, *SceneLayoutInput){
		"canvas below minimum": func(_ *SceneProfile, input *SceneLayoutInput) { input.CanvasSize = 63 },
		"canvas above maximum": func(_ *SceneProfile, input *SceneLayoutInput) { input.CanvasSize = 8193 },
		"zero subject width":   func(_ *SceneProfile, input *SceneLayoutInput) { input.SubjectWidth = 0 },
		"negative height":      func(_ *SceneProfile, input *SceneLayoutInput) { input.SubjectHeight = -1 },
		"subject wider":        func(_ *SceneProfile, input *SceneLayoutInput) { input.SubjectWidth = 1601 },
		"subject taller":       func(_ *SceneProfile, input *SceneLayoutInput) { input.SubjectHeight = 1601 },
		"copy count overflow": func(profile *SceneProfile, _ *SceneLayoutInput) {
			profile.MaxCopyLines = int(^uint(0) >> 1)
		},
		"badge count overflow": func(profile *SceneProfile, _ *SceneLayoutInput) {
			profile.MaxBadges = int(^uint(0) >> 1)
		},
	} {
		t.Run(name, func(t *testing.T) {
			candidateProfile := cloneSceneProfile(profile)
			input := SceneLayoutInput{CanvasSize: 1600, SubjectWidth: 800, SubjectHeight: 800}
			mutate(&candidateProfile, &input)
			got, err := BuildSceneLayout(candidateProfile, input)
			require.ErrorIs(t, err, ErrInputInvalid)
			require.Equal(t, SceneLayout{}, got)
		})
	}
}

func TestBuildSceneLayoutContainsBoundaryGeometryAndIsDeterministic(t *testing.T) {
	t.Parallel()

	profile, err := ResolveSceneProfile("shein_selling_point")
	require.NoError(t, err)
	original := cloneSceneProfile(profile)
	for name, input := range map[string]SceneLayoutInput{
		"minimum": {CanvasSize: 64, SubjectWidth: 1, SubjectHeight: 1},
		"maximum": {CanvasSize: 8192, SubjectWidth: 8192, SubjectHeight: 8192},
	} {
		t.Run(name, func(t *testing.T) {
			first, err := BuildSceneLayout(profile, input)
			require.NoError(t, err)
			second, err := BuildSceneLayout(profile, input)
			require.NoError(t, err)
			require.Equal(t, first, second)
			requireSceneLayoutWithinCanvas(t, first, input)
		})
	}
	require.Equal(t, original, profile)
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

	geometry := SceneLayoutInput{CanvasSize: 1600, SubjectWidth: 720, SubjectHeight: 720}
	first, err := BuildScenePlan(ScenePlanRequest{ProfileName: "shein_selling_point", Product: firstProduct, Geometry: geometry})
	require.NoError(t, err)
	second, err := BuildScenePlan(ScenePlanRequest{ProfileName: "shein_selling_point", Product: secondProduct, Geometry: geometry})
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
		for _, value := range []float64{layer.Bounds.X, layer.Bounds.Y, layer.Bounds.Width, layer.Bounds.Height} {
			require.False(t, math.IsNaN(value) || math.IsInf(value, 0))
		}
		require.GreaterOrEqual(t, layer.Bounds.X, 0.0)
		require.GreaterOrEqual(t, layer.Bounds.Y, 0.0)
		require.LessOrEqual(t, layer.Bounds.X+layer.Bounds.Width, 1.0)
		require.LessOrEqual(t, layer.Bounds.Y+layer.Bounds.Height, 1.0)
		require.Greater(t, layer.Opacity, 0.0)
		require.LessOrEqual(t, layer.Opacity, 1.0)
	}
	require.Equal(t, SceneLayout{
		CardWidth: 968, CardHeight: 1010, CardPoint: ScenePoint{X: 114, Y: 295},
		SubjectPoint:  ScenePoint{X: 248, Y: 497},
		SubjectBounds: ScenePixelBounds{X: 248, Y: 497, Width: 720, Height: 720},
		CardOpacity:   0.93, Engine: "selling_point_layout_v1", QualityGrade: "ideal",
	}, first.Layout)
	card := sceneLayerByID(t, first.Layers, "card")
	require.InDelta(t, 0.07125, card.Bounds.X, 0.0000001)
	require.InDelta(t, 0.184375, card.Bounds.Y, 0.0000001)
	require.InDelta(t, 0.605, card.Bounds.Width, 0.0000001)
	require.InDelta(t, 0.63125, card.Bounds.Height, 0.0000001)
	require.Equal(t, 0.93, card.Opacity)
	subject := sceneLayerByID(t, first.Layers, "subject")
	require.InDelta(t, 0.155, subject.Bounds.X, 0.0000001)
	require.InDelta(t, 0.310625, subject.Bounds.Y, 0.0000001)
	require.InDelta(t, 0.45, subject.Bounds.Width, 0.0000001)
	require.InDelta(t, 0.45, subject.Bounds.Height, 0.0000001)
}

func TestBuildScenePlanInfersOnlyProductCategoryAndDoesNotInventMarketplacePolicy(t *testing.T) {
	t.Parallel()

	product := validProductContext()
	product.ProductType = "running sneaker"
	plan, err := BuildScenePlan(ScenePlanRequest{
		ProfileName: "local_canvas_default",
		Product:     product,
		Options:     SceneOptions{SceneStyle: "studio"},
		Geometry:    SceneLayoutInput{CanvasSize: 1600, SubjectWidth: 720, SubjectHeight: 720},
	})
	require.NoError(t, err)
	require.Equal(t, "shoes", plan.Options.SceneCategory)
	require.Equal(t, "studio", plan.Options.SceneStyle)
	require.Empty(t, plan.Options.BackgroundTone, "marketplace defaults must be injected outside product/image")
}

func TestBuildScenePlanRejectsAbnormalProductInput(t *testing.T) {
	t.Parallel()

	for name, mutate := range map[string]func(*ScenePlanRequest){
		"missing geometry":         func(request *ScenePlanRequest) { request.Geometry = SceneLayoutInput{} },
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
			request := ScenePlanRequest{
				ProfileName: "local_canvas_default", Product: validProductContext(),
				Geometry: SceneLayoutInput{CanvasSize: 1600, SubjectWidth: 720, SubjectHeight: 720},
			}
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
		Geometry:    SceneLayoutInput{CanvasSize: 1600, SubjectWidth: 720, SubjectHeight: 720},
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

func requireSceneLayoutWithinCanvas(t *testing.T, layout SceneLayout, input SceneLayoutInput) {
	t.Helper()
	require.Greater(t, layout.CardWidth, 0)
	require.Greater(t, layout.CardHeight, 0)
	require.GreaterOrEqual(t, layout.CardPoint.X, 0)
	require.GreaterOrEqual(t, layout.CardPoint.Y, 0)
	require.LessOrEqual(t, layout.CardPoint.X+layout.CardWidth, input.CanvasSize)
	require.LessOrEqual(t, layout.CardPoint.Y+layout.CardHeight, input.CanvasSize)
	require.Equal(t, layout.SubjectPoint.X, layout.SubjectBounds.X)
	require.Equal(t, layout.SubjectPoint.Y, layout.SubjectBounds.Y)
	require.Equal(t, input.SubjectWidth, layout.SubjectBounds.Width)
	require.Equal(t, input.SubjectHeight, layout.SubjectBounds.Height)
	require.GreaterOrEqual(t, layout.SubjectBounds.X, 0)
	require.GreaterOrEqual(t, layout.SubjectBounds.Y, 0)
	require.LessOrEqual(t, layout.SubjectBounds.X+layout.SubjectBounds.Width, input.CanvasSize)
	require.LessOrEqual(t, layout.SubjectBounds.Y+layout.SubjectBounds.Height, input.CanvasSize)
	require.False(t, math.IsNaN(layout.CardOpacity) || math.IsInf(layout.CardOpacity, 0))
	require.Greater(t, layout.CardOpacity, 0.0)
	require.LessOrEqual(t, layout.CardOpacity, 1.0)
}

func sceneLayerByID(t *testing.T, layers []SceneLayer, id string) SceneLayer {
	t.Helper()
	for _, layer := range layers {
		if layer.ID == id {
			return layer
		}
	}
	t.Fatalf("scene layer %q not found", id)
	return SceneLayer{}
}
