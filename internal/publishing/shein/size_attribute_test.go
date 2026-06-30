package shein

import (
	"testing"

	"task-processor/internal/catalog/canonical"
	sheinattribute "task-processor/internal/shein/api/attribute"
	sheinproduct "task-processor/internal/shein/api/product"
)

func TestBuildPreviewProductIncludesSizeAttributeList(t *testing.T) {
	t.Parallel()

	valueID := 417
	pkg := &Package{
		DraftPayload: &RequestDraft{
			SpuName:      "Oversized Tee",
			SupplierCode: "SKU",
			SizeAttributeList: []sheinproduct.SizeAttribute{{
				AttributeID:                10,
				AttributeExtraValue:        "55",
				RelateSaleAttributeID:      87,
				RelateSaleAttributeValueID: valueID,
			}},
		},
	}

	got := BuildPreviewProduct(pkg)

	if got == nil {
		t.Fatal("BuildPreviewProduct() = nil")
	}
	if len(got.SizeAttributeList) != 1 {
		t.Fatalf("size_attribute_list = %#v, want 1 item", got.SizeAttributeList)
	}
	if got.SizeAttributeList[0].AttributeID != 10 || got.SizeAttributeList[0].RelateSaleAttributeValueID != valueID {
		t.Fatalf("size_attribute_list[0] = %#v", got.SizeAttributeList[0])
	}
}

func TestAssemblerBuildAppliesStructuredProductSizeToPreviewPayload(t *testing.T) {
	t.Parallel()

	sizeMValueID := 417
	sizeLValueID := 568
	assembler := NewAssembler(AssemblerConfig{
		SaleAttributeResolver: assemblerStubSaleAttributeResolver{
			resolution: &SaleAttributeResolution{
				Status:                   "resolved",
				SecondarySourceDimension: "Size",
				SecondaryAttributeID:     87,
				SKUValueAssignments: map[string]ResolvedSaleAttribute{
					normalizeText("M"): {Value: "M", AttributeID: 87, AttributeValueID: &sizeMValueID},
					normalizeText("L"): {Value: "L", AttributeID: 87, AttributeValueID: &sizeLValueID},
				},
			},
		},
	})
	canonicalProduct := &canonical.Product{
		Title: "Oversized Tee",
		Images: []canonical.Image{
			{URL: "https://example.com/main.jpg"},
		},
		Variants: []canonical.Variant{
			{
				SKU:        "SKU-M",
				Attributes: map[string]canonical.Attribute{"Size": {Value: "M"}},
				Stock:      5,
				IsDefault:  true,
			},
			{
				SKU:        "SKU-L",
				Attributes: map[string]canonical.Attribute{"Size": {Value: "L"}},
				Stock:      5,
			},
		},
	}
	productSize := `[[{"content":"尺码","remark":""},{"content":"肩宽(cm/in)","remark":""},{"content":"胸围(cm/in)","remark":""}],[{"content":"M","remark":""},{"content":"55cm/21.7in","remark":""},{"content":"112cm /44.1in","remark":""}],[{"content":"L","remark":""},{"content":"58cm/22.8in","remark":""},{"content":"118cm /46.5in","remark":""}]]`

	pkg := assembler.Build(&BuildRequest{Country: "US", Language: "en", ProductSize: productSize}, canonicalProduct, nil)

	if pkg == nil || pkg.PreviewPayload == nil {
		t.Fatal("expected preview payload")
	}
	got := pkg.PreviewPayload.SizeAttributeList
	if len(got) != 4 {
		t.Fatalf("size_attribute_list = %#v, want 4 items", got)
	}
	if got[0].AttributeID != 10 || got[0].AttributeExtraValue != "55" || got[0].RelateSaleAttributeID != 87 || got[0].RelateSaleAttributeValueID != sizeMValueID {
		t.Fatalf("first size attribute = %#v", got[0])
	}
	if got[3].AttributeID != 15 || got[3].AttributeExtraValue != "118" || got[3].RelateSaleAttributeValueID != sizeLValueID {
		t.Fatalf("last size attribute = %#v", got[3])
	}
}

func TestAssemblerBuildUsesTemplateSizeChartAttributeIDs(t *testing.T) {
	t.Parallel()

	sizeMValueID := 417
	assembler := NewAssembler(AssemblerConfig{
		AttributeResolver: NewAttributeResolver(stubAttributeAPI{
			templates: &sheinattribute.AttributeTemplateInfo{Data: []sheinattribute.AttributeTemplate{{
				AttributeInfos: []sheinattribute.AttributeInfo{
					{
						AttributeID:        20,
						AttributeName:      "胸围 (cm)",
						AttributeNameEn:    "Bust (cm)",
						AttributeType:      2,
						AttributeMode:      0,
						DataDimension:      2,
						AttributeStatus:    3,
						SourceSystemIDList: []int{1, 2, 6, 7},
					},
					{
						AttributeID:        55,
						AttributeName:      "长度 (cm)",
						AttributeNameEn:    "Length (cm)",
						AttributeType:      2,
						AttributeMode:      0,
						DataDimension:      2,
						AttributeStatus:    3,
						SourceSystemIDList: []int{1, 2, 6, 7},
					},
				},
			}}},
		}, nil),
		CategoryResolver: assemblerStubCategoryResolver{
			resolution: &CategoryResolution{Status: "resolved", CategoryID: 1727},
		},
		SaleAttributeResolver: assemblerStubSaleAttributeResolver{
			resolution: &SaleAttributeResolution{
				Status:                   "resolved",
				SecondarySourceDimension: "Size",
				SecondaryAttributeID:     87,
				SKUValueAssignments: map[string]ResolvedSaleAttribute{
					normalizeText("M"): {Value: "M", AttributeID: 87, AttributeValueID: &sizeMValueID},
				},
			},
		},
	})
	canonicalProduct := &canonical.Product{
		Title: "Tee Dress",
		Images: []canonical.Image{
			{URL: "https://example.com/main.jpg"},
		},
		Variants: []canonical.Variant{{
			SKU:        "SKU-M",
			Attributes: map[string]canonical.Attribute{"Size": {Value: "M"}},
			Stock:      5,
			IsDefault:  true,
		}},
	}
	productSize := `[[{"content":"尺码","remark":""},{"content":"胸围(cm/in)","remark":""},{"content":"衣长(cm/in)","remark":""}],[{"content":"M","remark":""},{"content":"112cm /44.1in","remark":""},{"content":"71cm/28in","remark":""}]]`

	pkg := assembler.Build(&BuildRequest{Country: "US", Language: "en", ProductSize: productSize}, canonicalProduct, nil)

	if pkg == nil || pkg.PreviewPayload == nil {
		t.Fatal("expected preview payload")
	}
	got := pkg.PreviewPayload.SizeAttributeList
	if len(got) != 2 {
		t.Fatalf("size_attribute_list = %#v, want 2 items", got)
	}
	if got[0].AttributeID != 20 || got[0].AttributeExtraValue != "112" || got[0].RelateSaleAttributeValueID != sizeMValueID {
		t.Fatalf("bust size attribute = %#v", got[0])
	}
	if got[1].AttributeID != 55 || got[1].AttributeExtraValue != "71" || got[1].RelateSaleAttributeValueID != sizeMValueID {
		t.Fatalf("length size attribute = %#v", got[1])
	}
	if pkg.AttributeResolution == nil || len(pkg.AttributeResolution.SizeChartAttributes) != 2 {
		t.Fatalf("size chart attributes = %#v, want 2", pkg.AttributeResolution)
	}
	if len(pkg.AttributeResolution.PendingAttributeCandidates) != 0 {
		t.Fatalf("pending display candidates = %#v, want size chart fields skipped", pkg.AttributeResolution.PendingAttributeCandidates)
	}
}

func TestAssemblerBuildUsesSizeAttributeHeaderResolverForUnknownSDSHeader(t *testing.T) {
	t.Parallel()

	sizeMValueID := 417
	resolver := &stubSizeAttributeHeaderResolver{
		resolution: SizeAttributeHeaderResolution{
			AttributeIDsByHeader: map[string]int{"下摆围(cm/in)": 58},
			ReviewNotes:          []string{"LLM 尺码表字段匹配: 下摆围 -> Hem"},
		},
	}
	assembler := NewAssembler(AssemblerConfig{
		AttributeResolver: NewAttributeResolver(stubAttributeAPI{
			templates: &sheinattribute.AttributeTemplateInfo{Data: []sheinattribute.AttributeTemplate{{
				AttributeInfos: []sheinattribute.AttributeInfo{{
					AttributeID:        58,
					AttributeName:      "摆围 (cm)",
					AttributeNameEn:    "Hem (cm)",
					AttributeType:      2,
					AttributeMode:      0,
					DataDimension:      2,
					AttributeStatus:    3,
					SourceSystemIDList: []int{1, 2, 6, 7},
				}},
			}}},
		}, nil),
		CategoryResolver: assemblerStubCategoryResolver{
			resolution: &CategoryResolution{Status: "resolved", CategoryID: 1727},
		},
		SaleAttributeResolver: assemblerStubSaleAttributeResolver{
			resolution: &SaleAttributeResolution{
				Status:                   "resolved",
				SecondarySourceDimension: "Size",
				SecondaryAttributeID:     87,
				SKUValueAssignments: map[string]ResolvedSaleAttribute{
					normalizeText("M"): {Value: "M", AttributeID: 87, AttributeValueID: &sizeMValueID},
				},
			},
		},
		SizeAttributeHeaderResolver: resolver,
	})
	canonicalProduct := &canonical.Product{
		Title: "Tee Dress",
		Images: []canonical.Image{
			{URL: "https://example.com/main.jpg"},
		},
		Variants: []canonical.Variant{{
			SKU:        "SKU-M",
			Attributes: map[string]canonical.Attribute{"Size": {Value: "M"}},
			Stock:      5,
			IsDefault:  true,
		}},
	}
	productSize := `[[{"content":"尺码","remark":""},{"content":"下摆围(cm/in)","remark":""}],[{"content":"M","remark":""},{"content":"112cm /44.1in","remark":""}]]`

	pkg := assembler.Build(&BuildRequest{Country: "US", Language: "en", ProductSize: productSize}, canonicalProduct, nil)

	if resolver.calls != 1 {
		t.Fatalf("resolver calls = %d, want 1", resolver.calls)
	}
	if len(resolver.lastInput.Headers) != 1 || resolver.lastInput.Headers[0] != "下摆围(cm/in)" {
		t.Fatalf("resolver headers = %#v, want unknown SDS header", resolver.lastInput.Headers)
	}
	if pkg == nil || pkg.PreviewPayload == nil {
		t.Fatal("expected preview payload")
	}
	got := pkg.PreviewPayload.SizeAttributeList
	if len(got) != 1 {
		t.Fatalf("size_attribute_list = %#v, want 1 item", got)
	}
	if got[0].AttributeID != 58 || got[0].AttributeExtraValue != "112" || got[0].RelateSaleAttributeValueID != sizeMValueID {
		t.Fatalf("size attribute = %#v", got[0])
	}
	if !containsReviewNote(pkg.ReviewNotes, "LLM 尺码表字段匹配") {
		t.Fatalf("review notes = %#v, want LLM size header note", pkg.ReviewNotes)
	}
}

type stubSizeAttributeHeaderResolver struct {
	calls      int
	lastInput  SizeAttributeHeaderResolutionInput
	resolution SizeAttributeHeaderResolution
}

func (s *stubSizeAttributeHeaderResolver) ResolveSizeAttributeHeaders(input SizeAttributeHeaderResolutionInput) SizeAttributeHeaderResolution {
	s.calls++
	s.lastInput = input
	return s.resolution
}
