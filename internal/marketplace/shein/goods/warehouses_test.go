package goods

import (
	"github.com/stretchr/testify/require"
	model "task-processor/internal/marketplace/shein/model"
	"testing"
)

func TestStockUsesActualMerchantWarehouseSet(t *testing.T) {
	input, rules, inventory := officialFixture()
	rules.Warehouses = []model.Warehouse{{Code: "wh-us", Name: "US warehouse", Type: 1, SaleCountries: []string{"US"}}, {Code: "wh-eu", Name: "EU warehouse", Type: 1, SaleCountries: []string{"DE"}}, {Code: "certified", Name: "Certified", Type: 2, SaleCountries: []string{"US"}}}
	result := BuildOfficial(input, rules, inventory, nil)
	require.Contains(t, issueFields(result), "skc_list.0.sku_list.0.stock_info_list", "one supplied row cannot omit ID when merchant has multiple warehouses")
	stock := &input.Product.SKCs[0].SKUs[0].Stock[0]
	stock.WarehouseID = "wh-us"
	stock.WarehouseName = "US warehouse"
	require.Empty(t, BuildOfficial(input, rules, inventory, nil).Issues)
	for _, id := range []string{"unknown", "wh-eu", "certified"} {
		stock.WarehouseID = id
		require.Contains(t, issueFields(BuildOfficial(input, rules, inventory, nil)), "skc_list.0.sku_list.0.stock_info_list", id)
	}
	stock.WarehouseID = "wh-us"
	stock.WarehouseName = "caller spoofed name"
	require.Contains(t, issueFields(BuildOfficial(input, rules, inventory, nil)), "skc_list.0.sku_list.0.stock_info_list")
	rules.Warehouses = nil
	stock.WarehouseID = ""
	stock.WarehouseName = ""
	rules.Warehouses = []model.Warehouse{{Code: "certified", Name: "Certified", Type: 2, SaleCountries: []string{"US"}}}
	require.Contains(t, issueFields(BuildOfficial(input, rules, inventory, nil)), "skc_list.0.sku_list.0.stock_info_list", "an anonymous row cannot bypass a sole certified warehouse")
	rules.Warehouses = nil
	require.Contains(t, issueFields(BuildOfficial(input, rules, inventory, nil)), "warehouses", "unread warehouse rules never mean single warehouse")
	rules.Warehouses = []model.Warehouse{}
	require.Empty(t, BuildOfficial(input, rules, inventory, nil).Issues, "a confirmed empty warehouse set accepts aggregate stock")
}
