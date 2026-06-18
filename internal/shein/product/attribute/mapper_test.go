package attribute

import (
	"context"
	"fmt"
	"testing"

	"task-processor/internal/model"
	"task-processor/internal/shein/aicache"
	sheinapi "task-processor/internal/shein/api/attribute"
	productapi "task-processor/internal/shein/api/product"
	sheinctx "task-processor/internal/shein/context"
)

type stubCustomAttributeValueProcessor struct {
	results map[string]CustomAttributeResult
}

func (s stubCustomAttributeValueProcessor) ProcessCustomAttributeValueWithRuntime(_ *sheinctx.TaskContext, _ *MapperRuntimeInput, _ int, attrValue string, _ bool) CustomAttributeResult {
	if result, ok := s.results[attrValue]; ok {
		return result
	}
	return CustomAttributeResult{}
}

type stubPlatformValueFallbackResolver struct {
	result    *PlatformValueFallbackResult
	err       error
	callCount int
	lastReq   *PlatformValueFallbackRequest
}

type stubAttributeAPI struct {
	validateErr       error
	validateCallCount int
}

func (s *stubAttributeAPI) GetAttributeTemplates(categoryID int) (*sheinapi.AttributeTemplateInfo, error) {
	return nil, nil
}

func (s *stubAttributeAPI) ValidateCustomAttributeValue(attributeID int, attributeValue string, categoryID int, spuName string) (*sheinapi.ValidateAttributeResponse, error) {
	s.validateCallCount++
	return nil, s.validateErr
}

func (s *stubAttributeAPI) AddCustomAttributeValue(req *sheinapi.AddCustomAttributeValueRequest) (*sheinapi.AddCustomAttributeValueResponse, error) {
	return nil, nil
}

func (s *stubPlatformValueFallbackResolver) ResolvePlatformValue(_ context.Context, req *PlatformValueFallbackRequest) (*PlatformValueFallbackResult, error) {
	s.callCount++
	s.lastReq = req
	return s.result, s.err
}

func TestMapSingleAttributeValues_DropsUnmatchedValuesWhenCustomAttributesAreDenied(t *testing.T) {
	mapper := &AttributeMapper{
		valueMatcher: NewAttributeValueMatcher(),
		processor: stubCustomAttributeValueProcessor{
			results: map[string]CustomAttributeResult{
				"6.5 X-Wide": {Success: false, PermissionDenied: true, ShouldContinue: true},
			},
		},
	}

	attr := &ResultAttribute{
		AttrID: 87,
		AttrValue: []AttributeValue{
			{ID: -1, Value: "6.5 X-Wide"},
		},
	}

	relations, err := mapper.mapSingleAttributeValues(nil, &MapperRuntimeInput{}, attr, false)
	if err != nil {
		t.Fatalf("mapSingleAttributeValues() error = %v", err)
	}
	if len(relations) != 0 {
		t.Fatalf("relations = %d, want 0", len(relations))
	}
	if len(attr.AttrValue) != 0 {
		t.Fatalf("attr values = %#v, want dropped value when custom attributes are denied", attr.AttrValue)
	}
}

func TestMapSingleAttributeValues_KeepsUnmatchedValuesWhenFailureIsNotPermissionDenied(t *testing.T) {
	mapper := &AttributeMapper{
		valueMatcher: NewAttributeValueMatcher(),
		processor: stubCustomAttributeValueProcessor{
			results: map[string]CustomAttributeResult{
				"6.5 X-Wide": {Success: false, PermissionDenied: false, ShouldContinue: true},
			},
		},
	}

	attr := &ResultAttribute{
		AttrID: 87,
		AttrValue: []AttributeValue{
			{ID: -1, Value: "6.5 X-Wide"},
		},
	}

	relations, err := mapper.mapSingleAttributeValues(nil, &MapperRuntimeInput{}, attr, false)
	if err != nil {
		t.Fatalf("mapSingleAttributeValues() error = %v", err)
	}
	if len(relations) != 0 {
		t.Fatalf("relations = %d, want 0", len(relations))
	}
	if len(attr.AttrValue) != 1 {
		t.Fatalf("attr value count = %d, want 1 when failure is not permission denied", len(attr.AttrValue))
	}
	if attr.AttrValue[0].Value != "6.5 X-Wide" {
		t.Fatalf("attr value = %q, want original value preserved", attr.AttrValue[0].Value)
	}
}

func TestMapSingleAttributeValues_MapsShoeSizeUsingAmazonSizeChart(t *testing.T) {
	mapper := &AttributeMapper{
		valueMatcher: NewAttributeValueMatcher(),
		processor:    stubCustomAttributeValueProcessor{},
	}

	attr := &ResultAttribute{
		AttrID: 87,
		AttrValue: []AttributeValue{
			{ID: -1, Value: "6.5 X-Wide"},
		},
	}

	runtime := &MapperRuntimeInput{
		CategoryID:   8838,
		ProductTitle: "Skechers Women's Go Walk 5 Walking Shoes",
		AmazonProduct: &model.Product{
			Title: "Skechers Women's Go Walk 5 Walking Shoes",
			SizeChart: &model.SizeChart{
				Headers: []string{"Brand Size", "US Size", "UK Size", "EU Size"},
				Rows: [][]string{
					{"6", "6", "3", "36"},
					{"6.5", "6.5", "3.5", "36.5"},
					{"7", "7", "4", "37"},
				},
			},
		},
		AttributeTemplates: &sheinapi.AttributeTemplateInfo{
			Data: []sheinapi.AttributeTemplate{
				{
					AttributeInfos: []sheinapi.AttributeInfo{
						{
							AttributeID: 87,
							AttributeValueInfoList: []sheinapi.AttributeValue{
								{AttributeValueID: 1235, AttributeValue: "235"},
								{AttributeValueID: 1240, AttributeValue: "240"},
							},
						},
					},
				},
			},
		},
	}

	relations, err := mapper.mapSingleAttributeValues(nil, runtime, attr, false)
	if err != nil {
		t.Fatalf("mapSingleAttributeValues() error = %v", err)
	}
	if len(relations) != 0 {
		t.Fatalf("relations = %d, want 0", len(relations))
	}
	if got := attr.AttrValue[0].ID.Int(); got != 1235 {
		t.Fatalf("mapped ID = %d, want 1235", got)
	}
}

func TestMapSingleAttributeValues_DoesNotApplyShoeResolverToNonShoeSizeChart(t *testing.T) {
	mapper := &AttributeMapper{
		valueMatcher: NewAttributeValueMatcher(),
		processor: stubCustomAttributeValueProcessor{
			results: map[string]CustomAttributeResult{
				"8": {Success: false, PermissionDenied: false, ShouldContinue: true},
			},
		},
	}

	attr := &ResultAttribute{
		AttrID: 87,
		AttrValue: []AttributeValue{
			{ID: -1, Value: "8"},
		},
	}

	runtime := &MapperRuntimeInput{
		CategoryID:   8838,
		ProductTitle: "Women's Casual Dress",
		AmazonProduct: &model.Product{
			Title: "Women's Casual Dress",
			SizeChart: &model.SizeChart{
				Headers: []string{"Size", "Bust", "Waist", "Hip"},
				Rows: [][]string{
					{"6", "84", "66", "90"},
					{"8", "88", "70", "94"},
					{"10", "92", "74", "98"},
				},
			},
		},
		AttributeTemplates: &sheinapi.AttributeTemplateInfo{
			Data: []sheinapi.AttributeTemplate{
				{
					AttributeInfos: []sheinapi.AttributeInfo{
						{
							AttributeID: 87,
							AttributeValueInfoList: []sheinapi.AttributeValue{
								{AttributeValueID: 1, AttributeValue: "240"},
								{AttributeValueID: 2, AttributeValue: "245"},
								{AttributeValueID: 3, AttributeValue: "250"},
							},
						},
					},
				},
			},
		},
	}

	relations, err := mapper.mapSingleAttributeValues(nil, runtime, attr, false)
	if err != nil {
		t.Fatalf("mapSingleAttributeValues() error = %v", err)
	}
	if len(relations) != 0 {
		t.Fatalf("relations = %d, want 0", len(relations))
	}
	if got := attr.AttrValue[0].ID.Int(); got != -1 {
		t.Fatalf("mapped ID = %d, want unresolved value for non-shoe size chart", got)
	}
}

func TestDetectPlatformValueDomain_IdentifiesMetricShoeSize(t *testing.T) {
	attrInfo := &sheinapi.AttributeInfo{
		AttributeID:   87,
		AttributeName: "Size",
		AttributeValueInfoList: []sheinapi.AttributeValue{
			{AttributeValueID: 1, AttributeValue: "230"},
			{AttributeValueID: 2, AttributeValue: "235"},
			{AttributeValueID: 3, AttributeValue: "240"},
		},
	}

	if got := detectPlatformValueDomain(attrInfo); got != platformValueDomainShoeMetricSize {
		t.Fatalf("detectPlatformValueDomain() = %q, want %q", got, platformValueDomainShoeMetricSize)
	}
}

func TestDetectPlatformValueDomain_DoesNotTreatApparelNumericValuesAsShoeSize(t *testing.T) {
	attrInfo := &sheinapi.AttributeInfo{
		AttributeID:   87,
		AttributeName: "Size",
		AttributeValueInfoList: []sheinapi.AttributeValue{
			{AttributeValueID: 1, AttributeValue: "6"},
			{AttributeValueID: 2, AttributeValue: "8"},
			{AttributeValueID: 3, AttributeValue: "10"},
		},
	}

	if got := detectPlatformValueDomain(attrInfo); got == platformValueDomainShoeMetricSize {
		t.Fatalf("detectPlatformValueDomain() = %q, want non-shoe domain", got)
	}
}

func TestMapSingleAttributeValues_MapsApparelAlphaSizeUsingNormalizedRules(t *testing.T) {
	mapper := &AttributeMapper{
		valueMatcher: NewAttributeValueMatcher(),
		processor:    stubCustomAttributeValueProcessor{},
	}

	attr := &ResultAttribute{
		AttrID: 87,
		AttrValue: []AttributeValue{
			{ID: -1, Value: "Small"},
		},
	}

	runtime := &MapperRuntimeInput{
		CategoryID:   8838,
		ProductTitle: "Women's Casual Blouse",
		AmazonProduct: &model.Product{
			Title: "Women's Casual Blouse",
			SizeChart: &model.SizeChart{
				Headers: []string{"Brand Size", "Bust (in)", "Sleeve Length (in)", "Shoulder (in)", "Length (in)"},
				Rows: [][]string{
					{"Small", "37.5", "8", "14", "27"},
					{"Medium", "39.8", "8.3", "14.6", "27.2"},
				},
			},
		},
		AttributeTemplates: &sheinapi.AttributeTemplateInfo{
			Data: []sheinapi.AttributeTemplate{
				{
					AttributeInfos: []sheinapi.AttributeInfo{
						{
							AttributeID: 87,
							AttributeValueInfoList: []sheinapi.AttributeValue{
								{AttributeValueID: 11, AttributeValue: "S"},
								{AttributeValueID: 12, AttributeValue: "M"},
								{AttributeValueID: 13, AttributeValue: "L"},
							},
						},
					},
				},
			},
		},
	}

	relations, err := mapper.mapSingleAttributeValues(nil, runtime, attr, false)
	if err != nil {
		t.Fatalf("mapSingleAttributeValues() error = %v", err)
	}
	if len(relations) != 0 {
		t.Fatalf("relations = %d, want 0", len(relations))
	}
	if got := attr.AttrValue[0].ID.Int(); got != 11 {
		t.Fatalf("mapped ID = %d, want 11", got)
	}
}

func TestMapSingleAttributeValues_MapsApparelNumericSizeUsingChartRow(t *testing.T) {
	mapper := &AttributeMapper{
		valueMatcher: NewAttributeValueMatcher(),
		processor:    stubCustomAttributeValueProcessor{},
	}

	attr := &ResultAttribute{
		AttrID: 87,
		AttrValue: []AttributeValue{
			{ID: -1, Value: "Medium"},
		},
	}

	runtime := &MapperRuntimeInput{
		CategoryID:   8838,
		ProductTitle: "Women's Casual Dress",
		AmazonProduct: &model.Product{
			Title: "Women's Casual Dress",
			SizeChart: &model.SizeChart{
				Headers: []string{"Brand Size", "US Size", "Bust (in)", "Waist Size (in)"},
				Rows: [][]string{
					{"Small", "4-6", "35.0", "27.0"},
					{"Medium", "8-10", "37.0", "29.0"},
					{"Large", "12-14", "39.0", "31.0"},
				},
			},
		},
		AttributeTemplates: &sheinapi.AttributeTemplateInfo{
			Data: []sheinapi.AttributeTemplate{
				{
					AttributeInfos: []sheinapi.AttributeInfo{
						{
							AttributeID: 87,
							AttributeValueInfoList: []sheinapi.AttributeValue{
								{AttributeValueID: 21, AttributeValue: "4"},
								{AttributeValueID: 22, AttributeValue: "6"},
								{AttributeValueID: 23, AttributeValue: "8"},
								{AttributeValueID: 24, AttributeValue: "10"},
								{AttributeValueID: 25, AttributeValue: "12"},
							},
						},
					},
				},
			},
		},
	}

	relations, err := mapper.mapSingleAttributeValues(nil, runtime, attr, false)
	if err != nil {
		t.Fatalf("mapSingleAttributeValues() error = %v", err)
	}
	if len(relations) != 0 {
		t.Fatalf("relations = %d, want 0", len(relations))
	}
	if got := attr.AttrValue[0].ID.Int(); got != 23 {
		t.Fatalf("mapped ID = %d, want 23", got)
	}
}

func TestMapSingleAttributeValues_UsesFallbackResolverAfterDeterministicResolversFail(t *testing.T) {
	fallback := &stubPlatformValueFallbackResolver{
		result: &PlatformValueFallbackResult{
			ResolvedValue: "M",
			Confidence:    0.92,
			Reason:        "LLM inferred Medium maps to M",
		},
	}
	mapper := &AttributeMapper{
		valueMatcher: NewAttributeValueMatcher(),
		processor:    stubCustomAttributeValueProcessor{},
	}

	attr := &ResultAttribute{
		AttrID: 87,
		AttrValue: []AttributeValue{
			{ID: -1, Value: "Med"},
		},
	}

	runtime := &MapperRuntimeInput{
		ProductTitle: "Women's Casual Blouse",
		AttributeTemplates: &sheinapi.AttributeTemplateInfo{
			Data: []sheinapi.AttributeTemplate{{
				AttributeInfos: []sheinapi.AttributeInfo{{
					AttributeID: 87,
					AttributeValueInfoList: []sheinapi.AttributeValue{
						{AttributeValueID: 11, AttributeValue: "S"},
						{AttributeValueID: 12, AttributeValue: "M"},
						{AttributeValueID: 13, AttributeValue: "L"},
					},
				}},
			}},
		},
		FallbackValueResolver: fallback,
	}

	relations, err := mapper.mapSingleAttributeValues(nil, runtime, attr, false)
	if err != nil {
		t.Fatalf("mapSingleAttributeValues() error = %v", err)
	}
	if len(relations) != 0 {
		t.Fatalf("relations = %d, want 0", len(relations))
	}
	if fallback.callCount != 1 {
		t.Fatalf("fallback call count = %d, want 1", fallback.callCount)
	}
	if got := attr.AttrValue[0].ID.Int(); got != 12 {
		t.Fatalf("mapped ID = %d, want 12", got)
	}
}

func TestMapSingleAttributeValues_IgnoresLowConfidenceFallbackResult(t *testing.T) {
	fallback := &stubPlatformValueFallbackResolver{
		result: &PlatformValueFallbackResult{
			ResolvedValue: "M",
			Confidence:    0.41,
			Reason:        "low confidence guess",
		},
	}
	mapper := &AttributeMapper{
		valueMatcher: NewAttributeValueMatcher(),
		processor: stubCustomAttributeValueProcessor{
			results: map[string]CustomAttributeResult{
				"Med": {Success: false, PermissionDenied: false, ShouldContinue: true},
			},
		},
	}

	attr := &ResultAttribute{
		AttrID: 87,
		AttrValue: []AttributeValue{
			{ID: -1, Value: "Med"},
		},
	}

	runtime := &MapperRuntimeInput{
		ProductTitle: "Women's Casual Blouse",
		AttributeTemplates: &sheinapi.AttributeTemplateInfo{
			Data: []sheinapi.AttributeTemplate{{
				AttributeInfos: []sheinapi.AttributeInfo{{
					AttributeID: 87,
					AttributeValueInfoList: []sheinapi.AttributeValue{
						{AttributeValueID: 11, AttributeValue: "S"},
						{AttributeValueID: 12, AttributeValue: "M"},
						{AttributeValueID: 13, AttributeValue: "L"},
					},
				}},
			}},
		},
		FallbackValueResolver: fallback,
		FallbackMinConfidence: 0.8,
	}

	relations, err := mapper.mapSingleAttributeValues(nil, runtime, attr, false)
	if err != nil {
		t.Fatalf("mapSingleAttributeValues() error = %v", err)
	}
	if len(relations) != 0 {
		t.Fatalf("relations = %d, want 0", len(relations))
	}
	if fallback.callCount != 1 {
		t.Fatalf("fallback call count = %d, want 1", fallback.callCount)
	}
	if got := attr.AttrValue[0].ID.Int(); got != -1 {
		t.Fatalf("mapped ID = %d, want unresolved when confidence is too low", got)
	}
}

func TestMapSingleAttributeValues_UsesCachedFallbackResult(t *testing.T) {
	cache := aicache.New(nil)
	runtime := &MapperRuntimeInput{
		ProductTitle: "Women's Casual Blouse",
		AttributeTemplates: &sheinapi.AttributeTemplateInfo{
			Data: []sheinapi.AttributeTemplate{{
				AttributeInfos: []sheinapi.AttributeInfo{{
					AttributeID: 87,
					AttributeValueInfoList: []sheinapi.AttributeValue{
						{AttributeValueID: 11, AttributeValue: "S"},
						{AttributeValueID: 12, AttributeValue: "M"},
						{AttributeValueID: 13, AttributeValue: "L"},
					},
				}},
			}},
		},
		FallbackCache: cache,
	}
	cacheKey := runtime.buildFallbackCacheKey(platformValueDomainApparelAlphaSize, 87, "Med", map[string]int{"S": 11, "M": 12, "L": 13})
	cache.Set(aicache.TypeAttrValueFallback, cacheKey, PlatformValueFallbackResult{
		ResolvedValue: "M",
		Confidence:    0.93,
		Reason:        "cached",
	})

	fallback := &stubPlatformValueFallbackResolver{}
	runtime.FallbackValueResolver = fallback

	mapper := &AttributeMapper{
		valueMatcher: NewAttributeValueMatcher(),
		processor:    stubCustomAttributeValueProcessor{},
	}
	attr := &ResultAttribute{
		AttrID: 87,
		AttrValue: []AttributeValue{
			{ID: -1, Value: "Med"},
		},
	}

	relations, err := mapper.mapSingleAttributeValues(nil, runtime, attr, false)
	if err != nil {
		t.Fatalf("mapSingleAttributeValues() error = %v", err)
	}
	if len(relations) != 0 {
		t.Fatalf("relations = %d, want 0", len(relations))
	}
	if fallback.callCount != 0 {
		t.Fatalf("fallback call count = %d, want 0 when cache hits", fallback.callCount)
	}
	if got := attr.AttrValue[0].ID.Int(); got != 12 {
		t.Fatalf("mapped ID = %d, want 12", got)
	}
}

func TestMapSingleAttributeValues_NarrowsMixedSizeSystemCandidatesForNonCustomGenericFallback(t *testing.T) {
	fallback := &stubPlatformValueFallbackResolver{
		result: &PlatformValueFallbackResult{
			ResolvedValue: "US6.5",
			Confidence:    0.96,
			Reason:        "source size is US width-based shoe size",
		},
	}
	mapper := &AttributeMapper{
		valueMatcher: NewAttributeValueMatcher(),
		processor:    stubCustomAttributeValueProcessor{},
	}

	attr := &ResultAttribute{
		AttrID: 87,
		AttrValue: []AttributeValue{
			{ID: -1, Value: "6.5 Wide"},
		},
	}

	runtime := &MapperRuntimeInput{
		CategoryID:   8838,
		ProductTitle: "Skechers Women's Go Walk 5 Walking Shoes",
		AmazonProduct: &model.Product{
			Title: "Skechers Women's Go Walk 5 Walking Shoes",
			SizeChart: &model.SizeChart{
				Headers: []string{"US Size", "CN Size"},
				Rows: [][]string{
					{"6.5", "235"},
				},
			},
		},
		AttributeTemplates: &sheinapi.AttributeTemplateInfo{
			Data: []sheinapi.AttributeTemplate{
				{
					AttributeInfos: []sheinapi.AttributeInfo{
						{
							AttributeID:   87,
							AttributeName: "Size",
							AttributeType: 2,
							AttributeValueInfoList: []sheinapi.AttributeValue{
								{AttributeValueID: 1235, AttributeValue: "235"},
								{AttributeValueID: 2235, AttributeValue: "US6.5"},
								{AttributeValueID: 2240, AttributeValue: "US7"},
							},
						},
					},
				},
			},
		},
		FallbackValueResolver: fallback,
	}

	relations, err := mapper.mapSingleAttributeValues(nil, runtime, attr, false)
	if err != nil {
		t.Fatalf("mapSingleAttributeValues() error = %v", err)
	}
	if len(relations) != 0 {
		t.Fatalf("relations = %d, want 0", len(relations))
	}
	if fallback.callCount != 1 {
		t.Fatalf("fallback call count = %d, want 1", fallback.callCount)
	}
	if fallback.lastReq == nil {
		t.Fatalf("fallback request is nil")
	}
	if got := fallback.lastReq.PlatformValues; len(got) != 2 || got[0] != "us6.5" || got[1] != "us7" {
		t.Fatalf("fallback platform values = %#v, want only narrowed US candidates", got)
	}
	if got := attr.AttrValue[0].ID.Int(); got != 2235 {
		t.Fatalf("mapped ID = %d, want 2235", got)
	}
}

func TestMapSingleAttributeValues_CachesCustomPermissionDeniedByCategoryAndAttribute(t *testing.T) {
	attrAPI := &stubAttributeAPI{
		validateErr: fmt.Errorf("API错误 [0]: 验证自定义属性值失败: 没有自定义属性值权限"),
	}
	mapper := &AttributeMapper{
		valueMatcher: NewAttributeValueMatcher(),
		processor:    NewCustomAttributeProcessor(),
	}

	attr := &ResultAttribute{
		AttrID: 87,
		AttrValue: []AttributeValue{
			{ID: -1, Value: "6 Wide"},
			{ID: -1, Value: "7.5 Wide"},
		},
	}

	runtime := &MapperRuntimeInput{
		CategoryID:         4455,
		ProductTitle:       "Skechers Women's Go Walk 5 Walking Shoes",
		AttributeTemplates: &sheinapi.AttributeTemplateInfo{Data: []sheinapi.AttributeTemplate{{AttributeInfos: []sheinapi.AttributeInfo{{AttributeID: 87, AttributeType: 2}}}}},
		AttributeAPI:       attrAPI,
	}

	relations, err := mapper.mapSingleAttributeValues(nil, runtime, attr, false)
	if err != nil {
		t.Fatalf("mapSingleAttributeValues() error = %v", err)
	}
	if len(relations) != 0 {
		t.Fatalf("relations = %d, want 0", len(relations))
	}
	if attrAPI.validateCallCount != 1 {
		t.Fatalf("validate custom attribute call count = %d, want 1 after permission denial is cached", attrAPI.validateCallCount)
	}
	if len(attr.AttrValue) != 0 {
		t.Fatalf("attr values = %#v, want all unmatched values dropped after cached permission denial", attr.AttrValue)
	}
}

func TestMapSingleAttributeValues_NarrowsMixedSizeSystemCandidatesByTargetSitePreference(t *testing.T) {
	fallback := &stubPlatformValueFallbackResolver{
		result: &PlatformValueFallbackResult{
			ResolvedValue: "US7",
			Confidence:    0.94,
			Reason:        "target site is US and source value is ambiguous numeric size",
		},
	}
	mapper := &AttributeMapper{
		valueMatcher: NewAttributeValueMatcher(),
		processor:    stubCustomAttributeValueProcessor{},
	}

	attr := &ResultAttribute{
		AttrID: 87,
		AttrValue: []AttributeValue{
			{ID: -1, Value: "7"},
		},
	}

	runtime := &MapperRuntimeInput{
		CategoryID:   8838,
		ProductTitle: "Skechers Women's Go Walk 5 Walking Shoes",
		Region:       "US",
		SiteList: []productapi.SiteInfo{{
			MainSite:    "shein",
			SubSiteList: []string{"shein-us"},
		}},
		AttributeTemplates: &sheinapi.AttributeTemplateInfo{
			Data: []sheinapi.AttributeTemplate{
				{
					AttributeInfos: []sheinapi.AttributeInfo{
						{
							AttributeID:   87,
							AttributeName: "Size",
							AttributeType: 2,
							AttributeValueInfoList: []sheinapi.AttributeValue{
								{AttributeValueID: 2235, AttributeValue: "US6.5"},
								{AttributeValueID: 2240, AttributeValue: "US7"},
								{AttributeValueID: 3337, AttributeValue: "BR37"},
								{AttributeValueID: 3340, AttributeValue: "BR38"},
							},
						},
					},
				},
			},
		},
		FallbackValueResolver: fallback,
	}

	relations, err := mapper.mapSingleAttributeValues(nil, runtime, attr, false)
	if err != nil {
		t.Fatalf("mapSingleAttributeValues() error = %v", err)
	}
	if len(relations) != 0 {
		t.Fatalf("relations = %d, want 0", len(relations))
	}
	if fallback.callCount != 0 {
		t.Fatalf("fallback call count = %d, want 0 after local structured size resolution", fallback.callCount)
	}
	if got := attr.AttrValue[0].ID.Int(); got != 2240 {
		t.Fatalf("mapped ID = %d, want 2240", got)
	}
}

func TestMapSingleAttributeValues_UsesMatchedChartColumnBeforeTargetSitePreference(t *testing.T) {
	fallback := &stubPlatformValueFallbackResolver{
		result: &PlatformValueFallbackResult{
			ResolvedValue: "US7",
			Confidence:    0.94,
			Reason:        "raw size matched US size column in the source chart",
		},
	}
	mapper := &AttributeMapper{
		valueMatcher: NewAttributeValueMatcher(),
		processor:    stubCustomAttributeValueProcessor{},
	}

	attr := &ResultAttribute{
		AttrID: 87,
		AttrValue: []AttributeValue{
			{ID: -1, Value: "7"},
		},
	}

	runtime := &MapperRuntimeInput{
		CategoryID:   8838,
		ProductTitle: "Skechers Women's Go Walk 5 Walking Shoes",
		Region:       "US",
		SiteList: []productapi.SiteInfo{{
			MainSite:    "shein",
			SubSiteList: []string{"shein-us"},
		}},
		AmazonProduct: &model.Product{
			Title: "Skechers Women's Go Walk 5 Walking Shoes",
			SizeChart: &model.SizeChart{
				Headers: []string{"BR Size", "US Size", "UK Size"},
				Rows: [][]string{
					{"35", "7", "4"},
				},
			},
		},
		AttributeTemplates: &sheinapi.AttributeTemplateInfo{
			Data: []sheinapi.AttributeTemplate{
				{
					AttributeInfos: []sheinapi.AttributeInfo{
						{
							AttributeID:   87,
							AttributeName: "Size",
							AttributeType: 2,
							AttributeValueInfoList: []sheinapi.AttributeValue{
								{AttributeValueID: 2240, AttributeValue: "US7"},
								{AttributeValueID: 3337, AttributeValue: "BR37"},
							},
						},
					},
				},
			},
		},
		FallbackValueResolver: fallback,
	}

	relations, err := mapper.mapSingleAttributeValues(nil, runtime, attr, false)
	if err != nil {
		t.Fatalf("mapSingleAttributeValues() error = %v", err)
	}
	if len(relations) != 0 {
		t.Fatalf("relations = %d, want 0", len(relations))
	}
	if fallback.callCount != 0 {
		t.Fatalf("fallback call count = %d, want 0 after local structured size resolution", fallback.callCount)
	}
	if got := attr.AttrValue[0].ID.Int(); got != 2240 {
		t.Fatalf("mapped ID = %d, want 2240", got)
	}
}

func TestMapSingleAttributeValues_ResolvesStructuredShoeSizeBeforeFallback(t *testing.T) {
	fallback := &stubPlatformValueFallbackResolver{
		result: &PlatformValueFallbackResult{
			ResolvedValue: "US7 Wide",
			Confidence:    0.95,
			Reason:        "should not be used",
		},
	}
	mapper := &AttributeMapper{
		valueMatcher: NewAttributeValueMatcher(),
		processor:    stubCustomAttributeValueProcessor{},
	}

	attr := &ResultAttribute{
		AttrID: 87,
		AttrValue: []AttributeValue{
			{ID: -1, Value: "7 Wide"},
		},
	}

	runtime := &MapperRuntimeInput{
		CategoryID:   8838,
		ProductTitle: "Skechers Women's Go Walk 5 Walking Shoes",
		AmazonProduct: &model.Product{
			Title: "Skechers Women's Go Walk 5 Walking Shoes",
			SizeChart: &model.SizeChart{
				Headers: []string{"US Size", "UK Size", "EU Size"},
				Rows: [][]string{
					{"7", "4", "37"},
				},
			},
		},
		AttributeTemplates: &sheinapi.AttributeTemplateInfo{
			Data: []sheinapi.AttributeTemplate{
				{
					AttributeInfos: []sheinapi.AttributeInfo{
						{
							AttributeID:   87,
							AttributeName: "Size",
							AttributeType: 2,
							AttributeValueInfoList: []sheinapi.AttributeValue{
								{AttributeValueID: 2240, AttributeValue: "US7"},
								{AttributeValueID: 2241, AttributeValue: "US7 Wide"},
								{AttributeValueID: 3337, AttributeValue: "BR37"},
							},
						},
					},
				},
			},
		},
		FallbackValueResolver: fallback,
	}

	relations, err := mapper.mapSingleAttributeValues(nil, runtime, attr, false)
	if err != nil {
		t.Fatalf("mapSingleAttributeValues() error = %v", err)
	}
	if len(relations) != 0 {
		t.Fatalf("relations = %d, want 0", len(relations))
	}
	if fallback.callCount != 0 {
		t.Fatalf("fallback call count = %d, want 0", fallback.callCount)
	}
	if got := attr.AttrValue[0].ID.Int(); got != 2241 {
		t.Fatalf("mapped ID = %d, want 2241", got)
	}
}

func TestMapSingleAttributeValues_FallsBackWhenStructuredShoeSizeCannotResolveUniquely(t *testing.T) {
	fallback := &stubPlatformValueFallbackResolver{
		result: &PlatformValueFallbackResult{
			ResolvedValue: "US7",
			Confidence:    0.94,
			Reason:        "width-specific value unavailable on platform",
		},
	}
	mapper := &AttributeMapper{
		valueMatcher: NewAttributeValueMatcher(),
		processor:    stubCustomAttributeValueProcessor{},
	}

	attr := &ResultAttribute{
		AttrID: 87,
		AttrValue: []AttributeValue{
			{ID: -1, Value: "7 Wide"},
		},
	}

	runtime := &MapperRuntimeInput{
		CategoryID:   8838,
		ProductTitle: "Skechers Women's Go Walk 5 Walking Shoes",
		AmazonProduct: &model.Product{
			Title: "Skechers Women's Go Walk 5 Walking Shoes",
			SizeChart: &model.SizeChart{
				Headers: []string{"US Size", "UK Size", "EU Size"},
				Rows: [][]string{
					{"7", "4", "37"},
				},
			},
		},
		AttributeTemplates: &sheinapi.AttributeTemplateInfo{
			Data: []sheinapi.AttributeTemplate{
				{
					AttributeInfos: []sheinapi.AttributeInfo{
						{
							AttributeID:   87,
							AttributeName: "Size",
							AttributeType: 2,
							AttributeValueInfoList: []sheinapi.AttributeValue{
								{AttributeValueID: 2240, AttributeValue: "US7"},
								{AttributeValueID: 3337, AttributeValue: "BR37"},
							},
						},
					},
				},
			},
		},
		FallbackValueResolver: fallback,
	}

	relations, err := mapper.mapSingleAttributeValues(nil, runtime, attr, false)
	if err != nil {
		t.Fatalf("mapSingleAttributeValues() error = %v", err)
	}
	if len(relations) != 0 {
		t.Fatalf("relations = %d, want 0", len(relations))
	}
	if fallback.callCount != 1 {
		t.Fatalf("fallback call count = %d, want 1", fallback.callCount)
	}
	if got := attr.AttrValue[0].ID.Int(); got != 2240 {
		t.Fatalf("mapped ID = %d, want 2240", got)
	}
}

func TestMapSingleAttributeValues_ResolvesSingleSizeOverRangeCandidateBeforeFallback(t *testing.T) {
	fallback := &stubPlatformValueFallbackResolver{
		result: &PlatformValueFallbackResult{
			ResolvedValue: "US8",
			Confidence:    0.95,
			Reason:        "should not be used when single-size candidate exists",
		},
	}
	mapper := &AttributeMapper{
		valueMatcher: NewAttributeValueMatcher(),
		processor:    stubCustomAttributeValueProcessor{},
	}

	attr := &ResultAttribute{
		AttrID: 87,
		AttrValue: []AttributeValue{
			{ID: -1, Value: "8"},
		},
	}

	runtime := &MapperRuntimeInput{
		CategoryID:   8838,
		ProductTitle: "Skechers Women's Go Walk 5 Walking Shoes",
		AmazonProduct: &model.Product{
			Title: "Skechers Women's Go Walk 5 Walking Shoes",
			SizeChart: &model.SizeChart{
				Headers: []string{"US Size", "UK Size", "EU Size"},
				Rows: [][]string{
					{"8", "5", "38"},
				},
			},
		},
		AttributeTemplates: &sheinapi.AttributeTemplateInfo{
			Data: []sheinapi.AttributeTemplate{{
				AttributeInfos: []sheinapi.AttributeInfo{{
					AttributeID:   87,
					AttributeName: "Size",
					AttributeType: 2,
					AttributeValueInfoList: []sheinapi.AttributeValue{
						{AttributeValueID: 8001, AttributeValue: "US8"},
						{AttributeValueID: 8002, AttributeValue: "US8-9"},
						{AttributeValueID: 8003, AttributeValue: "US8W"},
					},
				}},
			}},
		},
		FallbackValueResolver: fallback,
	}

	relations, err := mapper.mapSingleAttributeValues(nil, runtime, attr, false)
	if err != nil {
		t.Fatalf("mapSingleAttributeValues() error = %v", err)
	}
	if len(relations) != 0 {
		t.Fatalf("relations = %d, want 0", len(relations))
	}
	if fallback.callCount != 0 {
		t.Fatalf("fallback call count = %d, want 0", fallback.callCount)
	}
	if got := attr.AttrValue[0].ID.Int(); got != 8001 {
		t.Fatalf("mapped ID = %d, want 8001", got)
	}
}

func TestMapSingleAttributeValues_RemapSizeLikeValueEvenWhenInitialIDIsPositive(t *testing.T) {
	mapper := &AttributeMapper{
		valueMatcher: NewAttributeValueMatcher(),
		processor:    stubCustomAttributeValueProcessor{},
	}

	attr := &ResultAttribute{
		AttrID: 87,
		AttrValue: []AttributeValue{
			{ID: 710, Value: "10.5"},
		},
	}

	runtime := &MapperRuntimeInput{
		CategoryID:   8838,
		ProductTitle: "Skechers Women's Go Walk 5 Walking Shoes",
		AttributeTemplates: &sheinapi.AttributeTemplateInfo{
			Data: []sheinapi.AttributeTemplate{{
				AttributeInfos: []sheinapi.AttributeInfo{{
					AttributeID:   87,
					AttributeName: "Size",
					AttributeType: 2,
					AttributeValueInfoList: []sheinapi.AttributeValue{
						{AttributeValueID: 710, AttributeValue: "7"},
						{AttributeValueID: 1355, AttributeValue: "10.5"},
					},
				}},
			}},
		},
	}

	relations, err := mapper.mapSingleAttributeValues(nil, runtime, attr, false)
	if err != nil {
		t.Fatalf("mapSingleAttributeValues() error = %v", err)
	}
	if len(relations) != 0 {
		t.Fatalf("relations = %d, want 0", len(relations))
	}
	if got := attr.AttrValue[0].ID.Int(); got != 1355 {
		t.Fatalf("mapped ID = %d, want 1355", got)
	}
}
