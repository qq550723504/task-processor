package goods

import (
	"github.com/stretchr/testify/require"
	model "task-processor/internal/marketplace/shein/model"
	"task-processor/internal/product/asset"
	"testing"
)

func rulePointer[T any](value T) *T { return &value }
func officialFixture() (OfficialDraftInput, OfficialRuleSnapshot, asset.ApprovedAssetInventory) {
	input := OfficialDraftInput{
		Product: model.PublishProduct{
			CategoryID: 123, SupplierCode: "source-product",
			Names:      []model.LanguageContent{{Language: "en", Name: "Source product"}},
			Attributes: []model.AttributeValue{},
			SKCs: []model.ProductSKC{{
				SupplierCode: "merchant-code", SaleAttribute: model.AttributeValue{AttributeID: 12, AttributeValueID: rulePointer(int64(34))},
				SKUs: []model.ProductSKU{{SupplierSKU: "merchant-sku", Length: "10", Width: "10", Height: "10", Weight: rulePointer(10.0), MallState: 1, Prices: []model.ProductPrice{{BasePrice: 12.5, Currency: "USD", SubSite: "shein-us"}}, Stock: []model.ProductStock{{Quantity: 5}}, SaleAttributes: []model.AttributeValue{}}},
			}},
		},
		Images: []OfficialImageSlot{{Group: "skc", SKC: 0, AssetID: "main", Sort: 1, Type: 1}, {Group: "skc", SKC: 0, AssetID: "detail", Sort: 2, Type: 2}, {Group: "skc", SKC: 0, AssetID: "square", Sort: 3, Type: 5}},
	}
	rules := OfficialRuleSnapshot{Categories: []model.Category{{ID: 123, ProductTypeID: 456, Leaf: rulePointer(true)}}, Sites: []model.MainSite{{ID: "shein", Sites: []model.Site{{Abbreviation: "shein-us", Status: rulePointer(1), StoreType: rulePointer(2), Currency: "USD"}}}}, Fill: model.FillStandards{DefaultLanguage: "en", DefaultTitleMaximum: rulePointer(150), SupplierCodeInSPU: rulePointer(false), Fields: []model.FillRule{}, Pictures: []model.PictureRule{{Field: "switch_spu_picture", Enabled: rulePointer(false)}, {Field: "sku_image_required", Enabled: rulePointer(false)}}}, Attributes: model.AttributeTemplate{ProductTypeID: 456, MainAttributeStatus: rulePointer(1), Attributes: []model.Attribute{{ID: 12, Name: "Default", Type: rulePointer(1), Show: rulePointer(1), MainLabel: rulePointer(1), Mode: rulePointer(2), Status: rulePointer(3), MaximumSelections: rulePointer(1), Options: []model.AttributeOption{{ID: 34, Show: rulePointer(1), Name: "Default"}}}}}, Linked: []model.LinkedRules{{GroupID: "product", Attributes: []model.LinkedAttributeRule{}}}, Brands: []model.Brand{}}
	inventory := asset.ApprovedAssetInventory{Scope: asset.InventoryScope{TenantID: "org-a", ProductKey: "product-a", TargetPlatform: "shein", SourceSnapshotVersion: 1}, Assets: []asset.ApprovedAsset{{ID: "main", URL: "https://images.example.org/main.jpg", Width: 900, Height: 900}, {ID: "detail", URL: "https://images.example.org/detail.jpg", Width: 900, Height: 900}, {ID: "square", URL: "https://images.example.org/square.jpg", Width: 900, Height: 900}}}
	rules.Linked = append(rules.Linked, model.LinkedRules{GroupID: "sku-0-0", Attributes: []model.LinkedAttributeRule{}})
	return input, rules, inventory
}
func TestOfficialDraftUsesExactMerchantRulesAndNeverInventsRemoteImageEvidence(t *testing.T) {
	input, rules, inventory := officialFixture()
	result := BuildOfficial(input, rules, inventory, nil)
	require.Empty(t, result.Issues)
	require.True(t, result.ReadyForUpload)
	require.Empty(t, result.SubmissionPayload, "local complete data is not a provider wire payload")
	require.False(t, result.Product.IsSPUPic, "October sku_image_required does not turn the old image scheme into SPU mode")
	require.EqualValues(t, 456, result.Product.ProductTypeID)
	require.Equal(t, "OpenAPI", result.Product.SourceSystem)
	observations := []OfficialImageObservation{}
	for index, item := range inventory.Assets {
		observations = append(observations, OfficialImageObservation{AssetID: item.ID, SourceURL: item.URL, Width: item.Width, Height: item.Height, Type: input.Images[index].Type, RemoteURL: "https://img.shein.com/" + item.ID + ".jpg", ResponseHash: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"})
	}
	result = BuildOfficial(input, rules, inventory, observations)
	require.Empty(t, result.Issues)
	require.NotEmpty(t, result.SubmissionPayload)
	require.NotContains(t, string(result.SubmissionPayload), "images.example.org")
	observations[0].SourceURL = "https://images.example.org/replacement.jpg"
	result = BuildOfficial(input, rules, inventory, observations)
	require.Empty(t, result.SubmissionPayload)
}
func TestOfficialDraftReportsMissingFactsAndMerchantRequiredFields(t *testing.T) {
	input, rules, inventory := officialFixture()
	input.Product.SKCs[0].SKUs[0].Weight = nil
	rules.Fill.Fields = append(rules.Fill.Fields, model.FillRule{Field: "brand_code", Required: rulePointer(true), Show: rulePointer(true)})
	result := BuildOfficial(input, rules, inventory, nil)
	require.False(t, result.ReadyForUpload)
	require.Contains(t, issueFields(result), "skc_list.0.sku_list.0.weight")
	require.Contains(t, issueFields(result), "brand_code")
	rules.Fill.Fields = append(rules.Fill.Fields, model.FillRule{Field: "future_required_fact", Required: rulePointer(true), Show: rulePointer(true)})
	result = BuildOfficial(input, rules, inventory, nil)
	require.Contains(t, issueFields(result), "future_required_fact")
}
func TestOfficialDraftSKUImagesDependOnQuantityAndCategoryNotSKUCount(t *testing.T) {
	input, rules, inventory := officialFixture()
	second := input.Product.SKCs[0].SKUs[0]
	second.SupplierSKU = "second-sku"
	input.Product.SKCs[0].SKUs = append(input.Product.SKCs[0].SKUs, second)
	rules.Linked = append(rules.Linked, model.LinkedRules{GroupID: "sku-0-1", Attributes: []model.LinkedAttributeRule{}})
	// Actual sales templates must distinguish two SKUs; this fixture adds one.
	rules.Attributes.Attributes = append(rules.Attributes.Attributes, model.Attribute{ID: 56, Name: "Size", Type: rulePointer(1), Show: rulePointer(1), MainLabel: rulePointer(0), Mode: rulePointer(2), Status: rulePointer(3), MaximumSelections: rulePointer(1), Options: []model.AttributeOption{{ID: 78, Show: rulePointer(1)}, {ID: 79, Show: rulePointer(1)}}})
	input.Product.SKCs[0].SKUs[0].SaleAttributes = []model.AttributeValue{{AttributeID: 56, AttributeValueID: rulePointer(int64(78))}}
	input.Product.SKCs[0].SKUs[1].SaleAttributes = []model.AttributeValue{{AttributeID: 56, AttributeValueID: rulePointer(int64(79))}}
	result := BuildOfficial(input, rules, inventory, nil)
	require.Empty(t, result.Issues, "two individual SKUs do not require SKU photos")
	rules.Fill.Fields = append(rules.Fill.Fields, model.FillRule{Field: "quantity_info", Required: rulePointer(false), Show: rulePointer(true)})
	input.Product.SKCs[0].SKUs[0].Quantity = &model.QuantityInfo{Type: 2, Unit: 1, Quantity: 2}
	input.Product.SKCs[0].SKUs[1].Quantity = &model.QuantityInfo{Type: 1, Unit: 1, Quantity: 1}
	result = BuildOfficial(input, rules, inventory, nil)
	require.Contains(t, issueFields(result), "skc_list.0.sku_list.0.image_info")
	require.Contains(t, issueFields(result), "skc_list.0.sku_list.1.image_info")
}
func issueFields(result OfficialDraft) []string {
	fields := []string{}
	for _, issue := range result.Issues {
		fields = append(fields, issue.Field)
	}
	return fields
}

func TestOfficialDraftRejectsHiddenPartialFieldsAndMissingAssociatedAuthority(t *testing.T) {
	input, rules, inventory := officialFixture()
	rules.Fill.Fields = []model.FillRule{{Field: "quantity_info", Show: rulePointer(false), Required: rulePointer(false)}}
	input.Product.SKCs[0].SKUs[0].Quantity = &model.QuantityInfo{Type: 1, Unit: 1, Quantity: 1}
	result := BuildOfficial(input, rules, inventory, nil)
	require.Contains(t, issueFields(result), "quantity_info")
	input, rules, inventory = officialFixture()
	rules.Linked = []model.LinkedRules{}
	result = BuildOfficial(input, rules, inventory, nil)
	require.False(t, result.ReadyForUpload)
	require.Contains(t, issueFields(result), "linked_attributes")
	input, rules, inventory = officialFixture()
	rules.Attributes.Attributes = append(rules.Attributes.Attributes, model.Attribute{ID: 99, Type: rulePointer(4), Status: rulePointer(3), Mode: rulePointer(3), Show: rulePointer(1)})
	result = BuildOfficial(input, rules, inventory, nil)
	require.False(t, result.ReadyForUpload, "a required attribute with missing dimension must not disappear")
}
func TestOfficialDraftFollowsSPUSchemeBWithoutInventingSKCSquareRequirement(t *testing.T) {
	input, rules, inventory := officialFixture()
	rules.Fill.Pictures = []model.PictureRule{}
	for field, value := range map[string]bool{"switch_spu_picture": false, "sku_image_required": false, "spu_image_detail_show": true, "spu_image_detail_required": true, "spu_image_detail_single": false, "spu_image_square_show": true, "spu_image_square_required": true, "skc_image_detail_show": true, "skc_image_detail_required": true, "skc_image_detail_single": true, "skc_image_square_show": false, "skc_image_square_required": false} {
		rules.Fill.Pictures = append(rules.Fill.Pictures, model.PictureRule{Field: field, Enabled: rulePointer(value)})
	}
	inventory.Assets = append(inventory.Assets, asset.ApprovedAsset{ID: "spu-square", URL: "https://images.example.org/spu-square.jpg", Width: 1200, Height: 1200})
	input.Images = []OfficialImageSlot{{Group: "skc", AssetID: "main", Sort: 1, Type: 1}, {Group: "spu", AssetID: "main", Sort: 1, Type: 1}, {Group: "spu", AssetID: "detail", Sort: 2, Type: 2}, {Group: "spu", AssetID: "spu-square", Sort: 3, Type: 5}}
	result := BuildOfficial(input, rules, inventory, nil)
	require.Empty(t, result.Issues)
	require.True(t, result.Product.IsSPUPic)
	require.Len(t, result.Product.SKCs[0].ImageInfo.Images, 1)
	require.Len(t, result.Product.ImageInfo.Images, 3)
}
func TestOfficialDraftConsumesBothLinkedValueRangesAndPreservesInput(t *testing.T) {
	input, rules, inventory := officialFixture()
	rules.Attributes.Attributes = append(rules.Attributes.Attributes, model.Attribute{ID: 56, Name: "Material", Type: rulePointer(4), Show: rulePointer(1), Mode: rulePointer(3), Status: rulePointer(2), Dimension: rulePointer(1), Options: []model.AttributeOption{{ID: 91, Show: rulePointer(1)}, {ID: 92, Show: rulePointer(1)}}})
	rules.Linked = []model.LinkedRules{{GroupID: "product", Attributes: []model.LinkedAttributeRule{{AttributeID: 56, Values: []int64{91}, PrefilledValues: []int64{92}}}}}
	input.Product.Attributes = []model.AttributeValue{{AttributeID: 56, AttributeValueID: rulePointer(int64(92))}}
	rules.Linked = append(rules.Linked, model.LinkedRules{GroupID: "sku-0-0", Attributes: []model.LinkedAttributeRule{}})
	result := BuildOfficial(input, rules, inventory, nil)
	require.Empty(t, result.Issues)
	require.Empty(t, input.Product.SKCs[0].ImageInfo.Images)
	require.Zero(t, input.Product.ProductTypeID)
	input.Product.Attributes = nil
	result = BuildOfficial(input, rules, inventory, nil)
	require.Contains(t, issueFields(result), "product_attribute_list.56")
}

func TestOfficialDraftRequiresEveryQuantityAndRejectsInvalidOptionalSKUValues(t *testing.T) {
	input, rules, inventory := officialFixture()
	input.Product.SKCs[0].SKUs[0].PackageType = rulePointer("invalid")
	rules.Fill.Fields = []model.FillRule{{Field: "package_type", Show: rulePointer(true), Required: rulePointer(false)}}
	result := BuildOfficial(input, rules, inventory, nil)
	require.False(t, result.ReadyForUpload)
	require.Contains(t, issueFields(result), "skc_list.0.sku_list.0.package_type")
	input, rules, inventory = officialFixture()
	input.Product.SKCs[0].SKUs[0].MinimumStockQuantity = "not-a-number"
	rules.Fill.Fields = []model.FillRule{{Field: "minimum_stock_quantity", Show: rulePointer(true), Required: rulePointer(false)}}
	result = BuildOfficial(input, rules, inventory, nil)
	require.False(t, result.ReadyForUpload)
}
func TestOfficialDraftOnlyPublishesSupplierCodesInMerchantChosenDimension(t *testing.T) {
	input, rules, inventory := officialFixture()
	result := BuildOfficial(input, rules, inventory, nil)
	require.Empty(t, result.Product.SupplierCode)
	rules.Fill.SupplierCodeInSPU = rulePointer(true)
	result = BuildOfficial(input, rules, inventory, nil)
	require.Equal(t, "source-product", result.Product.SupplierCode)
	require.Empty(t, result.Product.SKCs[0].SupplierCode)
}

func TestOfficialDraftAppliesAssociatedRangesOnlyToTheirExactSKUCombination(t *testing.T) {
	input, rules, inventory := officialFixture()
	second := input.Product.SKCs[0].SKUs[0]
	second.SupplierSKU = "sku-two"
	input.Product.SKCs[0].SKUs = append(input.Product.SKCs[0].SKUs, second)
	rules.Attributes.Attributes = append(rules.Attributes.Attributes,
		model.Attribute{ID: 56, Name: "Size", Type: rulePointer(1), Show: rulePointer(1), MainLabel: rulePointer(0), Mode: rulePointer(2), Status: rulePointer(3), MaximumSelections: rulePointer(1), Options: []model.AttributeOption{{ID: 78, Show: rulePointer(1)}, {ID: 79, Show: rulePointer(1)}}},
		model.Attribute{ID: 57, Name: "Material", Type: rulePointer(4), Show: rulePointer(1), Mode: rulePointer(3), Status: rulePointer(2), Dimension: rulePointer(3), Options: []model.AttributeOption{{ID: 91, Show: rulePointer(1)}, {ID: 92, Show: rulePointer(1)}}})
	for i := range input.Product.SKCs[0].SKUs {
		input.Product.SKCs[0].SKUs[i].SaleAttributes = []model.AttributeValue{{AttributeID: 56, AttributeValueID: rulePointer(int64(78 + i))}}
		input.Product.SKCs[0].SKUs[i].Attributes = []model.AttributeValue{{AttributeID: 57, AttributeValueID: rulePointer(int64(91 + i))}}
	}
	rules.Linked = []model.LinkedRules{{GroupID: "product", Attributes: []model.LinkedAttributeRule{}}, {GroupID: "sku-0-0", Attributes: []model.LinkedAttributeRule{{AttributeID: 57, Values: []int64{91}}}}, {GroupID: "sku-0-1", Attributes: []model.LinkedAttributeRule{{AttributeID: 57, PrefilledValues: []int64{92}}}}}
	result := BuildOfficial(input, rules, inventory, nil)
	require.Empty(t, result.Issues, "SKU two's allowed range must not be applied to SKU one")
	rules.Linked = rules.Linked[:2]
	result = BuildOfficial(input, rules, inventory, nil)
	require.Contains(t, issueFields(result), "linked_attributes", "every actual combination needs a current result")
}
