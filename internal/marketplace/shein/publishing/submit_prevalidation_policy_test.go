package publishing

import (
	"strings"
	"testing"

	sheinproduct "task-processor/internal/shein/api/product"
)

func TestPreValidateSubmitProductRejectsMissingSupplierCode(t *testing.T) {
	productTypeID := 456
	err := PreValidateSubmitProduct(&sheinproduct.Product{
		SPUName:       "Test Product",
		CategoryID:    123,
		ProductTypeID: &productTypeID,
	})
	if err == nil {
		t.Fatalf("PreValidateSubmitProduct() returned nil, want validation error")
	}
}

func TestPreValidateSubmitProductAllowsPrimaryOnlyMultiSKU(t *testing.T) {
	product := minimalValidSubmitProduct()
	product.SKCList[0].SKUS = append(product.SKCList[0].SKUS, product.SKCList[0].SKUS[0])

	if err := PreValidateSubmitProductWithOptions(product, true); err != nil {
		t.Fatalf("PreValidateSubmitProductWithOptions(allow primary only) error = %v", err)
	}
	if err := PreValidateSubmitProductWithOptions(product, false); err == nil || !strings.Contains(err.Error(), "缺少销售属性") {
		t.Fatalf("PreValidateSubmitProductWithOptions(strict) error = %v, want missing sale attribute", err)
	}
}

func minimalValidSubmitProduct() *sheinproduct.Product {
	quantity := 1
	quantityType := 1
	quantityUnit := 1
	valueID := 101
	return &sheinproduct.Product{
		CategoryID: 123,
		MultiLanguageNameList: []sheinproduct.LanguageContent{{
			Language: "en",
			Name:     "Test Product",
		}},
		MultiLanguageDescList: []sheinproduct.LanguageContent{{
			Language: "en",
			Name:     "Description",
		}},
		ProductAttributeList: []sheinproduct.ProductAttribute{{
			AttributeID:      11,
			AttributeValueID: &valueID,
		}},
		SKCList: []sheinproduct.SKC{{
			SKUS: []sheinproduct.SKU{{
				SupplierSKU:   "SKU-1",
				CostInfo:      &sheinproduct.CostInfo{CostPrice: "1.23"},
				PriceInfoList: []sheinproduct.PriceInfo{{BasePrice: 2.34}},
				StockInfoList: []sheinproduct.StockInfo{{InventoryNum: 1}},
				QuantityInfo: &sheinproduct.QuantityInfo{
					Quantity:     &quantity,
					QuantityType: &quantityType,
					QuantityUnit: &quantityUnit,
				},
			}},
		}},
	}
}
