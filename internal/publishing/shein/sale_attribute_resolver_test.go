package shein

import (
	"context"
	"strings"
	"testing"

	openaiclient "task-processor/internal/infra/clients/openai"
	"task-processor/internal/productenrich"
	common "task-processor/internal/publishing/common"
	sheinattribute "task-processor/internal/shein/api/attribute"
)

type stubSequentialSaleLLM struct {
	responses []string
	index     int
}

func (s *stubSequentialSaleLLM) CreateChatCompletion(context.Context, *openaiclient.ChatCompletionRequest) (*openaiclient.ChatCompletionResponse, error) {
	return nil, nil
}

func (s *stubSequentialSaleLLM) Generate(context.Context, string) (string, error) {
	if s.index >= len(s.responses) {
		return "", nil
	}
	response := s.responses[s.index]
	s.index++
	return response, nil
}

func (s *stubSequentialSaleLLM) AnalyzeImage(context.Context, string, string) (string, error) {
	return "", nil
}

func (s *stubSequentialSaleLLM) GetDefaultModel() string {
	return "test"
}

func TestSaleAttributeResolverKeepsChosenSourceDimensionOrder(t *testing.T) {
	canonical := &productenrich.CanonicalProduct{
		VariantDimensions: []productenrich.ScrapedVariantDimension{
			{Name: "颜色", Values: []string{"红色", "蓝色"}},
			{Name: "尺码", Values: []string{"42", "43"}},
		},
		Variants: []productenrich.CanonicalVariant{
			{
				SKU: "SKU-RED-42",
				Attributes: map[string]productenrich.CanonicalAttribute{
					"颜色": {Value: "红色"},
					"尺码": {Value: "42"},
				},
			},
			{
				SKU: "SKU-BLUE-43",
				Attributes: map[string]productenrich.CanonicalAttribute{
					"颜色": {Value: "蓝色"},
					"尺码": {Value: "43"},
				},
			},
		},
	}
	pkg := &Package{CategoryID: 8824}
	llm := &stubSequentialSaleLLM{
		responses: []string{
			`{"primary_source_dimension":"颜色","secondary_source_dimension":"尺码","reasons":["source-dimension-plan"]}`,
			`{"primary_source_dimension":"尺码","secondary_source_dimension":"颜色","primary_attribute_id":87,"secondary_attribute_id":27,"reasons":["wrong-order"]}`,
		},
	}

	resolver := NewSaleAttributeResolver(stubAttributeAPI{
		templates: &sheinattribute.AttributeTemplateInfo{
			Data: []sheinattribute.AttributeTemplate{{
				AttributeInfos: []sheinattribute.AttributeInfo{
					{
						AttributeID:       27,
						AttributeName:     "颜色",
						AttributeNameEn:   "Color",
						AttributeType:     1,
						SKCScope:          boolPointer(true),
						AttributeInputNum: 1,
						AttributeValueInfoList: []sheinattribute.AttributeValue{
							{AttributeValueID: 11, AttributeValue: "红色", AttributeValueEn: "Red"},
							{AttributeValueID: 12, AttributeValue: "蓝色", AttributeValueEn: "Blue"},
						},
					},
					{
						AttributeID:       87,
						AttributeName:     "尺寸",
						AttributeNameEn:   "Size",
						AttributeType:     1,
						SKCScope:          boolPointer(true),
						AttributeInputNum: 1,
						AttributeValueInfoList: []sheinattribute.AttributeValue{
							{AttributeValueID: 21, AttributeValue: "42", AttributeValueEn: "42"},
							{AttributeValueID: 22, AttributeValue: "43", AttributeValueEn: "43"},
						},
					},
				},
			}},
		},
	}, llm)

	resolution := resolver.Resolve(&BuildRequest{}, canonical, pkg)
	if resolution.PrimarySourceDimension != "颜色" {
		t.Fatalf("primary source dimension = %q, want 颜色", resolution.PrimarySourceDimension)
	}
	if resolution.SecondarySourceDimension != "尺码" {
		t.Fatalf("secondary source dimension = %q, want 尺码", resolution.SecondarySourceDimension)
	}
	if resolution.PrimaryAttributeID != 27 {
		t.Fatalf("primary attribute id = %d, want 27", resolution.PrimaryAttributeID)
	}
	if resolution.SecondaryAttributeID != 0 {
		t.Fatalf("secondary attribute id = %d, want 0 without alias matching", resolution.SecondaryAttributeID)
	}

	variants := common.BuildVariants(canonical)
	groups := buildVariantGroups("", variants, &common.ImageSet{MainImage: "main.jpg"}, resolution)
	if len(groups) != 2 {
		t.Fatalf("group count = %d, want 2", len(groups))
	}
	if groups[0].skcName != "红色" || groups[1].skcName != "蓝色" {
		t.Fatalf("group names = %q, %q; want 红色/蓝色", groups[0].skcName, groups[1].skcName)
	}
}

func TestSaleAttributeResolverDoesNotMapMismatchedSourceToFirstTemplateAttribute(t *testing.T) {
	canonical := &productenrich.CanonicalProduct{
		VariantDimensions: []productenrich.ScrapedVariantDimension{
			{Name: "颜色", Values: []string{"白色"}},
		},
		Variants: []productenrich.CanonicalVariant{
			{
				SKU: "SKU-WHITE",
				Attributes: map[string]productenrich.CanonicalAttribute{
					"颜色": {Value: "白色"},
				},
			},
		},
	}
	pkg := &Package{CategoryID: 8218}

	resolver := NewSaleAttributeResolver(stubAttributeAPI{
		templates: &sheinattribute.AttributeTemplateInfo{
			Data: []sheinattribute.AttributeTemplate{{
				AttributeInfos: []sheinattribute.AttributeInfo{
					{
						AttributeID:       301,
						AttributeName:     "款式",
						AttributeNameEn:   "Style",
						AttributeType:     1,
						SKCScope:          boolPointer(true),
						AttributeInputNum: 1,
						AttributeValueInfoList: []sheinattribute.AttributeValue{
							{AttributeValueID: 901, AttributeValue: "白色", AttributeValueEn: "White"},
						},
					},
					{
						AttributeID:       27,
						AttributeName:     "颜色",
						AttributeNameEn:   "Color",
						AttributeType:     1,
						SKCScope:          boolPointer(true),
						AttributeInputNum: 1,
						AttributeValueInfoList: []sheinattribute.AttributeValue{
							{AttributeValueID: 739, AttributeValue: "白色", AttributeValueEn: "White"},
						},
					},
				},
			}},
		},
	}, nil)

	resolution := resolver.Resolve(&BuildRequest{}, canonical, pkg)
	if resolution.PrimaryAttributeID != 27 {
		t.Fatalf("primary attribute id = %d, want direct Color template 27", resolution.PrimaryAttributeID)
	}
	if len(resolution.SKCAttributes) == 0 || resolution.SKCAttributes[0].Name != "Color" {
		t.Fatalf("skc attributes = %+v, want Color selected", resolution.SKCAttributes)
	}
	if resolution.Status != "resolved" {
		t.Fatalf("status = %q, want resolved", resolution.Status)
	}
}

func TestSaleAttributeResolverPrefersPrimaryLabeledAttributeOverEarlierSecondaryAttribute(t *testing.T) {
	canonical := &productenrich.CanonicalProduct{
		VariantDimensions: []productenrich.ScrapedVariantDimension{
			{Name: "ai_style", Values: []string{"Blue Dog Graphic"}},
			{Name: "颜色", Values: []string{"白色"}},
		},
		Variants: []productenrich.CanonicalVariant{
			{SKU: "SKU-WHITE", Attributes: map[string]productenrich.CanonicalAttribute{"ai_style": {Value: "Blue Dog Graphic"}, "颜色": {Value: "白色"}}},
		},
	}
	pkg := &Package{CategoryID: 3105, SpuName: "Wall Clock"}
	resolver := NewSaleAttributeResolver(stubAttributeAPI{
		templates: &sheinattribute.AttributeTemplateInfo{
			Data: []sheinattribute.AttributeTemplate{{
				AttributeInfos: []sheinattribute.AttributeInfo{
					{
						AttributeID:       1001211,
						AttributeName:     "件数",
						AttributeNameEn:   "Quantity",
						AttributeType:     1,
						AttributeLabel:    0,
						AttributeInputNum: 1,
					},
					{
						AttributeID:       301,
						AttributeName:     "主规格",
						AttributeNameEn:   "Primary Spec",
						AttributeType:     1,
						AttributeLabel:    1,
						AttributeInputNum: 1,
					},
				},
			}},
		},
		validateCustom: func(attributeID int, attributeValue string, categoryID int, spuName string) (*sheinattribute.ValidateAttributeResponse, error) {
			resp := &sheinattribute.ValidateAttributeResponse{}
			resp.Data.AttributeID = attributeID
			resp.Data.PreAttributeValueID = 3001
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
	}, nil)

	resolution := resolver.Resolve(&BuildRequest{}, canonical, pkg)
	if resolution.PrimaryAttributeID != 301 {
		t.Fatalf("primary attribute id = %d, want primary labeled attribute 301", resolution.PrimaryAttributeID)
	}
	if len(resolution.SKCAttributes) == 0 || resolution.SKCAttributes[0].AttributeID != 301 {
		t.Fatalf("skc attributes = %+v, want primary labeled attribute selected", resolution.SKCAttributes)
	}
}

func TestSaleAttributeResolverPrefersCustomFirstTemplateSaleAttribute(t *testing.T) {
	canonical := &productenrich.CanonicalProduct{
		VariantDimensions: []productenrich.ScrapedVariantDimension{
			{Name: "ai_style", Values: []string{"Blue Dog Graphic"}},
			{Name: "颜色", Values: []string{"白色"}},
		},
		Variants: []productenrich.CanonicalVariant{
			{SKU: "SKU-WHITE", Attributes: map[string]productenrich.CanonicalAttribute{"ai_style": {Value: "Blue Dog Graphic"}, "颜色": {Value: "白色"}}},
		},
	}
	pkg := &Package{CategoryID: 3105, SpuName: "Wall Clock"}
	resolver := NewSaleAttributeResolver(stubAttributeAPI{
		templates: &sheinattribute.AttributeTemplateInfo{
			Data: []sheinattribute.AttributeTemplate{{
				AttributeInfos: []sheinattribute.AttributeInfo{
					{
						AttributeID:       301,
						AttributeName:     "款式",
						AttributeNameEn:   "Style",
						AttributeType:     1,
						SKCScope:          boolPointer(true),
						AttributeInputNum: 1,
					},
					{
						AttributeID:       27,
						AttributeName:     "颜色",
						AttributeNameEn:   "Color",
						AttributeType:     1,
						SKCScope:          boolPointer(true),
						AttributeInputNum: 1,
						AttributeValueInfoList: []sheinattribute.AttributeValue{
							{AttributeValueID: 739, AttributeValue: "白色", AttributeValueEn: "White"},
						},
					},
				},
			}},
		},
		validateCustom: func(attributeID int, attributeValue string, categoryID int, spuName string) (*sheinattribute.ValidateAttributeResponse, error) {
			resp := &sheinattribute.ValidateAttributeResponse{}
			resp.Data.AttributeID = attributeID
			resp.Data.PreAttributeValueID = 3001
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
	}, nil)

	resolution := resolver.Resolve(&BuildRequest{}, canonical, pkg)
	if resolution.PrimaryAttributeID != 301 {
		t.Fatalf("primary attribute id = %d, want custom first template sale attribute 301", resolution.PrimaryAttributeID)
	}
	if len(resolution.SKCAttributes) == 0 || resolution.SKCAttributes[0].Name != "Style" {
		t.Fatalf("skc attributes = %+v, want Style selected", resolution.SKCAttributes)
	}
	assignment := resolution.skcValueAssignments[normalizeText("Blue Dog Graphic")]
	if assignment.AttributeValueID == nil || *assignment.AttributeValueID != 9001 {
		t.Fatalf("custom assignment = %+v, want value id 9001", assignment)
	}
	if resolution.Status != "resolved" {
		t.Fatalf("status = %q, want resolved", resolution.Status)
	}
}

func TestSaleAttributeResolverKeepsFirstStyleTemplateWhenFixedValuesDoNotFit(t *testing.T) {
	canonical := &productenrich.CanonicalProduct{
		VariantDimensions: []productenrich.ScrapedVariantDimension{
			{Name: "ai_style", Values: []string{"Blue Dog Graphic"}},
			{Name: "颜色", Values: []string{"白色"}},
		},
		Variants: []productenrich.CanonicalVariant{
			{SKU: "SKU-WHITE", Attributes: map[string]productenrich.CanonicalAttribute{"ai_style": {Value: "Blue Dog Graphic"}, "颜色": {Value: "白色"}}},
		},
	}
	pkg := &Package{CategoryID: 3105, SpuName: "Wall Clock"}
	resolver := NewSaleAttributeResolver(stubAttributeAPI{
		templates: &sheinattribute.AttributeTemplateInfo{
			Data: []sheinattribute.AttributeTemplate{{
				AttributeInfos: []sheinattribute.AttributeInfo{
					{
						AttributeID:       301,
						AttributeName:     "款式",
						AttributeNameEn:   "Style",
						AttributeType:     1,
						SKCScope:          boolPointer(true),
						AttributeInputNum: 1,
						AttributeValueInfoList: []sheinattribute.AttributeValue{
							{AttributeValueID: 901, AttributeValue: "现代款", AttributeValueEn: "Modern Style"},
						},
					},
					{
						AttributeID:       27,
						AttributeName:     "颜色",
						AttributeNameEn:   "Color",
						AttributeType:     1,
						SKCScope:          boolPointer(true),
						AttributeInputNum: 1,
						AttributeValueInfoList: []sheinattribute.AttributeValue{
							{AttributeValueID: 739, AttributeValue: "白色", AttributeValueEn: "White"},
						},
					},
				},
			}},
		},
		validateCustom: func(attributeID int, attributeValue string, categoryID int, spuName string) (*sheinattribute.ValidateAttributeResponse, error) {
			resp := &sheinattribute.ValidateAttributeResponse{}
			resp.Data.AttributeID = attributeID
			resp.Data.PreAttributeValueID = 3001
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
	}, nil)

	resolution := resolver.Resolve(&BuildRequest{}, canonical, pkg)
	if resolution.PrimaryAttributeID != 301 {
		t.Fatalf("primary attribute id = %d, want first Style template 301 even when fixed values do not fit", resolution.PrimaryAttributeID)
	}
	if len(resolution.Candidates) == 0 || resolution.Candidates[0].Name != "Style" || resolution.Candidates[0].SelectedScope != "skc" {
		t.Fatalf("first candidate = %+v, want selected Style", resolution.Candidates)
	}
	if resolution.Status != "resolved" {
		t.Fatalf("status = %q, want resolved", resolution.Status)
	}
}

func TestSaleAttributeResolverUsesAIStyleForPrimaryStyleTypeWithoutMisusingColor(t *testing.T) {
	canonical := &productenrich.CanonicalProduct{
		VariantDimensions: []productenrich.ScrapedVariantDimension{
			{Name: "ai_style", Values: []string{"Blue Dog Graphic"}},
			{Name: "Color", Values: []string{"White"}},
			{Name: "Size", Values: []string{"10x10in"}},
		},
		Variants: []productenrich.CanonicalVariant{
			{SKU: "SKU-WHITE", Attributes: map[string]productenrich.CanonicalAttribute{
				"ai_style": {Value: "Blue Dog Graphic"},
				"Color":    {Value: "White"},
				"Size":     {Value: "10x10in"},
			}},
		},
	}
	pkg := &Package{CategoryID: 3105, SpuName: "Wall Clock"}
	resolver := NewSaleAttributeResolver(stubAttributeAPI{
		templates: &sheinattribute.AttributeTemplateInfo{
			Data: []sheinattribute.AttributeTemplate{{
				AttributeInfos: []sheinattribute.AttributeInfo{
					{
						AttributeID:       1001211,
						AttributeName:     "件数",
						AttributeNameEn:   "Quantity",
						AttributeType:     1,
						AttributeLabel:    0,
						AttributeInputNum: 1,
						AttributeValueInfoList: []sheinattribute.AttributeValue{
							{AttributeValueID: 1, AttributeValue: "1件", AttributeValueEn: "1pc"},
						},
					},
					{
						AttributeID:       1001184,
						AttributeName:     "款式",
						AttributeNameEn:   "Style Type",
						AttributeType:     1,
						AttributeLabel:    1,
						AttributeInputNum: 1,
						AttributeValueInfoList: []sheinattribute.AttributeValue{
							{AttributeValueID: 2, AttributeValue: "现代款", AttributeValueEn: "Modern"},
						},
					},
					{
						AttributeID:       27,
						AttributeName:     "颜色",
						AttributeNameEn:   "Color",
						AttributeType:     1,
						AttributeLabel:    0,
						AttributeInputNum: 1,
						AttributeValueInfoList: []sheinattribute.AttributeValue{
							{AttributeValueID: 739, AttributeValue: "白色", AttributeValueEn: "White"},
						},
					},
				},
			}},
		},
		validateCustom: func(attributeID int, attributeValue string, categoryID int, spuName string) (*sheinattribute.ValidateAttributeResponse, error) {
			if attributeID != 1001184 {
				t.Fatalf("custom validation attribute id = %d, want Style Type", attributeID)
			}
			if attributeValue != "Blue Dog Graphic" {
				t.Fatalf("custom validation value = %q, want AI style", attributeValue)
			}
			resp := &sheinattribute.ValidateAttributeResponse{}
			resp.Data.AttributeID = attributeID
			resp.Data.PreAttributeValueID = 3001
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
	}, nil)

	resolution := resolver.Resolve(&BuildRequest{}, canonical, pkg)
	if resolution.PrimaryAttributeID != 1001184 {
		t.Fatalf("primary attribute id = %d, want Style Type 1001184", resolution.PrimaryAttributeID)
	}
	if resolution.PrimarySourceDimension != "ai_style" {
		t.Fatalf("primary source dimension = %q, want ai_style", resolution.PrimarySourceDimension)
	}
	if assignment := resolution.skcValueAssignments[normalizeText("Blue Dog Graphic")]; assignment.AttributeValueID == nil || *assignment.AttributeValueID != 9001 {
		t.Fatalf("ai_style assignment = %+v, want custom value 9001", assignment)
	}
}

func TestSaleAttributeResolverKeepsClothingColorPrimaryOverAIStyle(t *testing.T) {
	canonical := &productenrich.CanonicalProduct{
		VariantDimensions: []productenrich.ScrapedVariantDimension{
			{Name: "ai_style", Values: []string{"Blue Dog Graphic"}},
			{Name: "Color", Values: []string{"White"}},
			{Name: "Size", Values: []string{"M"}},
		},
		Variants: []productenrich.CanonicalVariant{
			{SKU: "SKU-WHITE-M", Attributes: map[string]productenrich.CanonicalAttribute{
				"ai_style": {Value: "Blue Dog Graphic"},
				"Color":    {Value: "White"},
				"Size":     {Value: "M"},
			}},
		},
	}
	pkg := &Package{CategoryID: 1758}
	resolver := NewSaleAttributeResolver(stubAttributeAPI{
		templates: &sheinattribute.AttributeTemplateInfo{
			Data: []sheinattribute.AttributeTemplate{{
				AttributeInfos: []sheinattribute.AttributeInfo{
					{
						AttributeID:       27,
						AttributeName:     "颜色",
						AttributeNameEn:   "Color",
						AttributeType:     1,
						AttributeLabel:    1,
						AttributeInputNum: 1,
						AttributeValueInfoList: []sheinattribute.AttributeValue{
							{AttributeValueID: 739, AttributeValue: "白色", AttributeValueEn: "White"},
						},
					},
					{
						AttributeID:       87,
						AttributeName:     "尺寸",
						AttributeNameEn:   "Size",
						AttributeType:     1,
						AttributeLabel:    0,
						AttributeInputNum: 1,
						AttributeValueInfoList: []sheinattribute.AttributeValue{
							{AttributeValueID: 211, AttributeValue: "M", AttributeValueEn: "M"},
						},
					},
				},
			}},
		},
	}, nil)

	resolution := resolver.Resolve(&BuildRequest{}, canonical, pkg)
	if resolution.PrimaryAttributeID != 27 {
		t.Fatalf("primary attribute id = %d, want Color 27", resolution.PrimaryAttributeID)
	}
	if resolution.PrimarySourceDimension != "Color" {
		t.Fatalf("primary source dimension = %q, want Color", resolution.PrimarySourceDimension)
	}
}

func TestSaleAttributeResolverPromotesRequiredMultiValueColorOverEarlierStyleType(t *testing.T) {
	canonical := &productenrich.CanonicalProduct{
		VariantDimensions: []productenrich.ScrapedVariantDimension{
			{Name: "ai_style", Values: []string{"Denim cap graphic"}},
			{Name: "Color", Values: []string{"Washed Black", "Sand colored", "Carbon Gray"}},
			{Name: "Size", Values: []string{"One size"}},
		},
		Variants: []productenrich.CanonicalVariant{
			{SKU: "SKU-BLK", Attributes: map[string]productenrich.CanonicalAttribute{"ai_style": {Value: "Denim cap graphic"}, "Color": {Value: "Washed Black"}, "Size": {Value: "One size"}}},
			{SKU: "SKU-SAND", Attributes: map[string]productenrich.CanonicalAttribute{"ai_style": {Value: "Denim cap graphic"}, "Color": {Value: "Sand colored"}, "Size": {Value: "One size"}}},
			{SKU: "SKU-GRY", Attributes: map[string]productenrich.CanonicalAttribute{"ai_style": {Value: "Denim cap graphic"}, "Color": {Value: "Carbon Gray"}, "Size": {Value: "One size"}}},
		},
	}
	pkg := &Package{CategoryID: 1772, SpuName: "Washed denim hat"}
	resolver := NewSaleAttributeResolver(stubAttributeAPI{
		templates: &sheinattribute.AttributeTemplateInfo{
			Data: []sheinattribute.AttributeTemplate{{
				AttributeInfos: []sheinattribute.AttributeInfo{
					{
						AttributeID:       1001184,
						AttributeName:     "款式",
						AttributeNameEn:   "Style Type",
						AttributeType:     1,
						AttributeLabel:    1,
						AttributeInputNum: 1,
						AttributeValueInfoList: []sheinattribute.AttributeValue{
							{AttributeValueID: 886, AttributeValue: "默认", AttributeValueEn: "Default"},
						},
					},
					{
						AttributeID:       27,
						AttributeName:     "颜色",
						AttributeNameEn:   "Color",
						AttributeType:     1,
						AttributeStatus:   3,
						AttributeInputNum: 1,
						AttributeValueInfoList: []sheinattribute.AttributeValue{
							{AttributeValueID: 112, AttributeValue: "黑色", AttributeValueEn: "Black"},
							{AttributeValueID: 544, AttributeValue: "卡其色", AttributeValueEn: "Khaki"},
							{AttributeValueID: 2493, AttributeValue: "深灰色", AttributeValueEn: "Dark Grey"},
						},
					},
				},
			}},
		},
	}, &stubSequentialSaleLLM{responses: []string{
		`{"attribute_id":27,"attribute_value_id":2493,"reasons":["carbon gray maps to dark grey"]}`,
	}})

	resolution := resolver.Resolve(&BuildRequest{}, canonical, pkg)
	if resolution.PrimaryAttributeID != 27 {
		t.Fatalf("primary attribute id = %d, want required multi-value Color 27; candidates=%+v", resolution.PrimaryAttributeID, resolution.Candidates)
	}
	if resolution.PrimarySourceDimension != "Color" {
		t.Fatalf("primary source dimension = %q, want Color", resolution.PrimarySourceDimension)
	}
	if resolution.SecondaryAttributeID != 1001184 {
		t.Fatalf("secondary attribute id = %d, want Style Type 1001184", resolution.SecondaryAttributeID)
	}
}

func TestSaleAttributeResolverPromotesImportantSingleColorOverEarlierAIStyleType(t *testing.T) {
	canonical := &productenrich.CanonicalProduct{
		VariantDimensions: []productenrich.ScrapedVariantDimension{
			{Name: "ai_style", Values: []string{"National flag graphic"}},
			{Name: "Color", Values: []string{"Black"}},
			{Name: "Size", Values: []string{"One size"}},
		},
		Variants: []productenrich.CanonicalVariant{
			{SKU: "SKU-BLK", Attributes: map[string]productenrich.CanonicalAttribute{
				"ai_style": {Value: "National flag graphic"},
				"Color":    {Value: "Black"},
				"Size":     {Value: "One size"},
			}},
		},
	}
	pkg := &Package{CategoryID: 8794, SpuName: "Baseball cap"}
	resolver := NewSaleAttributeResolver(stubAttributeAPI{
		templates: &sheinattribute.AttributeTemplateInfo{
			Data: []sheinattribute.AttributeTemplate{{
				AttributeInfos: []sheinattribute.AttributeInfo{
					{
						AttributeID:       1001184,
						AttributeName:     "款式",
						AttributeNameEn:   "Style Type",
						AttributeType:     1,
						AttributeLabel:    1,
						AttributeInputNum: 1,
						AttributeValueInfoList: []sheinattribute.AttributeValue{
							{AttributeValueID: 901, AttributeValue: "国旗图案", AttributeValueEn: "National flag graphic"},
						},
					},
					{
						AttributeID:       27,
						AttributeName:     "颜色",
						AttributeNameEn:   "Color",
						AttributeType:     1,
						AttributeLabel:    1,
						SKCScope:          boolPointer(true),
						AttributeInputNum: 1,
						AttributeValueInfoList: []sheinattribute.AttributeValue{
							{AttributeValueID: 112, AttributeValue: "黑色", AttributeValueEn: "Black"},
						},
					},
					{
						AttributeID:       87,
						AttributeName:     "尺寸",
						AttributeNameEn:   "Size",
						AttributeType:     1,
						AttributeStatus:   3,
						AttributeInputNum: 1,
						AttributeValueInfoList: []sheinattribute.AttributeValue{
							{AttributeValueID: 8801, AttributeValue: "均码", AttributeValueEn: "One size"},
						},
					},
				},
			}},
		},
	}, nil)

	resolution := resolver.Resolve(&BuildRequest{}, canonical, pkg)
	if resolution.PrimaryAttributeID != 27 {
		t.Fatalf("primary attribute id = %d, want important Color 27; candidates=%+v", resolution.PrimaryAttributeID, resolution.Candidates)
	}
	if resolution.PrimarySourceDimension != "Color" {
		t.Fatalf("primary source dimension = %q, want Color", resolution.PrimarySourceDimension)
	}
	if len(resolution.Candidates) == 0 || resolution.Candidates[0].Name != "Color" || !resolution.Candidates[0].Important {
		t.Fatalf("first candidate = %+v, want important selected Color", resolution.Candidates)
	}
}

func TestSaleAttributeResolverSkipsTechnicalSourceDimensions(t *testing.T) {
	canonical := &productenrich.CanonicalProduct{
		VariantDimensions: []productenrich.ScrapedVariantDimension{
			{Name: "source_sds_sku", Values: []string{"MG17701062001"}},
			{Name: "颜色", Values: []string{"白色"}},
		},
		Variants: []productenrich.CanonicalVariant{
			{SKU: "SKU-WHITE", Attributes: map[string]productenrich.CanonicalAttribute{"source_sds_sku": {Value: "MG17701062001"}, "颜色": {Value: "白色"}}},
		},
	}
	pkg := &Package{CategoryID: 3105}
	resolver := NewSaleAttributeResolver(stubAttributeAPI{
		templates: &sheinattribute.AttributeTemplateInfo{
			Data: []sheinattribute.AttributeTemplate{{
				AttributeInfos: []sheinattribute.AttributeInfo{
					{
						AttributeID:       87,
						AttributeName:     "尺寸",
						AttributeNameEn:   "Size",
						AttributeType:     1,
						AttributeInputNum: 1,
						AttributeValueInfoList: []sheinattribute.AttributeValue{
							{AttributeValueID: 739, AttributeValue: "MG17701062001", AttributeValueEn: "MG17701062001"},
						},
					},
					{
						AttributeID:       27,
						AttributeName:     "颜色",
						AttributeNameEn:   "Color",
						AttributeType:     1,
						AttributeInputNum: 1,
						AttributeValueInfoList: []sheinattribute.AttributeValue{
							{AttributeValueID: 740, AttributeValue: "白色", AttributeValueEn: "White"},
						},
					},
				},
			}},
		},
	}, nil)

	resolution := resolver.Resolve(&BuildRequest{}, canonical, pkg)
	if resolution.PrimaryAttributeID != 27 {
		t.Fatalf("primary attribute id = %d, want color attribute 27 after technical dimensions are skipped", resolution.PrimaryAttributeID)
	}
	for _, candidate := range resolution.Candidates {
		if candidate.SourceDimension == "source_sds_sku" {
			t.Fatalf("technical source dimension should not become a sale candidate: %+v", candidate)
		}
	}
}

func TestTechnicalSaleSourceDimensionRecognizesNormalizedNames(t *testing.T) {
	if !isTechnicalSaleSourceDimension("source_sds_sku") {
		t.Fatal("source_sds_sku should be treated as technical")
	}
	if !isTechnicalSaleSourceDimension("supplier_sku") {
		t.Fatal("supplier_sku should be treated as technical")
	}
	if isTechnicalSaleSourceDimension("颜色") {
		t.Fatal("user-facing source dimensions should remain eligible")
	}
}

func TestSaleAttributeResolverMarksPartialWhenValueAssignmentsDoNotResolve(t *testing.T) {
	canonical := &productenrich.CanonicalProduct{
		VariantDimensions: []productenrich.ScrapedVariantDimension{
			{Name: "颜色", Values: []string{"一桌四椅套装", "月亮椅-矮椅"}},
		},
		Variants: []productenrich.CanonicalVariant{
			{
				SKU: "SKU-SET",
				Attributes: map[string]productenrich.CanonicalAttribute{
					"颜色": {Value: "一桌四椅套装"},
				},
			},
			{
				SKU: "SKU-CHAIR",
				Attributes: map[string]productenrich.CanonicalAttribute{
					"颜色": {Value: "月亮椅-矮椅"},
				},
			},
		},
	}
	pkg := &Package{CategoryID: 12143}
	resolver := NewSaleAttributeResolver(stubAttributeAPI{
		templates: &sheinattribute.AttributeTemplateInfo{
			Data: []sheinattribute.AttributeTemplate{{
				AttributeInfos: []sheinattribute.AttributeInfo{
					{
						AttributeID:       27,
						AttributeName:     "颜色",
						AttributeNameEn:   "Color",
						AttributeType:     1,
						SKCScope:          boolPointer(true),
						AttributeInputNum: 1,
						AttributeValueInfoList: []sheinattribute.AttributeValue{
							{AttributeValueID: 11, AttributeValue: "红色", AttributeValueEn: "Red"},
							{AttributeValueID: 12, AttributeValue: "蓝色", AttributeValueEn: "Blue"},
						},
					},
				},
			}},
		},
	}, nil)

	resolution := resolver.Resolve(&BuildRequest{}, canonical, pkg)
	if resolution.Status != "partial" {
		t.Fatalf("status = %q, want partial", resolution.Status)
	}
	if len(resolution.ReviewNotes) == 0 {
		t.Fatal("expected review notes for unresolved sale attribute values")
	}
	if len(resolution.skcValueAssignments) != 0 {
		t.Fatalf("skc assignments = %d, want 0", len(resolution.skcValueAssignments))
	}
}

func TestSaleAttributeResolverRejectsTemplateCandidateWithZeroValueFit(t *testing.T) {
	canonical := &productenrich.CanonicalProduct{
		VariantDimensions: []productenrich.ScrapedVariantDimension{
			{Name: "颜色", Values: []string{"一桌四椅套装", "月亮椅-矮椅", "月亮椅-高椅", "超轻折叠桌"}},
		},
		Variants: []productenrich.CanonicalVariant{
			{SKU: "SKU-1", Attributes: map[string]productenrich.CanonicalAttribute{"颜色": {Value: "一桌四椅套装"}}},
			{SKU: "SKU-2", Attributes: map[string]productenrich.CanonicalAttribute{"颜色": {Value: "月亮椅-矮椅"}}},
		},
	}
	pkg := &Package{CategoryID: 12143}
	resolver := NewSaleAttributeResolver(stubAttributeAPI{
		templates: &sheinattribute.AttributeTemplateInfo{
			Data: []sheinattribute.AttributeTemplate{{
				AttributeInfos: []sheinattribute.AttributeInfo{
					{
						AttributeID:       27,
						AttributeName:     "颜色",
						AttributeNameEn:   "Color",
						AttributeType:     1,
						SKCScope:          boolPointer(true),
						AttributeInputNum: 1,
						AttributeValueInfoList: []sheinattribute.AttributeValue{
							{AttributeValueID: 11, AttributeValue: "红色", AttributeValueEn: "Red"},
							{AttributeValueID: 12, AttributeValue: "蓝色", AttributeValueEn: "Blue"},
						},
					},
				},
			}},
		},
	}, nil)

	resolution := resolver.Resolve(&BuildRequest{}, canonical, pkg)
	if resolution.PrimaryAttributeID != 0 {
		t.Fatalf("primary attribute id = %d, want 0 when candidate fit is zero", resolution.PrimaryAttributeID)
	}
	if resolution.Status != "partial" {
		t.Fatalf("status = %q, want partial", resolution.Status)
	}
	if !resolution.RecommendCategoryReview {
		t.Fatalf("recommend_category_review = false, want true")
	}
	if resolution.CategoryReviewReason == "" {
		t.Fatal("expected category_review_reason")
	}
	if len(resolution.Candidates) == 0 || resolution.Candidates[0].SelectedScope != "" {
		t.Fatalf("expected unselected candidate when fit is zero: %+v", resolution.Candidates)
	}
	if len(resolution.ReviewNotes) == 0 {
		t.Fatal("expected review notes when candidate value fit is zero")
	}
	found := false
	foundCategoryHint := false
	for _, note := range resolution.ReviewNotes {
		if strings.Contains(note, "无有效拟合") {
			found = true
			if !strings.Contains(note, "套装/组合款") && !strings.Contains(note, "款式/型号") {
				t.Fatalf("expected semantic explanation in review note, got %q", note)
			}
		}
		if strings.Contains(note, "当前类目销售属性模板未提供可承接") {
			foundCategoryHint = true
		}
	}
	if !found {
		t.Fatalf("expected review notes to explain zero-fit candidate, got %v", resolution.ReviewNotes)
	}
	if !foundCategoryHint {
		t.Fatalf("expected category mismatch hint in review notes, got %v", resolution.ReviewNotes)
	}
}

func TestSaleAttributeResolverDoesNotSelectMismatchedStyleCandidateWhenColorHasZeroFit(t *testing.T) {
	canonical := &productenrich.CanonicalProduct{
		VariantDimensions: []productenrich.ScrapedVariantDimension{
			{Name: "颜色", Values: []string{"月亮椅-高椅", "月亮椅-矮椅"}},
		},
		Variants: []productenrich.CanonicalVariant{
			{SKU: "SKU-HIGH", Attributes: map[string]productenrich.CanonicalAttribute{"颜色": {Value: "月亮椅-高椅"}}},
			{SKU: "SKU-LOW", Attributes: map[string]productenrich.CanonicalAttribute{"颜色": {Value: "月亮椅-矮椅"}}},
		},
	}
	pkg := &Package{CategoryID: 12143}
	resolver := NewSaleAttributeResolver(stubAttributeAPI{
		templates: &sheinattribute.AttributeTemplateInfo{
			Data: []sheinattribute.AttributeTemplate{{
				AttributeInfos: []sheinattribute.AttributeInfo{
					{
						AttributeID:       27,
						AttributeName:     "颜色",
						AttributeNameEn:   "Color",
						AttributeType:     1,
						SKCScope:          boolPointer(true),
						AttributeInputNum: 1,
						AttributeValueInfoList: []sheinattribute.AttributeValue{
							{AttributeValueID: 11, AttributeValue: "红色", AttributeValueEn: "Red"},
							{AttributeValueID: 12, AttributeValue: "蓝色", AttributeValueEn: "Blue"},
						},
					},
					{
						AttributeID:       301,
						AttributeName:     "款式",
						AttributeNameEn:   "Style",
						AttributeType:     1,
						SKCScope:          boolPointer(true),
						AttributeInputNum: 1,
						AttributeValueInfoList: []sheinattribute.AttributeValue{
							{AttributeValueID: 901, AttributeValue: "月亮椅-高椅", AttributeValueEn: "High Moon Chair"},
							{AttributeValueID: 902, AttributeValue: "月亮椅-矮椅", AttributeValueEn: "Low Moon Chair"},
						},
					},
				},
			}},
		},
	}, nil)

	resolution := resolver.Resolve(&BuildRequest{}, canonical, pkg)
	if resolution.PrimaryAttributeID != 0 {
		t.Fatalf("primary attribute id = %d, want 0 for mismatched source/template", resolution.PrimaryAttributeID)
	}
	if resolution.Status != "partial" {
		t.Fatalf("status = %q, want partial", resolution.Status)
	}
	var colorCandidate *SaleAttributeCandidateInfo
	for i := range resolution.Candidates {
		candidate := &resolution.Candidates[i]
		if candidate.Name == "Color" {
			colorCandidate = candidate
		}
	}
	if colorCandidate == nil {
		t.Fatalf("expected Color candidate for review, got %+v", resolution.Candidates)
	}
	if colorCandidate.SelectedScope != "" {
		t.Fatalf("color candidate should remain unselected: %+v", colorCandidate)
	}
}
