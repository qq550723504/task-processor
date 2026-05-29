package listingkit

import (
	"fmt"
	"strings"
	"testing"

	sheinpub "task-processor/internal/publishing/shein"
	sheinattribute "task-processor/internal/shein/api/attribute"
	sheincategory "task-processor/internal/shein/api/category"
)

func TestEvaluateSheinCategoryFreshnessAllowsCurrentCategoryWhenStillLegal(t *testing.T) {
	t.Parallel()

	current := &SheinPackage{
		CategoryID:     3001,
		ProductTypeID:  intPtr(9001),
		CategoryIDList: []int{1, 2, 3001},
	}
	info := &sheincategory.CategoryInfo{
		CategoryID:    3001,
		ProductTypeID: 9001,
	}

	ok, message := evaluateSheinCategoryFreshness(current, info)
	if !ok {
		t.Fatalf("expected legal category to pass freshness, got %q", message)
	}
	if message == "" {
		t.Fatal("expected success message")
	}
}

func TestEvaluateSheinCategoryFreshnessBlocksProductTypeMismatch(t *testing.T) {
	t.Parallel()

	current := &SheinPackage{
		CategoryID:    3001,
		ProductTypeID: intPtr(9001),
	}
	info := &sheincategory.CategoryInfo{
		CategoryID:    3001,
		ProductTypeID: 9002,
	}

	ok, message := evaluateSheinCategoryFreshness(current, info)
	if ok {
		t.Fatal("expected product type mismatch to block")
	}
	if !containsAll(message, "category_id=3001", "product_type_id=9001", "product_type_id=9002") {
		t.Fatalf("message = %q, want category/product type mismatch detail", message)
	}
}

func TestEvaluateSheinAttributeFreshnessAllowsCurrentValueWhenStillLegal(t *testing.T) {
	t.Parallel()

	valueID := 1007571
	current := &SheinPackage{
		ResolvedAttributes: []sheinpub.ResolvedAttribute{{
			Name:             "Details",
			Value:            "Hanging Ornament",
			AttributeID:      31,
			AttributeValueID: &valueID,
		}},
	}
	templates := &sheinattribute.AttributeTemplateInfo{
		Data: []sheinattribute.AttributeTemplate{{
			AttributeInfos: []sheinattribute.AttributeInfo{{
				AttributeID:     31,
				AttributeName:   "Details",
				AttributeNameEn: "Details",
				AttributeStatus: 3,
				AttributeValueInfoList: []sheinattribute.AttributeValue{
					{AttributeValueID: 1007571, AttributeValue: "Hanging Ornament", AttributeValueEn: "Hanging Ornament"},
					{AttributeValueID: 1010327, AttributeValue: "Handbag", AttributeValueEn: "Handbag"},
				},
			}},
		}},
	}

	ok, message := evaluateSheinAttributeFreshness(current, templates)
	if !ok {
		t.Fatalf("expected legal attribute value to pass freshness check, got message %q", message)
	}
	if message == "" {
		t.Fatal("expected success message")
	}
}

func TestEvaluateSheinAttributeFreshnessBlocksOfflineValueID(t *testing.T) {
	t.Parallel()

	currentValueID := 1002592
	current := &SheinPackage{
		ResolvedAttributes: []sheinpub.ResolvedAttribute{{
			Name:             "Material",
			Value:            "PU",
			AttributeID:      160,
			AttributeValueID: &currentValueID,
		}},
	}
	templates := &sheinattribute.AttributeTemplateInfo{
		Data: []sheinattribute.AttributeTemplate{{
			AttributeInfos: []sheinattribute.AttributeInfo{{
				AttributeID:     160,
				AttributeName:   "Material",
				AttributeNameEn: "Material",
				AttributeStatus: 3,
				AttributeValueInfoList: []sheinattribute.AttributeValue{
					{AttributeValueID: 1000145, AttributeValue: "Polyurethane", AttributeValueEn: "Polyurethane"},
				},
			}},
		}},
	}

	ok, message := evaluateSheinAttributeFreshness(current, templates)
	if ok {
		t.Fatal("expected offline attribute value to block")
	}
	if message == "" {
		t.Fatal("expected invalid value message")
	}
	if got := message; !containsAll(got,
		"attribute_value_id=1002592",
		"Material",
		"PU",
	) {
		t.Fatalf("invalid-value message = %q, want current attribute detail", got)
	}
}

func TestEvaluateSheinAttributeFreshnessBlocksMissingRequiredAttribute(t *testing.T) {
	t.Parallel()

	valueID := 1002592
	current := &SheinPackage{
		ResolvedAttributes: []sheinpub.ResolvedAttribute{{
			Name:             "Material",
			Value:            "PU",
			AttributeID:      160,
			AttributeValueID: &valueID,
		}},
	}
	templates := &sheinattribute.AttributeTemplateInfo{
		Data: []sheinattribute.AttributeTemplate{{
			AttributeInfos: []sheinattribute.AttributeInfo{
				{
					AttributeID:     160,
					AttributeName:   "Material",
					AttributeNameEn: "Material",
					AttributeStatus: 3,
					AttributeValueInfoList: []sheinattribute.AttributeValue{
						{AttributeValueID: 1002592, AttributeValue: "PU", AttributeValueEn: "PU"},
					},
				},
				{
					AttributeID:     31,
					AttributeName:   "Details",
					AttributeNameEn: "Details",
					AttributeStatus: 3,
					AttributeValueInfoList: []sheinattribute.AttributeValue{
						{AttributeValueID: 1007571, AttributeValue: "Hanging Ornament", AttributeValueEn: "Hanging Ornament"},
					},
				},
			},
		}},
	}

	ok, message := evaluateSheinAttributeFreshness(current, templates)
	if ok {
		t.Fatal("expected missing required attribute to block")
	}
	if !strings.Contains(message, "Details") {
		t.Fatalf("message = %q, want missing required attribute name", message)
	}
}

func TestEvaluateSheinAttributeFreshnessIgnoresSaleScopeRequiredAttributes(t *testing.T) {
	t.Parallel()

	valueID := 1002592
	current := &SheinPackage{
		ResolvedAttributes: []sheinpub.ResolvedAttribute{{
			Name:             "Material",
			Value:            "PU",
			AttributeID:      160,
			AttributeValueID: &valueID,
		}},
	}
	templates := &sheinattribute.AttributeTemplateInfo{
		Data: []sheinattribute.AttributeTemplate{{
			AttributeInfos: []sheinattribute.AttributeInfo{
				{
					AttributeID:     160,
					AttributeName:   "Material",
					AttributeNameEn: "Material",
					AttributeStatus: 3,
					AttributeValueInfoList: []sheinattribute.AttributeValue{
						{AttributeValueID: 1002592, AttributeValue: "PU", AttributeValueEn: "PU"},
					},
				},
				{
					AttributeID:     1001,
					AttributeName:   "Color",
					AttributeNameEn: "Color",
					AttributeStatus: 3,
					AttributeType:   1,
					SKCScope:        boolPtr(true),
				},
			},
		}},
	}

	ok, message := evaluateSheinAttributeFreshness(current, templates)
	if !ok {
		t.Fatalf("expected sale-scope required attribute to be ignored by display-attribute freshness, got %q", message)
	}
}

func TestEvaluateSheinSaleAttributeFreshnessAllowsCurrentValuesWhenStillLegal(t *testing.T) {
	t.Parallel()

	valueID := 27
	current := &SheinPackage{
		SaleAttributeResolution: &sheinpub.SaleAttributeResolution{
			Status:               "resolved",
			PrimaryAttributeID:   1001,
			SecondaryAttributeID: 1002,
			SKCAttributes: []sheinpub.ResolvedSaleAttribute{{
				Scope:            "skc",
				AttributeID:      1001,
				AttributeValueID: &valueID,
				Value:            "Black",
			}},
			SKUAttributes: []sheinpub.ResolvedSaleAttribute{{
				Scope:       "sku",
				AttributeID: 1002,
				Value:       "M",
			}},
		},
	}
	templates := &sheinattribute.AttributeTemplateInfo{
		Data: []sheinattribute.AttributeTemplate{{
			AttributeID: []int{1001, 1002},
			AttributeInfos: []sheinattribute.AttributeInfo{
				{
					AttributeID:     1001,
					AttributeName:   "Color",
					AttributeNameEn: "Color",
					AttributeType:   1,
					AttributeLabel:  1,
					SKCScope:        boolPtr(true),
					AttributeValueInfoList: []sheinattribute.AttributeValue{
						{AttributeValueID: 27, AttributeValue: "Black", AttributeValueEn: "Black"},
						{AttributeValueID: 28, AttributeValue: "White", AttributeValueEn: "White"},
					},
				},
				{
					AttributeID:     1002,
					AttributeName:   "Size",
					AttributeNameEn: "Size",
					AttributeType:   1,
					SKCScope:        boolPtr(false),
				},
				{
					AttributeID:     1003,
					AttributeName:   "Style Type",
					AttributeNameEn: "Style Type",
					AttributeType:   1,
					AttributeLabel:  1,
					SKCScope:        boolPtr(false),
				},
			},
		}},
	}

	ok, message := evaluateSheinSaleAttributeFreshness(current, templates)
	if !ok {
		t.Fatalf("expected legal sale attributes to pass, got message %q", message)
	}
	if message == "" {
		t.Fatal("expected success message")
	}
}

func TestEvaluateSheinSaleAttributeFreshnessBlocksOfflineValueID(t *testing.T) {
	t.Parallel()

	valueID := 27
	current := &SheinPackage{
		SaleAttributeResolution: &sheinpub.SaleAttributeResolution{
			Status:             "resolved",
			PrimaryAttributeID: 1001,
			SKCAttributes: []sheinpub.ResolvedSaleAttribute{{
				Scope:            "skc",
				AttributeID:      1001,
				AttributeValueID: &valueID,
				Value:            "Black",
			}},
		},
	}
	templates := &sheinattribute.AttributeTemplateInfo{
		Data: []sheinattribute.AttributeTemplate{{
			AttributeID: []int{1001},
			AttributeInfos: []sheinattribute.AttributeInfo{
				{
					AttributeID:     1001,
					AttributeName:   "Color",
					AttributeNameEn: "Color",
					AttributeType:   1,
					AttributeLabel:  1,
					SKCScope:        boolPtr(true),
					AttributeValueInfoList: []sheinattribute.AttributeValue{
						{AttributeValueID: 28, AttributeValue: "White", AttributeValueEn: "White"},
					},
				},
			},
		}},
	}

	ok, message := evaluateSheinSaleAttributeFreshness(current, templates)
	if ok {
		t.Fatal("expected offline sale value to block")
	}
	if !containsAll(message, "attribute_value_id=27", "Black") {
		t.Fatalf("message = %q, want offline sale value detail", message)
	}
}

func TestEvaluateSheinSaleAttributeFreshnessBlocksMissingPrimaryTemplate(t *testing.T) {
	t.Parallel()

	valueID := 27
	current := &SheinPackage{
		SaleAttributeResolution: &sheinpub.SaleAttributeResolution{
			Status:             "resolved",
			PrimaryAttributeID: 1001,
			SKCAttributes: []sheinpub.ResolvedSaleAttribute{{
				Scope:            "skc",
				AttributeID:      1001,
				AttributeValueID: &valueID,
				Value:            "Black",
			}},
		},
	}
	templates := &sheinattribute.AttributeTemplateInfo{
		Data: []sheinattribute.AttributeTemplate{{
			AttributeID: []int{1002},
			AttributeInfos: []sheinattribute.AttributeInfo{
				{
					AttributeID:     1002,
					AttributeName:   "Size",
					AttributeNameEn: "Size",
					AttributeType:   1,
					SKCScope:        boolPtr(false),
				},
			},
		}},
	}

	ok, message := evaluateSheinSaleAttributeFreshness(current, templates)
	if ok {
		t.Fatal("expected missing primary template to block")
	}
	if !strings.Contains(message, "主规格") {
		t.Fatalf("message = %q, want primary template failure", message)
	}
}

func TestEvaluateSheinSaleAttributeFreshnessIgnoresUnselectedRequiredSaleCandidates(t *testing.T) {
	t.Parallel()

	valueID := 739
	current := &SheinPackage{
		SaleAttributeResolution: &sheinpub.SaleAttributeResolution{
			Status:             "resolved",
			PrimaryAttributeID: 27,
			SKCAttributes: []sheinpub.ResolvedSaleAttribute{{
				Name:             "Color",
				Scope:            "skc",
				AttributeID:      27,
				AttributeValueID: &valueID,
				Value:            "white",
			}},
		},
	}
	templates := &sheinattribute.AttributeTemplateInfo{
		Data: []sheinattribute.AttributeTemplate{{
			AttributeID: []int{27, 87, 1001184, 9991},
			AttributeInfos: []sheinattribute.AttributeInfo{
				{
					AttributeID:     27,
					AttributeName:   "Color",
					AttributeNameEn: "Color",
					AttributeType:   1,
					SKCScope:        boolPtr(false),
					AttributeValueInfoList: []sheinattribute.AttributeValue{
						{AttributeValueID: 739, AttributeValue: "white", AttributeValueEn: "white"},
					},
				},
				{
					AttributeID:     87,
					AttributeName:   "Size",
					AttributeNameEn: "Size",
					AttributeType:   1,
					AttributeIsShow: 1,
					SKCScope:        boolPtr(false),
				},
				{
					AttributeID:     1001184,
					AttributeName:   "Style Type",
					AttributeNameEn: "Style Type",
					AttributeType:   1,
					AttributeLabel:  1,
					SKCScope:        boolPtr(false),
				},
				{
					AttributeID:     9991,
					AttributeName:   "Type",
					AttributeNameEn: "Type",
					AttributeType:   1,
					AttributeIsShow: 1,
					SKCScope:        boolPtr(false),
				},
			},
		}},
	}

	ok, message := evaluateSheinSaleAttributeFreshness(current, templates)
	if !ok {
		t.Fatalf("expected unrelated required sale candidates to be ignored, got %q", message)
	}
}

func TestEvaluateSheinSaleAttributeFreshnessRepairsOfflineValueViaCustomValidation(t *testing.T) {
	t.Parallel()

	offlineValueID := 27
	current := &SheinPackage{
		CategoryID: 12143,
		SpuName:    "Bench Cushion",
		SaleAttributeResolution: &sheinpub.SaleAttributeResolution{
			Status:             "resolved",
			PrimaryAttributeID: 1001,
			SKCAttributes: []sheinpub.ResolvedSaleAttribute{{
				Name:             "Color",
				Scope:            "skc",
				AttributeID:      1001,
				AttributeValueID: &offlineValueID,
				Value:            "米驼",
			}},
		},
	}
	templates := &sheinattribute.AttributeTemplateInfo{
		Data: []sheinattribute.AttributeTemplate{{
			AttributeID: []int{1001},
			AttributeInfos: []sheinattribute.AttributeInfo{{
				AttributeID:     1001,
				AttributeName:   "Color",
				AttributeNameEn: "Color",
				AttributeType:   1,
				AttributeLabel:  1,
				SKCScope:        boolPtr(true),
				AttributeValueInfoList: []sheinattribute.AttributeValue{
					{AttributeValueID: 28, AttributeValue: "White", AttributeValueEn: "White"},
				},
			}},
		}},
	}
	api := stubFreshnessAttributeAPI{
		validateCustom: func(attributeID int, attributeValue string, categoryID int, spuName string) (*sheinattribute.ValidateAttributeResponse, error) {
			resp := &sheinattribute.ValidateAttributeResponse{}
			resp.Data.AttributeID = attributeID
			resp.Data.PreAttributeValueID = 3001
			resp.Data.AttributeValueNameMultis = []struct {
				Language                string `json:"language"`
				AttributeValueNameMulti string `json:"attribute_value_name_multi"`
				WarningType             int    `json:"warning_type"`
			}{
				{Language: "en", AttributeValueNameMulti: "Cream Beige"},
			}
			return resp, nil
		},
		addCustom: func(req *sheinattribute.AddCustomAttributeValueRequest) (*sheinattribute.AddCustomAttributeValueResponse, error) {
			resp := &sheinattribute.AddCustomAttributeValueResponse{}
			resp.Info.Data.CustomAttributeRelation = []sheinattribute.CustomAttributeRelation{{
				PreAttributeValueID: 3001,
				AttributeValueID:    9001,
			}}
			return resp, nil
		},
	}

	ok, message, changed := evaluateSheinSaleAttributeFreshnessWithCustomValidation(current, templates, api)
	if !ok {
		t.Fatalf("expected custom validation repair to pass, got %q", message)
	}
	if !changed {
		t.Fatal("expected sale freshness repair to report changes")
	}
	got := current.SaleAttributeResolution.SKCAttributes[0].AttributeValueID
	if got == nil || *got != 9001 {
		t.Fatalf("repaired attribute_value_id = %v, want 9001", got)
	}
}

func TestEvaluateSheinSaleAttributeFreshnessBlocksWhenCustomValidationRejectsOfflineValue(t *testing.T) {
	t.Parallel()

	offlineValueID := 27
	current := &SheinPackage{
		CategoryID: 12144,
		SpuName:    "Bench Cushion",
		SaleAttributeResolution: &sheinpub.SaleAttributeResolution{
			Status:             "resolved",
			PrimaryAttributeID: 1001,
			SKCAttributes: []sheinpub.ResolvedSaleAttribute{{
				Name:             "Color",
				Scope:            "skc",
				AttributeID:      1001,
				AttributeValueID: &offlineValueID,
				Value:            "米驼",
			}},
		},
	}
	templates := &sheinattribute.AttributeTemplateInfo{
		Data: []sheinattribute.AttributeTemplate{{
			AttributeID: []int{1001},
			AttributeInfos: []sheinattribute.AttributeInfo{{
				AttributeID:     1001,
				AttributeName:   "Color",
				AttributeNameEn: "Color",
				AttributeType:   1,
				AttributeLabel:  1,
				SKCScope:        boolPtr(true),
				AttributeValueInfoList: []sheinattribute.AttributeValue{
					{AttributeValueID: 28, AttributeValue: "White", AttributeValueEn: "White"},
				},
			}},
		}},
	}
	api := stubFreshnessAttributeAPI{
		validateCustom: func(attributeID int, attributeValue string, categoryID int, spuName string) (*sheinattribute.ValidateAttributeResponse, error) {
			return nil, fmt.Errorf("没有自定义属性值权限")
		},
	}

	ok, message, changed := evaluateSheinSaleAttributeFreshnessWithCustomValidation(current, templates, api)
	if ok {
		t.Fatal("expected unresolved offline sale value to keep blocking")
	}
	if changed {
		t.Fatal("expected no mutations when custom validation rejects value")
	}
	if !containsAll(message, "attribute_value_id=27", "米驼") {
		t.Fatalf("message = %q, want offline value detail", message)
	}
}

type stubFreshnessAttributeAPI struct {
	validateCustom func(attributeID int, attributeValue string, categoryID int, spuName string) (*sheinattribute.ValidateAttributeResponse, error)
	addCustom      func(req *sheinattribute.AddCustomAttributeValueRequest) (*sheinattribute.AddCustomAttributeValueResponse, error)
}

func (s stubFreshnessAttributeAPI) GetAttributeTemplates(categoryID int) (*sheinattribute.AttributeTemplateInfo, error) {
	return nil, nil
}

func (s stubFreshnessAttributeAPI) ValidateCustomAttributeValue(attributeID int, attributeValue string, categoryID int, spuName string) (*sheinattribute.ValidateAttributeResponse, error) {
	if s.validateCustom == nil {
		return nil, nil
	}
	return s.validateCustom(attributeID, attributeValue, categoryID, spuName)
}

func (s stubFreshnessAttributeAPI) AddCustomAttributeValue(req *sheinattribute.AddCustomAttributeValueRequest) (*sheinattribute.AddCustomAttributeValueResponse, error) {
	if s.addCustom == nil {
		return nil, nil
	}
	return s.addCustom(req)
}

func containsAll(haystack string, needles ...string) bool {
	for _, needle := range needles {
		if !strings.Contains(haystack, needle) {
			return false
		}
	}
	return true
}
