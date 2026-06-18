package publish

import (
	"strings"
	"testing"

	sheinproduct "task-processor/internal/shein/api/product"
)

func TestPublishProductValidatorRejectsDuplicateSupplierSKUs(t *testing.T) {
	validator := NewPublishProductValidator()
	input := &ValidationInput{
		ProductData: &sheinproduct.Product{
			CategoryID:            100,
			MultiLanguageNameList: []sheinproduct.LanguageContent{{Language: "en", Name: "Shoes"}},
			MultiLanguageDescList: []sheinproduct.LanguageContent{{Language: "en", Name: "Desc"}},
			ProductAttributeList: []sheinproduct.ProductAttribute{
				{AttributeID: 101, AttributeExtraValue: "Leather"},
			},
			SKCList: []sheinproduct.SKC{
				{
					SaleAttribute: sheinproduct.SaleAttribute{AttributeID: 501, AttributeValueID: 11},
					SKUS: []sheinproduct.SKU{
						{
							SupplierSKU:   "dup-sku",
							CostInfo:      &sheinproduct.CostInfo{CostPrice: "10", Currency: "USD"},
							PriceInfoList: []sheinproduct.PriceInfo{{SubSite: "US", BasePrice: 12, Currency: "USD"}},
							StockInfoList: []sheinproduct.StockInfo{{InventoryNum: 1, MerchantWarehouseCode: "DEFAULT"}},
							SaleAttributeList: []sheinproduct.SaleAttribute{
								{AttributeID: 502, AttributeValueID: 21},
							},
						},
					},
				},
				{
					SaleAttribute: sheinproduct.SaleAttribute{AttributeID: 501, AttributeValueID: 12},
					SKUS: []sheinproduct.SKU{
						{
							SupplierSKU:   "dup-sku",
							CostInfo:      &sheinproduct.CostInfo{CostPrice: "10", Currency: "USD"},
							PriceInfoList: []sheinproduct.PriceInfo{{SubSite: "US", BasePrice: 12, Currency: "USD"}},
							StockInfoList: []sheinproduct.StockInfo{{InventoryNum: 1, MerchantWarehouseCode: "DEFAULT"}},
							SaleAttributeList: []sheinproduct.SaleAttribute{
								{AttributeID: 502, AttributeValueID: 22},
							},
						},
					},
				},
			},
		},
	}

	err := validator.validateSKCAndSKUData(input)
	if err == nil {
		t.Fatal("expected duplicate supplier sku validation error")
	}
	if !strings.Contains(err.Error(), "重复") || !strings.Contains(err.Error(), "dup-sku") {
		t.Fatalf("expected duplicate supplier sku detail, got %v", err)
	}
}
