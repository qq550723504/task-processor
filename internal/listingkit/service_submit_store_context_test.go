package listingkit

import (
	"testing"

	sheinwarehouse "task-processor/internal/shein/api/warehouse"
)

func TestPickSheinWarehouseCodePrefersMatchingSaleCountry(t *testing.T) {
	t.Parallel()

	warehouses := &sheinwarehouse.WarehouseResponse{
		Data: []sheinwarehouse.Warehouse{
			{WarehouseCode: "WH-EU", SaleCountryList: []string{"DE", "FR"}},
			{WarehouseCode: "WH-US", SaleCountryList: []string{"US", "CA"}},
		},
	}

	if got := pickSheinWarehouseCode(warehouses, "US"); got != "WH-US" {
		t.Fatalf("pick warehouse = %q, want WH-US", got)
	}
}

func TestPickSheinWarehouseCodeFallsBackToFirstWarehouse(t *testing.T) {
	t.Parallel()

	warehouses := &sheinwarehouse.WarehouseResponse{
		Data: []sheinwarehouse.Warehouse{
			{WarehouseCode: "WH-FIRST", SaleCountryList: []string{"DE"}},
			{WarehouseCode: "WH-SECOND", SaleCountryList: []string{"US"}},
		},
	}

	if got := pickSheinWarehouseCode(warehouses, "JP"); got != "WH-FIRST" {
		t.Fatalf("pick warehouse = %q, want WH-FIRST", got)
	}
}
