package shein

import (
	"context"
	"strings"
	"testing"

	"task-processor/internal/catalog/canonical"
	openaiclient "task-processor/internal/infra/clients/openai"
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
	canonical := &canonical.Product{
		VariantDimensions: []canonical.ScrapedVariantDimension{
			{Name: "颜色", Values: []string{"红色", "蓝色"}},
			{Name: "尺码", Values: []string{"42", "43"}},
		},
		Variants: []canonical.Variant{
			{
				SKU: "SKU-RED-42",
				Attributes: map[string]canonical.Attribute{
					"颜色": {Value: "红色"},
					"尺码": {Value: "42"},
				},
			},
			{
				SKU: "SKU-BLUE-43",
				Attributes: map[string]canonical.Attribute{
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
	canonical := &canonical.Product{
		VariantDimensions: []canonical.ScrapedVariantDimension{
			{Name: "颜色", Values: []string{"白色"}},
		},
		Variants: []canonical.Variant{
			{
				SKU: "SKU-WHITE",
				Attributes: map[string]canonical.Attribute{
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
	canonical := &canonical.Product{
		VariantDimensions: []canonical.ScrapedVariantDimension{
			{Name: "ai_style", Values: []string{"Blue Dog Graphic"}},
			{Name: "颜色", Values: []string{"白色"}},
		},
		Variants: []canonical.Variant{
			{SKU: "SKU-WHITE", Attributes: map[string]canonical.Attribute{"ai_style": {Value: "Blue Dog Graphic"}, "颜色": {Value: "白色"}}},
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
	canonical := &canonical.Product{
		VariantDimensions: []canonical.ScrapedVariantDimension{
			{Name: "ai_style", Values: []string{"Blue Dog Graphic"}},
			{Name: "颜色", Values: []string{"白色"}},
		},
		Variants: []canonical.Variant{
			{SKU: "SKU-WHITE", Attributes: map[string]canonical.Attribute{"ai_style": {Value: "Blue Dog Graphic"}, "颜色": {Value: "白色"}}},
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
	canonical := &canonical.Product{
		VariantDimensions: []canonical.ScrapedVariantDimension{
			{Name: "ai_style", Values: []string{"Blue Dog Graphic"}},
			{Name: "颜色", Values: []string{"白色"}},
		},
		Variants: []canonical.Variant{
			{SKU: "SKU-WHITE", Attributes: map[string]canonical.Attribute{"ai_style": {Value: "Blue Dog Graphic"}, "颜色": {Value: "白色"}}},
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
	canonical := &canonical.Product{
		VariantDimensions: []canonical.ScrapedVariantDimension{
			{Name: "ai_style", Values: []string{"Blue Dog Graphic"}},
			{Name: "Color", Values: []string{"White"}},
			{Name: "Size", Values: []string{"10x10in"}},
		},
		Variants: []canonical.Variant{
			{SKU: "SKU-WHITE", Attributes: map[string]canonical.Attribute{
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

func TestSaleAttributeResolverSanitizesPromptLikeStyleTypeValueWithRules(t *testing.T) {
	canonical := &canonical.Product{
		VariantDimensions: []canonical.ScrapedVariantDimension{
			{Name: "ai_style", Values: []string{"Blue Dog Graphic - please design printable artwork with suitable English text, 3000 px * 2"}},
			{Name: "Size", Values: []string{"40x60cm", "50x80cm"}},
		},
		Variants: []canonical.Variant{
			{SKU: "SKU-40", Attributes: map[string]canonical.Attribute{
				"ai_style": {Value: "Blue Dog Graphic - please design printable artwork with suitable English text, 3000 px * 2"},
				"Size":     {Value: "40x60cm"},
			}},
			{SKU: "SKU-50", Attributes: map[string]canonical.Attribute{
				"ai_style": {Value: "Blue Dog Graphic - please design printable artwork with suitable English text, 3000 px * 2"},
				"Size":     {Value: "50x80cm"},
			}},
		},
	}
	pkg := &Package{CategoryID: 12014, SpuName: "Flannel non slip floor mat"}
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
							{AttributeValueID: 2, AttributeValue: "默认", AttributeValueEn: "Default"},
						},
					},
					{
						AttributeID:       87,
						AttributeName:     "尺寸",
						AttributeNameEn:   "Size",
						AttributeType:     1,
						AttributeInputNum: 1,
						AttributeValueInfoList: []sheinattribute.AttributeValue{
							{AttributeValueID: 1916605, AttributeValue: "40x60cm", AttributeValueEn: "40x60cm"},
							{AttributeValueID: 11115576, AttributeValue: "50x80cm", AttributeValueEn: "50x80cm"},
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
				t.Fatalf("custom validation value = %q, want Blue Dog Graphic", attributeValue)
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
	if !resolution.ValueSanitized || resolution.ValueSanitizationSource != "rule_trimmed" {
		t.Fatalf("sanitization = (%v, %q), want rule_trimmed", resolution.ValueSanitized, resolution.ValueSanitizationSource)
	}
	if assignment := resolution.skcValueAssignments[normalizeText("Blue Dog Graphic - please design printable artwork with suitable English text, 3000 px * 2")]; assignment.Value != "Blue Dog Graphic" {
		t.Fatalf("sanitized ai_style assignment = %+v, want value Blue Dog Graphic", assignment)
	}
}

func TestSaleAttributeResolverUsesLLMToSanitizePromptLikeStyleTypeValue(t *testing.T) {
	canonical := &canonical.Product{
		VariantDimensions: []canonical.ScrapedVariantDimension{
			{Name: "ai_style", Values: []string{"Please design something amazing for my wall clock with suitable English text and graphics for printing, 3000 pixels wide."}},
			{Name: "Size", Values: []string{"10x10in", "12x12in"}},
		},
		Variants: []canonical.Variant{
			{SKU: "SKU-10", Attributes: map[string]canonical.Attribute{
				"ai_style": {Value: "Please design something amazing for my wall clock with suitable English text and graphics for printing, 3000 pixels wide."},
				"Size":     {Value: "10x10in"},
			}},
			{SKU: "SKU-12", Attributes: map[string]canonical.Attribute{
				"ai_style": {Value: "Please design something amazing for my wall clock with suitable English text and graphics for printing, 3000 pixels wide."},
				"Size":     {Value: "12x12in"},
			}},
		},
	}
	pkg := &Package{CategoryID: 3105, SpuName: "Wall clock"}
	llm := &stubSequentialSaleLLM{responses: []string{"", `{"value":"Mystic Graphic","reasons":["short style value extracted"]}`}}
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
							{AttributeValueID: 2, AttributeValue: "默认", AttributeValueEn: "Default"},
						},
					},
					{
						AttributeID:       87,
						AttributeName:     "尺寸",
						AttributeNameEn:   "Size",
						AttributeType:     1,
						AttributeInputNum: 1,
						AttributeValueInfoList: []sheinattribute.AttributeValue{
							{AttributeValueID: 101, AttributeValue: "10x10in", AttributeValueEn: "10x10in"},
							{AttributeValueID: 102, AttributeValue: "12x12in", AttributeValueEn: "12x12in"},
						},
					},
				},
			}},
		},
		validateCustom: func(attributeID int, attributeValue string, categoryID int, spuName string) (*sheinattribute.ValidateAttributeResponse, error) {
			if attributeValue != "Mystic Graphic" {
				t.Fatalf("custom validation value = %q, want Mystic Graphic", attributeValue)
			}
			resp := &sheinattribute.ValidateAttributeResponse{}
			resp.Data.AttributeID = attributeID
			resp.Data.PreAttributeValueID = 4001
			return resp, nil
		},
		addCustom: func(req *sheinattribute.AddCustomAttributeValueRequest) (*sheinattribute.AddCustomAttributeValueResponse, error) {
			resp := &sheinattribute.AddCustomAttributeValueResponse{}
			resp.Info.Data.CustomAttributeRelation = []sheinattribute.CustomAttributeRelation{{
				PreAttributeValueID: 4001,
				AttributeValueID:    9002,
			}}
			return resp, nil
		},
	}, llm)

	resolution := resolver.Resolve(&BuildRequest{}, canonical, pkg)
	if !resolution.ValueSanitized || resolution.ValueSanitizationSource != "llm_extracted" {
		t.Fatalf("sanitization = (%v, %q), want llm_extracted", resolution.ValueSanitized, resolution.ValueSanitizationSource)
	}
	if assignment := resolution.skcValueAssignments[normalizeText("Please design something amazing for my wall clock with suitable English text and graphics for printing, 3000 pixels wide.")]; assignment.Value != "Mystic Graphic" {
		t.Fatalf("llm-sanitized assignment = %+v, want value Mystic Graphic", assignment)
	}
}

func TestSaleAttributeResolverRequiresManualReviewWhenLLMReturnsPromptLikeText(t *testing.T) {
	canonical := &canonical.Product{
		VariantDimensions: []canonical.ScrapedVariantDimension{
			{Name: "ai_style", Values: []string{"Please design a printable style with suitable English text and graphics for the product, 3000 pixels wide."}},
			{Name: "Size", Values: []string{"One size", "Travel size"}},
		},
		Variants: []canonical.Variant{
			{SKU: "SKU-1", Attributes: map[string]canonical.Attribute{
				"ai_style": {Value: "Please design a printable style with suitable English text and graphics for the product, 3000 pixels wide."},
				"Size":     {Value: "One size"},
			}},
			{SKU: "SKU-2", Attributes: map[string]canonical.Attribute{
				"ai_style": {Value: "Please design a printable style with suitable English text and graphics for the product, 3000 pixels wide."},
				"Size":     {Value: "Travel size"},
			}},
		},
	}
	pkg := &Package{CategoryID: 8218, SpuName: "Travel tumbler"}
	llm := &stubSequentialSaleLLM{responses: []string{"", `{"value":"Please design an image with suitable text"}`}}
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
							{AttributeValueID: 2, AttributeValue: "默认", AttributeValueEn: "Default"},
						},
					},
				},
			}},
		},
	}, llm)

	resolution := resolver.Resolve(&BuildRequest{}, canonical, pkg)
	if resolution.ValueSanitized {
		t.Fatalf("value sanitized = true, want false when manual review is required")
	}
	if resolution.ValueSanitizationSource != "" {
		t.Fatalf("sanitization source = %q, want empty", resolution.ValueSanitizationSource)
	}
	if len(resolution.skcValueAssignments) != 0 {
		t.Fatalf("skc value assignments = %+v, want none when manual review is required", resolution.skcValueAssignments)
	}
	if resolution.Status != "partial" {
		t.Fatalf("status = %q, want partial", resolution.Status)
	}
	if !resolution.ValuePromptContaminated {
		t.Fatalf("value_prompt_contaminated = false, want true")
	}
	if !strings.Contains(resolution.ValueResolutionNote, "manual review required") {
		t.Fatalf("value resolution note = %q, want manual review hint", resolution.ValueResolutionNote)
	}
}

func TestSaleAttributeResolverKeepsClothingColorPrimaryOverAIStyle(t *testing.T) {
	canonical := &canonical.Product{
		VariantDimensions: []canonical.ScrapedVariantDimension{
			{Name: "ai_style", Values: []string{"Blue Dog Graphic"}},
			{Name: "Color", Values: []string{"White"}},
			{Name: "Size", Values: []string{"M"}},
		},
		Variants: []canonical.Variant{
			{SKU: "SKU-WHITE-M", Attributes: map[string]canonical.Attribute{
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
	canonical := &canonical.Product{
		VariantDimensions: []canonical.ScrapedVariantDimension{
			{Name: "ai_style", Values: []string{"Denim cap graphic"}},
			{Name: "Color", Values: []string{"Washed Black", "Sand colored", "Carbon Gray"}},
			{Name: "Size", Values: []string{"One size"}},
		},
		Variants: []canonical.Variant{
			{SKU: "SKU-BLK", Attributes: map[string]canonical.Attribute{"ai_style": {Value: "Denim cap graphic"}, "Color": {Value: "Washed Black"}, "Size": {Value: "One size"}}},
			{SKU: "SKU-SAND", Attributes: map[string]canonical.Attribute{"ai_style": {Value: "Denim cap graphic"}, "Color": {Value: "Sand colored"}, "Size": {Value: "One size"}}},
			{SKU: "SKU-GRY", Attributes: map[string]canonical.Attribute{"ai_style": {Value: "Denim cap graphic"}, "Color": {Value: "Carbon Gray"}, "Size": {Value: "One size"}}},
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
	canonical := &canonical.Product{
		VariantDimensions: []canonical.ScrapedVariantDimension{
			{Name: "ai_style", Values: []string{"National flag graphic"}},
			{Name: "Color", Values: []string{"Black"}},
			{Name: "Size", Values: []string{"One size"}},
		},
		Variants: []canonical.Variant{
			{SKU: "SKU-BLK", Attributes: map[string]canonical.Attribute{
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
	canonical := &canonical.Product{
		VariantDimensions: []canonical.ScrapedVariantDimension{
			{Name: "source_sds_sku", Values: []string{"MG17701062001"}},
			{Name: "颜色", Values: []string{"白色"}},
		},
		Variants: []canonical.Variant{
			{SKU: "SKU-WHITE", Attributes: map[string]canonical.Attribute{"source_sds_sku": {Value: "MG17701062001"}, "颜色": {Value: "白色"}}},
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
	canonical := &canonical.Product{
		VariantDimensions: []canonical.ScrapedVariantDimension{
			{Name: "颜色", Values: []string{"一桌四椅套装", "月亮椅-矮椅"}},
		},
		Variants: []canonical.Variant{
			{
				SKU: "SKU-SET",
				Attributes: map[string]canonical.Attribute{
					"颜色": {Value: "一桌四椅套装"},
				},
			},
			{
				SKU: "SKU-CHAIR",
				Attributes: map[string]canonical.Attribute{
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
	canonical := &canonical.Product{
		VariantDimensions: []canonical.ScrapedVariantDimension{
			{Name: "颜色", Values: []string{"一桌四椅套装", "月亮椅-矮椅", "月亮椅-高椅", "超轻折叠桌"}},
		},
		Variants: []canonical.Variant{
			{SKU: "SKU-1", Attributes: map[string]canonical.Attribute{"颜色": {Value: "一桌四椅套装"}}},
			{SKU: "SKU-2", Attributes: map[string]canonical.Attribute{"颜色": {Value: "月亮椅-矮椅"}}},
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

func TestSaleAttributeResolverAllowsGenericSecondaryCandidateWithZeroValueFitForCustomValues(t *testing.T) {
	canonical := &canonical.Product{
		VariantDimensions: []canonical.ScrapedVariantDimension{
			{Name: "Color", Values: []string{"white"}},
			{Name: "Size", Values: []string{`30"×40"`, `40"×50"`}},
		},
		Variants: []canonical.Variant{
			{SKU: "SKU-30", Attributes: map[string]canonical.Attribute{"Color": {Value: "white"}, "Size": {Value: `30"×40"`}}},
			{SKU: "SKU-40", Attributes: map[string]canonical.Attribute{"Color": {Value: "white"}, "Size": {Value: `40"×50"`}}},
		},
	}
	pkg := &Package{CategoryID: 3086, SpuName: "Blanket cover"}
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
							{AttributeValueID: 739, AttributeValue: "白色", AttributeValueEn: "White"},
						},
					},
					{
						AttributeID:       87,
						AttributeName:     "尺寸",
						AttributeNameEn:   "Size",
						AttributeType:     1,
						AttributeInputNum: 1,
						AttributeValueInfoList: []sheinattribute.AttributeValue{
							{AttributeValueID: 1, AttributeValue: "One Size", AttributeValueEn: "One Size"},
						},
					},
				},
			}},
		},
		validateCustom: func(attributeID int, attributeValue string, categoryID int, spuName string) (*sheinattribute.ValidateAttributeResponse, error) {
			resp := &sheinattribute.ValidateAttributeResponse{}
			resp.Data.AttributeID = attributeID
			resp.Data.AttributeValueNameMultis = []struct {
				Language                string `json:"language"`
				AttributeValueNameMulti string `json:"attribute_value_name_multi"`
				WarningType             int    `json:"warning_type"`
			}{
				{Language: "en", AttributeValueNameMulti: attributeValue},
			}
			if attributeValue == "30 inch by 40 inch" {
				resp.Data.PreAttributeValueID = 3001
			} else if attributeValue == "40 inch by 50 inch" {
				resp.Data.PreAttributeValueID = 3002
			}
			return resp, nil
		},
		addCustom: func(req *sheinattribute.AddCustomAttributeValueRequest) (*sheinattribute.AddCustomAttributeValueResponse, error) {
			resp := &sheinattribute.AddCustomAttributeValueResponse{}
			valueID := int64(9101)
			if len(req.PreAttributeValueList) == 0 {
				t.Fatal("expected pre attribute value list")
			}
			if req.PreAttributeValueList[0].AttributeValue == "40 inch by 50 inch" {
				valueID = 9102
			}
			resp.Info.Data.CustomAttributeRelation = []sheinattribute.CustomAttributeRelation{{
				PreAttributeValueID: req.PreAttributeValueList[0].PreAttributeValueID,
				AttributeValueID:    valueID,
			}}
			return resp, nil
		},
	}, nil)

	resolution := resolver.Resolve(&BuildRequest{}, canonical, pkg)
	if resolution.PrimaryAttributeID != 27 {
		t.Fatalf("primary attribute id = %d, want 27", resolution.PrimaryAttributeID)
	}
	if resolution.SecondaryAttributeID != 87 {
		t.Fatalf("secondary attribute id = %d, want 87", resolution.SecondaryAttributeID)
	}
	if assignment := resolution.skuValueAssignments[normalizeText(`30"×40"`)]; assignment.AttributeValueID == nil || *assignment.AttributeValueID != 9101 {
		t.Fatalf("first size assignment = %+v, want custom 9101", assignment)
	}
	if assignment := resolution.skuValueAssignments[normalizeText(`40"×50"`)]; assignment.AttributeValueID == nil || *assignment.AttributeValueID != 9102 {
		t.Fatalf("second size assignment = %+v, want custom 9102", assignment)
	}
	foundSecondary := false
	for _, item := range resolution.SelectionSummary {
		if strings.Contains(item, "次销售属性使用源维度 Size 映射到 Size") {
			foundSecondary = true
			break
		}
	}
	if !foundSecondary {
		t.Fatalf("selection summary = %v, want secondary size selection", resolution.SelectionSummary)
	}
}

func TestSaleAttributeResolverDoesNotSelectMismatchedStyleCandidateWhenColorHasZeroFit(t *testing.T) {
	canonical := &canonical.Product{
		VariantDimensions: []canonical.ScrapedVariantDimension{
			{Name: "颜色", Values: []string{"月亮椅-高椅", "月亮椅-矮椅"}},
		},
		Variants: []canonical.Variant{
			{SKU: "SKU-HIGH", Attributes: map[string]canonical.Attribute{"颜色": {Value: "月亮椅-高椅"}}},
			{SKU: "SKU-LOW", Attributes: map[string]canonical.Attribute{"颜色": {Value: "月亮椅-矮椅"}}},
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
