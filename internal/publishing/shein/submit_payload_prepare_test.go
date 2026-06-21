package shein

import (
	"strings"
	"testing"

	sheinproduct "task-processor/internal/shein/api/product"
)

func TestPrepareProductForSubmitNormalizesIdentityAndTransportFields(t *testing.T) {
	t.Parallel()

	product := &sheinproduct.Product{
		SPUName:      "Local title",
		SupplierCode: "MG8089003001",
		SKCList: []sheinproduct.SKC{{
			SKUS: []sheinproduct.SKU{{
				SupplierSKU: "MG8089003001-V245612-TED715457-PETBANDA",
			}},
		}},
	}

	PrepareProductForNewSubmit(product)

	if product.SPUName != "" {
		t.Fatalf("SPUName = %q, want empty", product.SPUName)
	}
	if strings.TrimSpace(product.PointKey) == "" {
		t.Fatal("PointKey is empty, want generated UUID")
	}
	if product.SourceSystem != "listingkit" {
		t.Fatalf("SourceSystem = %q, want listingkit", product.SourceSystem)
	}
	if product.SupplierCode != "MG8089003001-PETBANDA" {
		t.Fatalf("SupplierCode = %q, want MG8089003001-PETBANDA", product.SupplierCode)
	}
	if product.Extra.FromPageID == nil || *product.Extra.FromPageID != "product_publish" {
		t.Fatalf("FromPageID = %+v, want product_publish", product.Extra.FromPageID)
	}
	if product.Extra.ControlPriceData == nil || product.Extra.SPUTag == nil || product.Extra.ConfirmVolumeSKU == nil || product.Extra.ConfirmWeightSKU == nil {
		t.Fatalf("extra transport fields = %+v, want initialized collections", product.Extra)
	}
}

func TestPrepareProductForSubmitAppliesSettingsAndSKUDefaults(t *testing.T) {
	t.Parallel()

	stockCount := 7
	product := &sheinproduct.Product{
		SiteList: []sheinproduct.SiteInfo{{MainSite: "US", SubSiteList: []string{"US"}}},
		SKCList: []sheinproduct.SKC{{
			ImageInfo: sheinproduct.ImageInfo{
				ImageInfoList: []sheinproduct.ImageDetail{{ImageType: 1, ImageSort: 1, ImageURL: "https://img.example/skc.jpg"}},
			},
			SKUS: []sheinproduct.SKU{{
				StockCount: &stockCount,
			}},
		}},
	}

	PrepareProductForSubmit(product, SubmitPayloadSettings{Site: "US", WarehouseCode: "WH-1,WH-2"})

	sku := product.SKCList[0].SKUS[0]
	if len(product.SiteList) == 0 || product.SiteList[0].MainSite != "shein" || len(product.SiteList[0].SubSiteList) == 0 || product.SiteList[0].SubSiteList[0] != defaultSubmitSubSite {
		t.Fatalf("SiteList = %+v, want shein/%s", product.SiteList, defaultSubmitSubSite)
	}
	if product.SKCList[0].ShelfWay != defaultSubmitSKCShelfWay {
		t.Fatalf("ShelfWay = %d, want %d", product.SKCList[0].ShelfWay, defaultSubmitSKCShelfWay)
	}
	if len(sku.StockInfoList) != 1 || sku.StockInfoList[0].MerchantWarehouseCode != "WH-1" || sku.StockInfoList[0].InventoryNum != stockCount {
		t.Fatalf("StockInfoList = %+v, want WH-1 inventory", sku.StockInfoList)
	}
	if sku.StockCount != nil {
		t.Fatalf("StockCount = %v, want nil after stock_info_list normalization", *sku.StockCount)
	}
	if sku.QuantityInfo == nil || sku.QuantityInfo.Quantity == nil || *sku.QuantityInfo.Quantity != 1 {
		t.Fatalf("QuantityInfo = %+v, want default quantity", sku.QuantityInfo)
	}
	if sku.PackageType != 3 || sku.StopPurchase != 1 {
		t.Fatalf("PackageType/StopPurchase = %d/%d, want 3/1", sku.PackageType, sku.StopPurchase)
	}
	if sku.LengthUnit != "Inch" || sku.Length != "1" || sku.Width != "1" || sku.Height != "1" {
		t.Fatalf("dimensions = %q/%q/%q/%q, want Inch/1/1/1", sku.LengthUnit, sku.Length, sku.Width, sku.Height)
	}
}
