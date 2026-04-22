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
	if resolution.SecondaryAttributeID != 87 {
		t.Fatalf("secondary attribute id = %d, want 87", resolution.SecondaryAttributeID)
	}

	variants := common.BuildVariants(canonical)
	groups := buildVariantGroups(variants, &common.ImageSet{MainImage: "main.jpg"}, resolution)
	if len(groups) != 2 {
		t.Fatalf("group count = %d, want 2", len(groups))
	}
	if groups[0].skcName != "红色" || groups[1].skcName != "蓝色" {
		t.Fatalf("group names = %q, %q; want 红色/蓝色", groups[0].skcName, groups[1].skcName)
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

func TestSaleAttributeResolverSelectsStyleCandidateWhenNameMatchedColorHasZeroFit(t *testing.T) {
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
	if resolution.PrimaryAttributeID != 301 {
		t.Fatalf("primary attribute id = %d, want 301", resolution.PrimaryAttributeID)
	}
	if resolution.Status != "resolved" {
		t.Fatalf("status = %q, want resolved", resolution.Status)
	}
	if len(resolution.Candidates) < 2 {
		t.Fatalf("expected both Color and Style candidates, got %+v", resolution.Candidates)
	}

	var colorCandidate, styleCandidate *SaleAttributeCandidateInfo
	for i := range resolution.Candidates {
		candidate := &resolution.Candidates[i]
		switch candidate.Name {
		case "Color":
			colorCandidate = candidate
		case "Style":
			styleCandidate = candidate
		}
	}
	if colorCandidate == nil || styleCandidate == nil {
		t.Fatalf("expected Color and Style candidates, got %+v", resolution.Candidates)
	}
	if colorCandidate.SelectedScope != "" {
		t.Fatalf("color candidate should remain unselected: %+v", colorCandidate)
	}
	if styleCandidate.SelectedScope != "skc" {
		t.Fatalf("style candidate selected_scope = %q, want skc", styleCandidate.SelectedScope)
	}
}
