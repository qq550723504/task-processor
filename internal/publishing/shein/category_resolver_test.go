package shein

import (
	"errors"
	"strings"
	"testing"

	"task-processor/internal/productenrich"
	sheinapi "task-processor/internal/shein/api"
	sheincategory "task-processor/internal/shein/api/category"
	sheincategoryselector "task-processor/internal/shein/category"
)

type stubCategoryAPI struct {
	suggestResponse  *sheincategory.SuggestCategoryResponse
	suggestErr       error
	categoryInfoByID map[int]*sheincategory.CategoryInfo
	categoryInfo     *sheincategory.CategoryInfo
	categoryErr      error
	categoryTree     *sheincategory.CategoryTreeResponse
	categoryTreeErr  error
}

func (s stubCategoryAPI) GetCategory(categoryID int) (*sheincategory.CategoryInfo, error) {
	if s.categoryInfoByID != nil {
		if info, ok := s.categoryInfoByID[categoryID]; ok {
			return info, s.categoryErr
		}
		return nil, s.categoryErr
	}
	return s.categoryInfo, s.categoryErr
}

func (s stubCategoryAPI) GetCategoryTree() (*sheincategory.CategoryTreeResponse, error) {
	return s.categoryTree, s.categoryTreeErr
}

func (s stubCategoryAPI) SuggestCategoryByText(productInfo string) (*sheincategory.SuggestCategoryResponse, error) {
	return s.suggestResponse, s.suggestErr
}

func TestCategoryResolverReturnsPartialWhenSuggestCategoryFails(t *testing.T) {
	resolver := NewCategoryResolverWithFallbacks(stubCategoryAPI{
		suggestErr: &sheinapi.AuthenticationExpiredError{
			Code:     "20302",
			Message:  "认证已过期，需要重新登录",
			TenantID: 227,
			ShopID:   869,
		},
	}, stubCategorySuggestFallback{
		err: &sheinapi.AuthenticationExpiredError{
			Code:     "20302",
			Message:  "认证已过期，需要重新登录",
			TenantID: 227,
			ShopID:   869,
		},
	}, nil)

	resolution := resolver.Resolve(&BuildRequest{Text: "Sports Shoes"}, &productenrich.CanonicalProduct{}, &Package{
		CategoryName: "Product",
		CategoryPath: []string{"General", "Product"},
	})

	if resolution.Status != "partial" {
		t.Fatalf("expected partial status, got %q", resolution.Status)
	}
	if resolution.Source != "fallback" {
		t.Fatalf("expected fallback source, got %q", resolution.Source)
	}
	if len(resolution.ReviewNotes) == 0 {
		t.Fatal("expected unresolved review note")
	}
}

func TestFormatCategoryResolutionAPIErrorUsesRawErrorWhenNotAuthExpired(t *testing.T) {
	note := formatCategoryResolutionAPIError(errors.New("temporary upstream error"))
	if note != "SHEIN 类目在线解析失败: temporary upstream error" {
		t.Fatalf("unexpected note: %q", note)
	}
}

type stubCategoryTreeFallback struct {
	selectedID int
	err        error
}

func (s stubCategoryTreeFallback) SelectCategoryID(query string, tree *sheincategory.CategoryTreeResponse) (int, error) {
	return s.selectedID, s.err
}

type stubCategorySuggestFallback struct {
	selectedID int
	err        error
}

func (s stubCategorySuggestFallback) SelectCategoryID(input sheincategoryselector.CoreItemInput, api CategoryAPI) (int, error) {
	return s.selectedID, s.err
}

type categorySuggestFallbackFunc func(input sheincategoryselector.CoreItemInput, api CategoryAPI) (int, error)

func (f categorySuggestFallbackFunc) SelectCategoryID(input sheincategoryselector.CoreItemInput, api CategoryAPI) (int, error) {
	return f(input, api)
}

func TestCategoryResolverFallsBackToCategoryTreeSelection(t *testing.T) {
	resolver := NewCategoryResolverWithTreeFallback(stubCategoryAPI{
		suggestResponse: &sheincategory.SuggestCategoryResponse{},
		categoryTree:    &sheincategory.CategoryTreeResponse{},
		categoryInfo: &sheincategory.CategoryInfo{
			CategoryID:             8824,
			LevelOneCategoryID:     10,
			LevelOneCategoryName:   "Shoes",
			LevelTwoCategoryID:     20,
			LevelTwoCategoryName:   "Women Shoes",
			LevelThreeCategoryID:   8824,
			LevelThreeCategoryName: "Sneakers",
			ProductTypeID:          9988,
		},
	}, stubCategoryTreeFallback{selectedID: 8824})

	resolution := resolver.Resolve(&BuildRequest{Text: "running shoes"}, &productenrich.CanonicalProduct{}, &Package{})
	if resolution.Status != "resolved" {
		t.Fatalf("status = %q, want resolved", resolution.Status)
	}
	if resolution.Source != "ai_category_tree" {
		t.Fatalf("source = %q, want ai_category_tree", resolution.Source)
	}
	if resolution.CategoryID != 8824 {
		t.Fatalf("category_id = %d, want 8824", resolution.CategoryID)
	}
}

func TestCategoryResolverReturnsPartialWhenCategoryTreeLoadFails(t *testing.T) {
	resolver := NewCategoryResolverWithTreeFallback(stubCategoryAPI{
		suggestResponse: &sheincategory.SuggestCategoryResponse{},
		categoryTreeErr: errors.New("tree temporarily unavailable"),
	}, stubCategoryTreeFallback{})

	resolution := resolver.Resolve(&BuildRequest{Text: "running shoes"}, &productenrich.CanonicalProduct{}, &Package{})
	if resolution.Status != "partial" {
		t.Fatalf("status = %q, want partial", resolution.Status)
	}
	if len(resolution.ReviewNotes) == 0 {
		t.Fatal("expected review notes")
	}
	if resolution.ReviewNotes[len(resolution.ReviewNotes)-1] != "SHEIN 类目树加载失败: tree temporarily unavailable" {
		t.Fatalf("unexpected review note: %q", resolution.ReviewNotes[len(resolution.ReviewNotes)-1])
	}
}

func TestCategoryResolverSuggestsAlternativeCategoryFromTreeFallback(t *testing.T) {
	resolver := NewCategoryResolverWithTreeFallback(stubCategoryAPI{
		categoryTree: &sheincategory.CategoryTreeResponse{},
		categoryInfo: &sheincategory.CategoryInfo{
			CategoryID:             8824,
			LevelOneCategoryID:     10,
			LevelOneCategoryName:   "Shoes",
			LevelTwoCategoryID:     20,
			LevelTwoCategoryName:   "Women Shoes",
			LevelThreeCategoryID:   8824,
			LevelThreeCategoryName: "Sneakers",
			ProductTypeID:          9988,
		},
	}, stubCategoryTreeFallback{selectedID: 8824})

	recommender := resolver.(categoryRecommender)
	suggestion := recommender.SuggestAlternative(&BuildRequest{Text: "running shoes"}, &productenrich.CanonicalProduct{
		Title:        "women running shoes",
		CategoryPath: []string{"Shoes", "Women Shoes"},
	}, &Package{
		CategoryID:   12143,
		CategoryPath: []string{"Shoes", "Women Shoes"},
	})
	if suggestion == nil {
		t.Fatal("expected category suggestion")
	}
	if suggestion.CategoryID != 8824 {
		t.Fatalf("suggested category_id = %d, want 8824", suggestion.CategoryID)
	}
	if suggestion.Source != "ai_category_tree" {
		t.Fatalf("source = %q, want ai_category_tree", suggestion.Source)
	}
}

func TestCategoryResolverSuggestsAlternativeCategoryFromSuggestFallback(t *testing.T) {
	resolver := NewCategoryResolverWithFallbacks(stubCategoryAPI{
		categoryInfo: &sheincategory.CategoryInfo{
			CategoryID:             6001,
			LevelOneCategoryID:     900,
			LevelOneCategoryName:   "Kitchen & Dining",
			LevelTwoCategoryID:     901,
			LevelTwoCategoryName:   "Drinkware",
			LevelThreeCategoryID:   6001,
			LevelThreeCategoryName: "Tumblers",
			ProductTypeID:          2001,
		},
	}, stubCategorySuggestFallback{selectedID: 6001}, nil)
	recommender := resolver.(categoryRecommender)
	suggestion := recommender.SuggestAlternative(&BuildRequest{
		Text:               "420ml stainless steel tumbler",
		TargetCategoryHint: "12143",
	}, &productenrich.CanonicalProduct{
		Title:        "420ml 不锈钢保温杯",
		CategoryPath: []string{"Kitchen", "Drinkware"},
		Attributes: map[string]productenrich.CanonicalAttribute{
			"材质": {Value: "不锈钢"},
			"容量": {Value: "420ml"},
		},
	}, &Package{
		CategoryID:   12143,
		CategoryPath: []string{"Kitchen", "Drinkware"},
	})
	if suggestion == nil {
		t.Fatal("expected suggest fallback suggestion")
	}
	if suggestion.Source != "suggest_category_by_text" {
		t.Fatalf("source = %q, want suggest_category_by_text", suggestion.Source)
	}
}

func TestCategoryResolverFallsBackToTreeWhenSuggestFallbackCandidateRejected(t *testing.T) {
	resolver := NewCategoryResolverWithFallbacks(stubCategoryAPI{
		categoryInfoByID: map[int]*sheincategory.CategoryInfo{
			9343: {
				CategoryID:             9343,
				LevelOneCategoryID:     4478,
				LevelOneCategoryName:   "女士服装",
				LevelTwoCategoryID:     9340,
				LevelTwoCategoryName:   "女士制服&特殊服饰",
				LevelThreeCategoryID:   9345,
				LevelThreeCategoryName: "女士装扮服饰&角色扮演服饰",
				LevelFourCategoryID:    intPointer(9343),
				LevelFourCategoryName:  stringPointer("洛丽塔服饰"),
				ProductTypeID:          5991,
			},
			6001: {
				CategoryID:             6001,
				LevelOneCategoryID:     900,
				LevelOneCategoryName:   "Kitchen & Dining",
				LevelTwoCategoryID:     901,
				LevelTwoCategoryName:   "Drinkware",
				LevelThreeCategoryID:   6001,
				LevelThreeCategoryName: "Tumblers",
				ProductTypeID:          2001,
			},
		},
		categoryTree: &sheincategory.CategoryTreeResponse{},
	}, stubCategorySuggestFallback{selectedID: 9343}, stubCategoryTreeFallback{selectedID: 6001})

	recommender := resolver.(categoryRecommender)
	suggestion := recommender.SuggestAlternative(&BuildRequest{Text: "420ml stainless steel tumbler"}, &productenrich.CanonicalProduct{
		Title:        "420ml 不锈钢保温杯",
		CategoryPath: []string{"Kitchen", "Drinkware"},
		Attributes: map[string]productenrich.CanonicalAttribute{
			"材质": {Value: "不锈钢"},
			"容量": {Value: "420ml"},
		},
	}, &Package{
		CategoryID: 12143,
	})
	if suggestion == nil {
		t.Fatal("expected tree fallback suggestion after suggest candidate rejected")
	}
	if suggestion.Source != "ai_category_tree" {
		t.Fatalf("source = %q, want ai_category_tree", suggestion.Source)
	}
}

func TestCategoryResolverRejectsCrossFamilySuggestedCategory(t *testing.T) {
	resolver := NewCategoryResolverWithTreeFallback(stubCategoryAPI{
		categoryTree: &sheincategory.CategoryTreeResponse{},
		categoryInfo: &sheincategory.CategoryInfo{
			CategoryID:             9343,
			LevelOneCategoryID:     4478,
			LevelOneCategoryName:   "女士服装",
			LevelTwoCategoryID:     9340,
			LevelTwoCategoryName:   "女士制服&特殊服饰",
			LevelThreeCategoryID:   9345,
			LevelThreeCategoryName: "女士装扮服饰&角色扮演服饰",
			LevelFourCategoryID:    intPointer(9343),
			LevelFourCategoryName:  stringPointer("洛丽塔服饰"),
			ProductTypeID:          5991,
		},
	}, stubCategoryTreeFallback{selectedID: 9343})

	recommender := resolver.(categoryRecommender)
	suggestion := recommender.SuggestAlternative(&BuildRequest{Text: "camping moon chair"}, &productenrich.CanonicalProduct{
		CategoryPath: []string{"Outdoor Recreation", "Camping Gear", "Camping Furniture"},
	}, &Package{
		CategoryID:   12143,
		CategoryPath: []string{"Outdoor Recreation", "Camping Gear", "Camping Furniture"},
	})
	if suggestion != nil {
		t.Fatalf("expected nil suggestion for cross-family candidate, got %+v", suggestion)
	}
}

func TestCategoryResolverRejectsCrossFamilySuggestedCategoryWithoutCurrentPath(t *testing.T) {
	resolver := NewCategoryResolverWithTreeFallback(stubCategoryAPI{
		categoryTree: &sheincategory.CategoryTreeResponse{},
		categoryInfo: &sheincategory.CategoryInfo{
			CategoryID:             9343,
			LevelOneCategoryID:     4478,
			LevelOneCategoryName:   "女士服装",
			LevelTwoCategoryID:     9340,
			LevelTwoCategoryName:   "女士制服&特殊服饰",
			LevelThreeCategoryID:   9345,
			LevelThreeCategoryName: "女士装扮服饰&角色扮演服饰",
			LevelFourCategoryID:    intPointer(9343),
			LevelFourCategoryName:  stringPointer("洛丽塔服饰"),
			ProductTypeID:          5991,
		},
	}, stubCategoryTreeFallback{selectedID: 9343})

	recommender := resolver.(categoryRecommender)
	suggestion := recommender.SuggestAlternative(&BuildRequest{Text: "ultralight camping moon chair"}, &productenrich.CanonicalProduct{
		Title:       "户外露营月亮椅 折叠桌椅套装",
		Attributes:  map[string]productenrich.CanonicalAttribute{"用途": {Value: "露营"}, "材质": {Value: "铝合金"}},
		Description: "适合露营和户外折叠桌椅场景",
	}, &Package{
		CategoryID: 12143,
	})
	if suggestion != nil {
		t.Fatalf("expected nil suggestion for cross-family candidate without current path, got %+v", suggestion)
	}
}

func TestCategoryResolverAcceptsSuggestionWhenStructuredSignalsAlign(t *testing.T) {
	resolver := NewCategoryResolverWithTreeFallback(stubCategoryAPI{
		categoryTree: &sheincategory.CategoryTreeResponse{},
		categoryInfo: &sheincategory.CategoryInfo{
			CategoryID:             6001,
			LevelOneCategoryID:     900,
			LevelOneCategoryName:   "Kitchen & Dining",
			LevelTwoCategoryID:     901,
			LevelTwoCategoryName:   "Drinkware",
			LevelThreeCategoryID:   6001,
			LevelThreeCategoryName: "Tumblers",
			ProductTypeID:          2001,
		},
	}, stubCategoryTreeFallback{selectedID: 6001})

	recommender := resolver.(categoryRecommender)
	suggestion := recommender.SuggestAlternative(&BuildRequest{Text: "420ml stainless steel tumbler"}, &productenrich.CanonicalProduct{
		Title:        "420ml 不锈钢保温杯",
		CategoryPath: []string{"Kitchen", "Drinkware"},
		Attributes: map[string]productenrich.CanonicalAttribute{
			"材质": {Value: "不锈钢"},
			"容量": {Value: "420ml"},
		},
		VariantDimensions: []productenrich.ScrapedVariantDimension{
			{Name: "颜色", Values: []string{"裸粉", "抹茶绿", "黑色"}},
		},
	}, &Package{
		CategoryID: 12143,
	})
	if suggestion == nil {
		t.Fatal("expected suggestion when structured product signals align")
	}
	if suggestion.CategoryID != 6001 {
		t.Fatalf("suggested category_id = %d, want 6001", suggestion.CategoryID)
	}
}

func TestShouldAcceptSuggestedCategoryRequiresEnoughStructuredFit(t *testing.T) {
	canonical := &productenrich.CanonicalProduct{
		Title:        "420ml stainless steel tumbler",
		CategoryPath: []string{"Kitchen", "Drinkware"},
		Attributes: map[string]productenrich.CanonicalAttribute{
			"材质": {Value: "不锈钢"},
			"容量": {Value: "420ml"},
		},
	}

	if !shouldAcceptSuggestedCategory(canonical, &Package{CategoryPath: []string{"Kitchen", "Drinkware"}}, &CategorySuggestion{
		CategoryID:     6001,
		MatchedPath:    []string{"Kitchen & Dining", "Drinkware", "Tumblers"},
		CategoryIDList: []int{900, 901, 6001},
	}) {
		t.Fatal("expected drinkware-aligned suggestion to pass fit threshold")
	}

	if shouldAcceptSuggestedCategory(canonical, &Package{CategoryPath: []string{"Kitchen", "Drinkware"}}, &CategorySuggestion{
		CategoryID:     9343,
		MatchedPath:    []string{"女士服装", "女士制服&特殊服饰", "女士装扮服饰&角色扮演服饰", "洛丽塔服饰"},
		CategoryIDList: []int{4478, 9340, 9345, 9343},
	}) {
		t.Fatal("expected cross-family suggestion to fail fit threshold")
	}
}

func TestBuildCategoryQueryIncludesStructuredOutdoorCushionSignals(t *testing.T) {
	query := buildCategoryQuery(&BuildRequest{}, &productenrich.CanonicalProduct{
		Title:        "New Women's Summer Thin Ice Silk Pajamas",
		Description:  "Outdoor garden bench cushion for hanging chair and balcony seating",
		CategoryPath: []string{"Home", "Outdoor"},
		Attributes: map[string]productenrich.CanonicalAttribute{
			"产品类别": {Value: "椅垫"},
			"空间":   {Value: "室外,阳台"},
			"材质":   {Value: "涤纶"},
		},
		VariantDimensions: []productenrich.ScrapedVariantDimension{
			{Name: "颜色", Values: []string{"深蓝", "米黄"}},
		},
	}, &Package{
		Attributes: map[string]string{
			"产品类别": "椅垫",
			"空间":   "室外,阳台",
		},
	})

	for _, expected := range []string{"椅垫", "室外,阳台", "New Women's Summer Thin Ice Silk Pajamas"} {
		if !strings.Contains(query, expected) {
			t.Fatalf("query = %q, want to contain %q", query, expected)
		}
	}
	if strings.Contains(query, "Outdoor garden bench cushion") {
		t.Fatalf("query should not include weak description when strong signals already exist: %q", query)
	}
}

func TestBuildCategoryQuerySkipsWeakDescriptionWhenStrongSignalsExist(t *testing.T) {
	query := buildCategoryQuery(&BuildRequest{Text: "iCOSS Smart Toilet Seat Bidet Attachment"}, &productenrich.CanonicalProduct{
		Title:       "Outdoor Bench Cushion",
		Description: "iCOSS Smart Toilet Seat Bidet Attachment",
		Attributes: map[string]productenrich.CanonicalAttribute{
			"产品类别": {Value: "椅垫"},
			"空间":   {Value: "室外,阳台"},
			"材质":   {Value: "涤纶"},
		},
		VariantDimensions: []productenrich.ScrapedVariantDimension{
			{Name: "尺寸", Values: []string{"150*100*10CM"}},
		},
	}, &Package{
		CategoryName: "Product",
		CategoryPath: []string{"General", "Product"},
		Attributes: map[string]string{
			"产品类别": "椅垫",
		},
	})

	if strings.Contains(query, "Smart Toilet Seat Bidet Attachment") {
		t.Fatalf("query should skip weak noisy text when strong signals exist: %q", query)
	}
	if strings.Contains(query, "General > Product") {
		t.Fatalf("query should skip generic category placeholders: %q", query)
	}
}

func TestBuildCategorySuggestionQueryUsesWeakTextWhenSignalsAreSparse(t *testing.T) {
	query := buildCategorySuggestionQuery(&BuildRequest{Text: "stainless steel tumbler 420ml"}, &productenrich.CanonicalProduct{
		Title: "Travel Cup",
	}, &Package{})

	if query != "Travel Cup" {
		t.Fatalf("query should prefer concise title seed, got %q", query)
	}
}

func TestBuildCategorySuggestionQueryFallsBackToCompactAttributes(t *testing.T) {
	query := buildCategorySuggestionQuery(&BuildRequest{}, &productenrich.CanonicalProduct{
		Attributes: map[string]productenrich.CanonicalAttribute{
			"产品类别": {Value: "椅垫"},
			"材质":   {Value: "涤纶"},
			"用途":   {Value: "户外"},
		},
	}, &Package{})

	if query != "椅垫 涤纶 户外" {
		t.Fatalf("query should fall back to compact attribute seed, got %q", query)
	}
}

func TestBuildCategorySuggestionQueryPrefersSourceCategoryLeafWhenTitleMissing(t *testing.T) {
	query := buildCategorySuggestionQuery(&BuildRequest{Text: "some noisy request"}, &productenrich.CanonicalProduct{
		CategoryPath: []string{"家居饰品", "户外用品", "户外坐垫"},
	}, &Package{})

	if query != "户外用品 户外坐垫" {
		t.Fatalf("query should prefer compact source category seed, got %q", query)
	}
}

func TestBuildCategorySuggestInputIncludesStructuredSignals(t *testing.T) {
	input := buildCategorySuggestInput(&BuildRequest{Text: "noisy request"}, &productenrich.CanonicalProduct{
		Title:        "跨境亚马逊户外防水防晒长凳垫吊椅垫",
		CategoryPath: []string{"家居饰品", "户外用品", "户外坐垫"},
		Attributes: map[string]productenrich.CanonicalAttribute{
			"产品类别": {Value: "椅垫"},
			"空间":   {Value: "室外,阳台"},
			"材质":   {Value: "涤纶"},
		},
	}, &Package{})

	if input.Title != "跨境亚马逊户外防水防晒长凳垫吊椅垫" {
		t.Fatalf("title = %q", input.Title)
	}
	if input.ProductType != "椅垫" {
		t.Fatalf("product type = %q, want 椅垫", input.ProductType)
	}
	if strings.Join(input.CategoryPath, " > ") != "家居饰品 > 户外用品 > 户外坐垫" {
		t.Fatalf("category path = %v", input.CategoryPath)
	}
	if input.Attributes["空间"] != "室外,阳台" {
		t.Fatalf("attributes = %#v, want 空间", input.Attributes)
	}
}

func TestBuildCategoryFamilyConflictSummaryDetectsOutdoorCushionVsApparel(t *testing.T) {
	canonical := &productenrich.CanonicalProduct{
		Title:        "Outdoor bench cushion for hanging chair",
		Description:  "Garden seat cushion for balcony and patio furniture",
		CategoryPath: []string{"Outdoor", "Furniture", "Cushions"},
		Attributes: map[string]productenrich.CanonicalAttribute{
			"产品类别": {Value: "椅垫"},
			"空间":   {Value: "室外,阳台"},
			"材质":   {Value: "涤纶"},
		},
	}
	pkg := &Package{
		CategoryPath: []string{"女士服装", "女士制服&特殊服饰", "女士装扮服饰&角色扮演服饰", "角色扮演服饰"},
		Attributes: map[string]string{
			"产品类别": "椅垫",
			"空间":   "室外,阳台",
			"材质":   "涤纶",
		},
	}

	recommend, reason := buildCategoryFamilyConflictSummary(canonical, pkg)
	if !recommend {
		t.Fatal("expected outdoor cushion vs apparel conflict to require review")
	}
	if reason == "" {
		t.Fatal("expected non-empty category review reason")
	}
}

func intPointer(v int) *int { return &v }

func stringPointer(v string) *string { return &v }
