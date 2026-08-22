package common

import "testing"

func TestSelectWarehouseCodeForSitePrefersMatchingSaleCountry(t *testing.T) {
	warehouses := []WarehouseOption{
		{Code: "WH-EU", SaleCountries: []string{"DE", "FR"}},
		{Code: "WH-US", SaleCountries: []string{"US", "CA"}},
	}
	if got := SelectWarehouseCodeForSite(warehouses, " us "); got != "WH-US" {
		t.Fatalf("SelectWarehouseCodeForSite() = %q, want WH-US", got)
	}
}

func TestSelectWarehouseCodeForSiteFallsBackToFirstWarehouse(t *testing.T) {
	warehouses := []WarehouseOption{
		{Code: "WH-FIRST", SaleCountries: []string{"DE"}},
		{Code: "WH-SECOND", SaleCountries: []string{"US"}},
	}
	if got := SelectWarehouseCodeForSite(warehouses, "JP"); got != "WH-FIRST" {
		t.Fatalf("SelectWarehouseCodeForSite() = %q, want WH-FIRST", got)
	}
}

func TestSelectWarehouseCodeForSiteReturnsEmptyForNoWarehouses(t *testing.T) {
	if got := SelectWarehouseCodeForSite(nil, "US"); got != "" {
		t.Fatalf("SelectWarehouseCodeForSite(nil) = %q, want empty", got)
	}
}
