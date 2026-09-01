package image

import (
	"image"
	"image/color"
	"reflect"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestIsWhiteBackgroundUsesAvailableCornersForTinyImages(t *testing.T) {
	t.Parallel()

	tinyWhite := image.NewNRGBA(image.Rect(0, 0, 2, 2))
	for y := 0; y < 2; y++ {
		for x := 0; x < 2; x++ {
			tinyWhite.Set(x, y, color.NRGBA{R: 245, G: 245, B: 245, A: 255})
		}
	}
	require.True(t, IsWhiteBackground(tinyWhite))

	tinyWhite.Set(0, 0, color.NRGBA{R: 20, G: 20, B: 20, A: 255})
	tinyWhite.Set(1, 0, color.NRGBA{R: 20, G: 20, B: 20, A: 255})
	require.False(t, IsWhiteBackground(tinyWhite))
	require.False(t, IsWhiteBackground(nil))
}

func TestAssessIPRiskIsDeterministicAndDoesNotMutateAudits(t *testing.T) {
	t.Parallel()

	audits := []ImageAudit{
		{ImageURL: "https://cdn.example/nike-shoe.png", HasLogo: true, PrimaryObject: "shoe"},
		{ImageURL: "https://cdn.example/plain.png", HasOverlayText: true, HasPromoBadge: true},
	}
	before := append([]ImageAudit(nil), audits...)
	before[0].Issues = append([]string(nil), audits[0].Issues...)

	first := AssessIPRisk("https://detail.1688.com/offer/1", audits)
	second := AssessIPRisk("https://detail.1688.com/offer/1", []ImageAudit{audits[1], audits[0]})

	require.Equal(t, RiskHigh, first.Level)
	require.Equal(t, 1.0, first.Score)
	require.Equal(t, first, second)
	require.True(t, reflect.DeepEqual(before, audits), "AssessIPRisk mutated its input")
	require.IsNonDecreasing(t, first.Reasons)
}

func TestNormalizeGenerationMetadataDefensivelyCopiesAndCanonicalizesValues(t *testing.T) {
	t.Parallel()

	input := GenerationMetadata{
		Capability:      " render_scene ",
		ModelFamily:     " image-model ",
		InvocationID:    " invocation-1 ",
		PromptReference: " scene/default ",
		PromptVersion:   " v2 ",
		Values:          map[string]string{"seed": " 42 ", "empty": " "},
	}

	got, err := NormalizeGenerationMetadata(input)
	require.NoError(t, err)
	require.Equal(t, "render_scene", got.Capability)
	require.Equal(t, "image-model", got.ModelFamily)
	require.Equal(t, map[string]string{"seed": "42"}, got.Values)

	input.Values["seed"] = "changed"
	require.Equal(t, "42", got.Values["seed"])
}

func TestNormalizeGenerationMetadataRejectsOversizedValues(t *testing.T) {
	t.Parallel()

	values := make(map[string]string, 129)
	for i := 0; i < 129; i++ {
		values[string(rune(0x2000+i))] = "value"
	}
	_, err := NormalizeGenerationMetadata(GenerationMetadata{Capability: "render_scene", Values: values})
	require.ErrorIs(t, err, ErrInputInvalid)
}

func TestNormalizeGenerationMetadataRejectsKeysThatConflictAfterNormalization(t *testing.T) {
	t.Parallel()

	_, err := NormalizeGenerationMetadata(GenerationMetadata{
		Capability: "render_scene",
		Values: map[string]string{
			" seed ": "41",
			"seed":   "42",
		},
	})
	require.ErrorIs(t, err, ErrInputInvalid)
}
