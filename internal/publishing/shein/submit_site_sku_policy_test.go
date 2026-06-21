package shein

import (
	"testing"

	sheinproduct "task-processor/internal/shein/api/product"
)

func TestEnsureSubmitSitesNormalizesUSDefaults(t *testing.T) {
	t.Parallel()

	product := &sheinproduct.Product{
		SiteList: []sheinproduct.SiteInfo{{
			MainSite:    "US",
			SubSiteList: []string{"US"},
		}},
	}

	EnsureSubmitSites(product, SubmitPayloadSettings{Site: "US"})

	if len(product.SiteList) != 1 {
		t.Fatalf("site list len = %d, want 1", len(product.SiteList))
	}
	if product.SiteList[0].MainSite != "shein" {
		t.Fatalf("main site = %q, want shein", product.SiteList[0].MainSite)
	}
	if len(product.SiteList[0].SubSiteList) != 1 || product.SiteList[0].SubSiteList[0] != "shein-us" {
		t.Fatalf("sub sites = %#v, want shein-us", product.SiteList[0].SubSiteList)
	}
}

func TestEnsureSubmitSKUsNormalizesStockQuantityDimensionsAndPriceSite(t *testing.T) {
	t.Parallel()

	stockCount := 7
	supplierCode := "SKC-1"
	product := &sheinproduct.Product{
		SKCList: []sheinproduct.SKC{{
			SupplierCode: &supplierCode,
			SaleAttribute: sheinproduct.SaleAttribute{
				PreFillSpec:        true,
				IsSPPSaleAttribute: true,
			},
			SKUS: []sheinproduct.SKU{{
				StockCount:    &stockCount,
				Weight:        1,
				WeightUnit:    "lb",
				PriceInfoList: []sheinproduct.PriceInfo{{SubSite: "US"}},
			}},
		}},
	}

	EnsureSubmitSKUs(product, SubmitPayloadSettings{WarehouseCode: "WH-CA-1,WH-US-1"})

	skc := product.SKCList[0]
	if skc.SupplierCode != nil {
		t.Fatalf("supplier code = %v, want nil", *skc.SupplierCode)
	}
	if skc.SaleAttribute.PreFillSpec || skc.SaleAttribute.IsSPPSaleAttribute {
		t.Fatalf("sale attribute flags = %+v, want disabled", skc.SaleAttribute)
	}
	if skc.ShelfWay != defaultSubmitSKCShelfWay {
		t.Fatalf("shelf way = %d, want %d", skc.ShelfWay, defaultSubmitSKCShelfWay)
	}

	sku := skc.SKUS[0]
	if sku.StockCount != nil {
		t.Fatalf("stock count = %v, want nil", *sku.StockCount)
	}
	if len(sku.StockInfoList) != 1 || sku.StockInfoList[0].InventoryNum != 7 || sku.StockInfoList[0].MerchantWarehouseCode != "WH-CA-1" {
		t.Fatalf("stock info = %+v, want inventory 7 in WH-CA-1", sku.StockInfoList)
	}
	if sku.QuantityInfo == nil || *sku.QuantityInfo.Quantity != 1 || *sku.QuantityInfo.QuantityType != 1 || *sku.QuantityInfo.QuantityUnit != 1 {
		t.Fatalf("quantity info = %+v, want all ones", sku.QuantityInfo)
	}
	if sku.PackageType != 3 || sku.StopPurchase != 1 {
		t.Fatalf("package/stop = %d/%d, want 3/1", sku.PackageType, sku.StopPurchase)
	}
	if sku.LengthUnit != "Inch" || sku.Length != "1" || sku.Width != "1" || sku.Height != "1" {
		t.Fatalf("dimensions = %q/%q/%q/%q, want Inch/1/1/1", sku.LengthUnit, sku.Length, sku.Width, sku.Height)
	}
	if sku.WeightUnit != "g" || sku.Weight != 453.59 {
		t.Fatalf("weight = %.2f %s, want 453.59 g", sku.Weight, sku.WeightUnit)
	}
	if len(sku.PriceInfoList) != 1 || sku.PriceInfoList[0].SubSite != "shein-us" {
		t.Fatalf("price site = %+v, want shein-us", sku.PriceInfoList)
	}
	if sku.CompetingCostPriceImages == nil || sku.SaleAttributeList == nil {
		t.Fatalf("normalized slices should be non-nil: %+v", sku)
	}
}

func TestNormalizeSubmitWeightClampsAndConvertsUnits(t *testing.T) {
	t.Parallel()

	sku := &sheinproduct.SKU{Weight: 2, WeightUnit: "kg"}
	NormalizeSubmitWeight(sku)
	if sku.Weight != 2000 || sku.WeightUnit != "g" {
		t.Fatalf("weight = %.2f %s, want 2000 g", sku.Weight, sku.WeightUnit)
	}

	sku = &sheinproduct.SKU{Weight: 0, WeightUnit: "g"}
	NormalizeSubmitWeight(sku)
	if sku.Weight != minSubmitWeightGrams || sku.WeightUnit != "g" {
		t.Fatalf("min weight = %.2f %s, want %.2f g", sku.Weight, sku.WeightUnit, minSubmitWeightGrams)
	}
}
