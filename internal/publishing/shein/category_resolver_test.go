package shein

import (
	"errors"
	"testing"

	"task-processor/internal/productenrich"
	sheinapi "task-processor/internal/shein/api"
	sheincategory "task-processor/internal/shein/api/category"
)

type stubCategoryAPI struct {
	suggestResponse *sheincategory.SuggestCategoryResponse
	suggestErr      error
	categoryInfoByID map[int]*sheincategory.CategoryInfo
	categoryInfo    *sheincategory.CategoryInfo
	categoryErr     error
	categoryTree    *sheincategory.CategoryTreeResponse
	categoryTreeErr error
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
	resolver := NewCategoryResolver(stubCategoryAPI{
		suggestErr: &sheinapi.AuthenticationExpiredError{
			Code:     "20302",
			Message:  "认证已过期，需要重新登录",
			TenantID: 227,
			ShopID:   869,
		},
	})

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
	if len(resolution.ReviewNotes) != 1 {
		t.Fatalf("expected exactly one review note, got %d", len(resolution.ReviewNotes))
	}
	if resolution.ReviewNotes[0] != "SHEIN 类目在线解析失败: 认证过期 [20302]: 认证已过期，需要重新登录 (TenantID: 227, ShopID: 869)" {
		t.Fatalf("unexpected review note: %q", resolution.ReviewNotes[0])
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

func (s stubCategorySuggestFallback) SelectCategoryID(query string, api CategoryAPI) (int, error) {
	return s.selectedID, s.err
}

type categorySuggestFallbackFunc func(query string, api CategoryAPI) (int, error)

func (f categorySuggestFallbackFunc) SelectCategoryID(query string, api CategoryAPI) (int, error) {
	return f(query, api)
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
	var capturedQuery string
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
	}, categorySuggestFallbackFunc(func(query string, api CategoryAPI) (int, error) {
		capturedQuery = query
		return 6001, nil
	}), nil)

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
		CategoryID: 12143,
	})
	if suggestion == nil {
		t.Fatal("expected suggestion from suggest fallback")
	}
	if suggestion.CategoryID != 6001 {
		t.Fatalf("suggested category_id = %d, want 6001", suggestion.CategoryID)
	}
	if suggestion.Source != "suggest_category" {
		t.Fatalf("source = %q, want suggest_category", suggestion.Source)
	}
	if capturedQuery == "12143" {
		t.Fatal("expected suggestion query to ignore target_category_hint when richer product text exists")
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

func intPointer(v int) *int { return &v }

func stringPointer(v string) *string { return &v }
