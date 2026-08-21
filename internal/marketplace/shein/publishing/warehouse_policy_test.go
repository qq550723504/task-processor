package publishing

import "testing"

func TestSubmitPreferredWarehouseCodeUsesFirstConfiguredWarehouse(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		warehouseCode string
		want          string
	}{
		"first csv item": {
			warehouseCode: " WH-CA-1, WH-US-1 ",
			want:          "WH-CA-1",
		},
		"skips blanks": {
			warehouseCode: " , WH-US-1 ",
			want:          "WH-US-1",
		},
		"default sentinel": {
			warehouseCode: " , ",
			want:          "DEFAULT",
		},
	}

	for name, tt := range tests {
		tt := tt
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			if got := SubmitPreferredWarehouseCode(tt.warehouseCode); got != tt.want {
				t.Fatalf("SubmitPreferredWarehouseCode(%q) = %q, want %q", tt.warehouseCode, got, tt.want)
			}
		})
	}
}

func TestSelectWarehouseCodeForSitePrefersMatchingSaleCountry(t *testing.T) {
	t.Parallel()

	warehouses := []WarehouseOption{
		{Code: "WH-EU", SaleCountries: []string{"DE", "FR"}},
		{Code: "WH-US", SaleCountries: []string{"US", "CA"}},
	}

	if got := SelectWarehouseCodeForSite(warehouses, " us "); got != "WH-US" {
		t.Fatalf("SelectWarehouseCodeForSite() = %q, want WH-US", got)
	}
}

func TestSelectWarehouseCodeForSiteFallsBackToFirstWarehouse(t *testing.T) {
	t.Parallel()

	warehouses := []WarehouseOption{
		{Code: " WH-FIRST ", SaleCountries: []string{"DE"}},
		{Code: "WH-SECOND", SaleCountries: []string{"US"}},
	}

	if got := SelectWarehouseCodeForSite(warehouses, "JP"); got != "WH-FIRST" {
		t.Fatalf("SelectWarehouseCodeForSite() = %q, want WH-FIRST", got)
	}
}

func TestSelectWarehouseCodeForSiteReturnsEmptyForNoWarehouses(t *testing.T) {
	t.Parallel()

	if got := SelectWarehouseCodeForSite(nil, "US"); got != "" {
		t.Fatalf("SelectWarehouseCodeForSite(nil) = %q, want empty", got)
	}
}
