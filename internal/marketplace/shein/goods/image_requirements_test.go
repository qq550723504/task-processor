package goods

import (
	"testing"

	"github.com/stretchr/testify/require"
	model "task-processor/internal/marketplace/shein/model"
)

func TestImageRequirementsProjectActualCategoryAndQuantityRules(t *testing.T) {
	input, rules, _ := officialFixture()
	actual, err := ResolveOfficialImageRequirements(input.Product, rules.Fill, nil)
	require.NoError(t, err)
	require.Equal(t, OfficialImageConstraintVersion, actual.Version)
	require.False(t, actual.NewScheme)
	group := func(group string, skc, sku int, requirements OfficialImageRequirements) OfficialImageGroupRequirement {
		for _, item := range requirements.Groups {
			if item.Group == group && item.SKC == skc && item.SKU == sku {
				return item
			}
		}
		t.Fatalf("missing %s/%d/%d", group, skc, sku)
		return OfficialImageGroupRequirement{}
	}
	require.Equal(t, []OfficialImageTypeRequirement{{Type: 1, Minimum: 1, Maximum: 1}, {Type: 2, Minimum: 1, Maximum: 10}, {Type: 5, Minimum: 1, Maximum: 1}, {Type: 6, Minimum: 0, Maximum: 1}}, group("skc", 0, 0, actual).Types)
	require.Equal(t, 0, group("sku", 0, 0, actual).Types[0].Minimum)
	second := input.Product.SKCs[0].SKUs[0]
	input.Product.SKCs[0].SKUs = append(input.Product.SKCs[0].SKUs, second)
	input.Product.SKCs[0].SKUs[0].Quantity = &model.QuantityInfo{Quantity: 2}
	actual, err = ResolveOfficialImageRequirements(input.Product, rules.Fill, nil)
	require.NoError(t, err)
	require.Equal(t, 1, group("sku", 0, 0, actual).Types[0].Minimum)
	require.Equal(t, 1, group("sku", 0, 1, actual).Types[0].Minimum)
	input.Product.SKCs[0].SKUs[0].Quantity = nil
	actual, err = ResolveOfficialImageRequirements(input.Product, rules.Fill, []OfficialImageSlot{{Group: "sku", SKC: 0, SKU: 1, Type: 1, Sort: 1}})
	require.NoError(t, err)
	require.Equal(t, 1, group("sku", 0, 0, actual).Types[0].Minimum, "one SKU selection makes the same SKC's complete SKU set required")
	rules.Fill.Pictures = append(rules.Fill.Pictures, model.PictureRule{Field: "spu_image_detail_show", Enabled: rulePointer(true)})
	_, err = ResolveOfficialImageRequirements(input.Product, rules.Fill, nil)
	require.Error(t, err, "incomplete official scheme cannot fall back to the old rules")
}

func TestImageRequirementsUseSameNativeDimensionsForPlanAndPublishing(t *testing.T) {
	for _, item := range []struct {
		group                    string
		imageType, width, height int
		valid                    bool
	}{
		{"skc", 1, 1024, 1024, true}, {"skc", 2, 1340, 1785, true}, {"skc", 2, 900, 900, true}, {"skc", 1, 2201, 2201, false},
		{"spu", 5, 1024, 1024, false}, {"spu", 5, 1200, 1200, true}, {"skc", 5, 1024, 1024, true},
		{"skc", 6, 1024, 1024, false}, {"skc", 6, 80, 80, true}, {"detail", 7, 1024, 1024, false}, {"detail", 7, 1200, 1600, true}, {"detail", 7, 900, 1200, false},
	} {
		require.Equal(t, item.valid, OfficialImageSizeAllowed(item.group, item.imageType, item.width, item.height), "%+v", item)
	}
}

func TestImageRequirementsRejectUnknownAndConflictingRuleObservations(t *testing.T) {
	input, rules, _ := officialFixture()
	for _, mutation := range []func(*model.FillStandards){
		func(fill *model.FillStandards) { fill.Pictures = nil },
		func(fill *model.FillStandards) { fill.Pictures[1].Enabled = nil },
		func(fill *model.FillStandards) {
			fill.Pictures = append(fill.Pictures, model.PictureRule{Field: "sku_image_required", Enabled: rulePointer(true)})
		},
	} {
		fill := rules.Fill
		fill.Pictures = append([]model.PictureRule(nil), fill.Pictures...)
		mutation(&fill)
		_, err := ResolveOfficialImageRequirements(input.Product, fill, nil)
		require.Error(t, err)
	}
}

func TestImageRequirementsSelectionAndFinalBuilderAgreeOnImagePositions(t *testing.T) {
	input, rules, inventory := officialFixture()
	requirements, err := ResolveOfficialImageRequirements(input.Product, rules.Fill, input.Images)
	require.NoError(t, err)
	dimensions := []OfficialImageDimensions{}
	for _, image := range inventory.Assets {
		dimensions = append(dimensions, OfficialImageDimensions{image.ID, image.Width, image.Height})
	}
	require.Empty(t, requirements.ValidateSelection(input.Images, dimensions))
	for _, mutation := range []func(*OfficialDraftInput){
		func(input *OfficialDraftInput) { input.Images[0].Type = 7 },
		func(input *OfficialDraftInput) { input.Images[0].Sort = 2 },
		func(input *OfficialDraftInput) { input.Images[0].SKC = 1 },
		func(input *OfficialDraftInput) { input.Images[2].Sort = 2 },
		func(input *OfficialDraftInput) { input.Images = input.Images[:2] },
	} {
		changed := input
		changed.Images = append([]OfficialImageSlot(nil), input.Images...)
		mutation(&changed)
		require.NotEmpty(t, requirements.ValidateSelection(changed.Images, dimensions))
		require.Contains(t, issueFields(BuildOfficial(changed, rules, inventory, nil)), "skc_list.0.image_info")
	}
	require.True(t, requirements.AllowsSlot(OfficialImageSlot{Group: "detail", Type: 7, Sort: 1}))
	require.False(t, requirements.AllowsSlot(OfficialImageSlot{Group: "sku", Type: 2, Sort: 2}))
}
