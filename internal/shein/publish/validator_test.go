package publish

import (
	"strings"
	"testing"

	sheinproduct "task-processor/internal/shein/api/product"
)

func TestPublishProductValidatorRejectsUnresolvedProductAttributes(t *testing.T) {
	validator := NewPublishProductValidator()
	input := &ValidationInput{
		ProductData: &sheinproduct.Product{
			CategoryID:             100,
			MultiLanguageNameList:  []sheinproduct.LanguageContent{{Language: "en", Name: "Shoes"}},
			MultiLanguageDescList:  []sheinproduct.LanguageContent{{Language: "en", Name: "Desc"}},
			ProductAttributeList:   []sheinproduct.ProductAttribute{{AttributeExtraValue: "Leather"}},
			SKCList:                []sheinproduct.SKC{{SKUS: []sheinproduct.SKU{{SupplierSKU: "sku-1", CostInfo: &sheinproduct.CostInfo{CostPrice: "10", Currency: "USD"}, PriceInfoList: []sheinproduct.PriceInfo{{SubSite: "US", BasePrice: 12, Currency: "USD"}}, StockInfoList: []sheinproduct.StockInfo{{InventoryNum: 1, MerchantWarehouseCode: "DEFAULT"}}}}}},
		},
	}

	err := validator.validateResolvedMappings(input)
	if err == nil || !strings.Contains(err.Error(), "attribute_id") {
		t.Fatalf("expected unresolved attribute id error, got %v", err)
	}
}

func TestPublishProductValidatorRejectsMissingGroupedSaleAttributes(t *testing.T) {
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
					SKUS: []sheinproduct.SKU{
						{
							SupplierSKU:   "sku-red-42",
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
					SKUS: []sheinproduct.SKU{
						{
							SupplierSKU:   "sku-blue-42",
							CostInfo:      &sheinproduct.CostInfo{CostPrice: "10", Currency: "USD"},
							PriceInfoList: []sheinproduct.PriceInfo{{SubSite: "US", BasePrice: 12, Currency: "USD"}},
							StockInfoList: []sheinproduct.StockInfo{{InventoryNum: 1, MerchantWarehouseCode: "DEFAULT"}},
							SaleAttributeList: []sheinproduct.SaleAttribute{
								{AttributeID: 502, AttributeValueID: 21},
							},
						},
					},
				},
			},
		},
	}

	err := validator.validateResolvedMappings(input)
	if err == nil || !strings.Contains(err.Error(), "SKC[0]缺少真实销售属性映射") {
		t.Fatalf("expected missing skc sale attribute error, got %v", err)
	}
}

func TestPublishProductValidatorAllowsResolvedVariantMappings(t *testing.T) {
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
							SupplierSKU:   "sku-red-42",
							CostInfo:      &sheinproduct.CostInfo{CostPrice: "10", Currency: "USD"},
							PriceInfoList: []sheinproduct.PriceInfo{{SubSite: "US", BasePrice: 12, Currency: "USD"}},
							StockInfoList: []sheinproduct.StockInfo{{InventoryNum: 1, MerchantWarehouseCode: "DEFAULT"}},
							SaleAttributeList: []sheinproduct.SaleAttribute{
								{AttributeID: 502, AttributeValueID: 21},
							},
						},
						{
							SupplierSKU:   "sku-red-43",
							CostInfo:      &sheinproduct.CostInfo{CostPrice: "10", Currency: "USD"},
							PriceInfoList: []sheinproduct.PriceInfo{{SubSite: "US", BasePrice: 12, Currency: "USD"}},
							StockInfoList: []sheinproduct.StockInfo{{InventoryNum: 1, MerchantWarehouseCode: "DEFAULT"}},
							SaleAttributeList: []sheinproduct.SaleAttribute{
								{AttributeID: 502, AttributeValueID: 22},
							},
						},
					},
				},
				{
					SaleAttribute: sheinproduct.SaleAttribute{AttributeID: 501, AttributeValueID: 12},
					SKUS: []sheinproduct.SKU{
						{
							SupplierSKU:   "sku-blue-42",
							CostInfo:      &sheinproduct.CostInfo{CostPrice: "10", Currency: "USD"},
							PriceInfoList: []sheinproduct.PriceInfo{{SubSite: "US", BasePrice: 12, Currency: "USD"}},
							StockInfoList: []sheinproduct.StockInfo{{InventoryNum: 1, MerchantWarehouseCode: "DEFAULT"}},
							SaleAttributeList: []sheinproduct.SaleAttribute{
								{AttributeID: 502, AttributeValueID: 21},
							},
						},
					},
				},
			},
		},
	}

	if err := validator.validateResolvedMappings(input); err != nil {
		t.Fatalf("expected resolved mappings to pass, got %v", err)
	}
}

func TestPublishProductValidatorPreValidateAllowsNilContextForStandaloneValidation(t *testing.T) {
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
							SupplierSKU:   "sku-red-42",
							CostInfo:      &sheinproduct.CostInfo{CostPrice: "10", Currency: "USD"},
							PriceInfoList: []sheinproduct.PriceInfo{{SubSite: "US", BasePrice: 12, Currency: "USD"}},
							StockInfoList: []sheinproduct.StockInfo{{InventoryNum: 1, MerchantWarehouseCode: "DEFAULT"}},
							SaleAttributeList: []sheinproduct.SaleAttribute{
								{AttributeID: 502, AttributeValueID: 21},
							},
						},
					},
				},
			},
		},
	}

	if err := validator.PreValidateProductData(nil, input); err != nil {
		t.Fatalf("expected standalone validation to pass with nil ctx, got %v", err)
	}
}

func TestPublishProductValidatorPreValidateReturnsDiagnosticCriticalReason(t *testing.T) {
	validator := NewPublishProductValidator()
	input := &ValidationInput{
		ProductData: &sheinproduct.Product{
			CategoryID:            100,
			MultiLanguageNameList: []sheinproduct.LanguageContent{{Language: "en", Name: "Shoes"}},
			MultiLanguageDescList: []sheinproduct.LanguageContent{{Language: "en", Name: "Desc"}},
			ProductAttributeList: []sheinproduct.ProductAttribute{
				{AttributeExtraValue: "Leather"},
			},
			SKCList: []sheinproduct.SKC{
				{
					SaleAttribute: sheinproduct.SaleAttribute{AttributeID: 501, AttributeValueID: 11},
					SKUS: []sheinproduct.SKU{
						{
							SupplierSKU:   "sku-red-42",
							CostInfo:      &sheinproduct.CostInfo{CostPrice: "10", Currency: "USD"},
							PriceInfoList: []sheinproduct.PriceInfo{{SubSite: "US", BasePrice: 12, Currency: "USD"}},
							StockInfoList: []sheinproduct.StockInfo{{InventoryNum: 1, MerchantWarehouseCode: "DEFAULT"}},
							SaleAttributeList: []sheinproduct.SaleAttribute{
								{AttributeID: 502, AttributeValueID: 21},
							},
						},
					},
				},
			},
		},
	}

	err := validator.PreValidateProductData(nil, input)
	if err == nil {
		t.Fatal("expected validation error")
	}
	if !strings.Contains(err.Error(), "属性映射") {
		t.Fatalf("expected diagnostic mapping reason in error, got %v", err)
	}
	if !strings.Contains(err.Error(), "attribute_id") {
		t.Fatalf("expected attribute_id detail in error, got %v", err)
	}
}
