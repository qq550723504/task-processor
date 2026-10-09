package goods

import (
	"encoding/json"
	"github.com/stretchr/testify/require"
	model "task-processor/internal/marketplace/shein/model"
	"testing"
)

func TestOfficialDraftUsesApplicationTypeInsteadOfSiteStoreType(t *testing.T) {
	input, rules, inventory := officialFixture()
	rules.ApplicationMode = model.ModeSelfOperated
	rules.Sites[0].Sites[0].StoreType = rulePointer(1)
	result := BuildOfficial(input, rules, inventory, nil)
	require.Empty(t, result.Issues, "platform/own site type is unrelated to developer application mode")
	for _, mode := range []model.ApplicationMode{model.ModeSemiManaged, model.ModeFullyManaged} {
		input, rules, inventory = officialFixture()
		rules.ApplicationMode = mode
		rules.Fill.Currency = rulePointer("CNY")
		sku := &input.Product.SKCs[0].SKUs[0]
		sku.Prices = nil
		sku.Cost = &model.CostPrice{Price: "10.50", Currency: "CNY"}
		input.Product.SKCs[0].ShelfWay = "1"
		if mode == model.ModeFullyManaged {
			input.Product.SKCs[0].ShelfRequire = "0"
			rules.Sites = nil
			sku.StopPurchase = rulePointer(1)
		}
		result = BuildOfficial(input, rules, inventory, nil)
		require.Empty(t, result.Issues, string(mode))
		require.True(t, result.ReadyForUpload)
		wire, err := json.Marshal(result.Product)
		require.NoError(t, err)
		require.NotContains(t, string(wire), "price_info_list")
		if mode == model.ModeFullyManaged {
			require.NotContains(t, string(wire), "site_list")
			sku.Stock[0].WarehouseID = "caller-warehouse"
			result = BuildOfficial(input, rules, inventory, nil)
			require.Contains(t, issueFields(result), "skc_list.0.sku_list.0.stock_info_list")
			sku.Stock[0].WarehouseID = ""
			sku.StopPurchase = nil
			result = BuildOfficial(input, rules, inventory, nil)
			require.Contains(t, issueFields(result), "skc_list.0.sku_list.0.stop_purchase")
		}
		sku.Cost.Currency = "USD"
		result = BuildOfficial(input, rules, inventory, nil)
		require.Contains(t, issueFields(result), "skc_list.0.sku_list.0.cost_info")
	}
	input, rules, inventory = officialFixture()
	rules.ApplicationMode = ""
	result = BuildOfficial(input, rules, inventory, nil)
	require.Contains(t, issueFields(result), "application_type")
}
