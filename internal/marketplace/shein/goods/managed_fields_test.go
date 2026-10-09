package goods

import (
	"github.com/stretchr/testify/require"
	model "task-processor/internal/marketplace/shein/model"
	"testing"
)

func TestManagedPublicationRequiresExplicitShelfInstructions(t *testing.T) {
	for _, mode := range []model.ApplicationMode{model.ModeSemiManaged, model.ModeFullyManaged} {
		input, rules, inventory := officialFixture()
		rules.ApplicationMode = mode
		rules.Fill.Currency = rulePointer("CNY")
		sku := &input.Product.SKCs[0].SKUs[0]
		sku.Prices = nil
		sku.Cost = &model.CostPrice{Price: "10.50", Currency: "CNY"}
		if mode == model.ModeFullyManaged {
			rules.Sites = nil
			sku.StopPurchase = rulePointer(1)
		}
		result := BuildOfficial(input, rules, inventory, nil)
		require.Contains(t, issueFields(result), "skc_list.0.shelf_way")
		if mode == model.ModeFullyManaged {
			require.Contains(t, issueFields(result), "skc_list.0.shelf_require")
		}
		input.Product.SKCs[0].ShelfWay = "2"
		input.Product.SKCs[0].ShelfRequire = "0"
		result = BuildOfficial(input, rules, inventory, nil)
		require.Contains(t, issueFields(result), "skc_list.0.hope_on_sale_date")
		input.Product.SKCs[0].HopeOnSaleDate = "2026-11-01 09:00:00"
		result = BuildOfficial(input, rules, inventory, nil)
		require.Empty(t, result.Issues)
		input.Product.SKCs[0].HopeOnSaleDate = "2026-02-30 09:00:00"
		require.Contains(t, issueFields(BuildOfficial(input, rules, inventory, nil)), "skc_list.0.hope_on_sale_date")
		input.Product.SKCs[0].ShelfWay = "1"
		require.Contains(t, issueFields(BuildOfficial(input, rules, inventory, nil)), "skc_list.0.hope_on_sale_date", "automatic shelving cannot retain a scheduled date")
	}
}

func TestSampleMustMatchAnActualProductSpecification(t *testing.T) {
	input, rules, inventory := officialFixture()
	rules.Fill.Fields = append(rules.Fill.Fields, model.FillRule{Field: "sample_spec", Show: rulePointer(true), Required: rulePointer(false)})
	input.Product.Sample = &model.SampleInfo{Spec: model.SampleSpec{Main: model.SampleMainSpec{AttributeID: 12, ValueID: 999}}, JudgeType: 2, ReserveFlag: 2, SpotFlag: 1}
	result := BuildOfficial(input, rules, inventory, nil)
	require.Contains(t, issueFields(result), "sample_info.sample_spec")
	input.Product.Sample.Spec.Main.ValueID = 34
	result = BuildOfficial(input, rules, inventory, nil)
	require.Empty(t, result.Issues)
	input.Product.Sample.ReserveFlag = 1
	require.Contains(t, issueFields(BuildOfficial(input, rules, inventory, nil)), "sample_info")
}
