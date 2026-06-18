package shein

import (
	"context"
	"errors"
	"strings"
	"testing"

	"task-processor/internal/catalog/canonical"
	sheinapi "task-processor/internal/shein/api"
	sheincategory "task-processor/internal/shein/api/category"
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

	resolution := resolver.Resolve(&BuildRequest{Text: "Sports Shoes"}, &canonical.Product{}, &Package{
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

func (s stubCategoryTreeFallback) SelectCategoryID(_ context.Context, query string, tree *sheincategory.CategoryTreeResponse) (int, error) {
	return s.selectedID, s.err
}

type stubCategorySuggestFallback struct {
	selectedID int
	err        error
}

func (s stubCategorySuggestFallback) SelectCategoryID(_ context.Context, input CategoryCoreItemInput, api CategoryAPI) (int, error) {
	return s.selectedID, s.err
}

type categorySuggestFallbackFunc func(ctx context.Context, input CategoryCoreItemInput, api CategoryAPI) (int, error)

func (f categorySuggestFallbackFunc) SelectCategoryID(ctx context.Context, input CategoryCoreItemInput, api CategoryAPI) (int, error) {
	return f(ctx, input, api)
}

type stubCategorySemanticVerifier struct {
	validation *CategorySemanticValidation
}

func (s stubCategorySemanticVerifier) ValidateProductCategory(_ context.Context, _ *canonical.Product, _ *Package, _ []string) *CategorySemanticValidation {
	return s.validation
}

func TestCategoryResolverAcceptsSuggestedCategoryWhenSemanticValidationIsUnavailable(t *testing.T) {
	resolver := NewCategoryResolverWithSemanticVerifier(stubCategoryAPI{
		categoryInfoByID: map[int]*sheincategory.CategoryInfo{
			2696: {
				CategoryID:             2696,
				LevelOneCategoryID:     1,
				LevelOneCategoryName:   "宠物用品",
				LevelTwoCategoryID:     2,
				LevelTwoCategoryName:   "宠物健康护理",
				LevelThreeCategoryID:   3,
				LevelThreeCategoryName: "宠物急救",
				LevelFourCategoryID:    intPointer(2696),
				LevelFourCategoryName:  stringPointer("宠物绷带"),
				ProductTypeID:          9001,
			},
		},
	}, stubCategorySuggestFallback{selectedID: 2696}, nil, stubCategorySemanticVerifier{})

	resolution := resolver.Resolve(&BuildRequest{Text: "格子宠物三角巾"}, &canonical.Product{
		Title:        "格子宠物三角巾",
		CategoryPath: []string{"美国本地直发", "宠物用品", "宠物方巾"},
	}, &Package{})

	if resolution.Status != "resolved" {
		t.Fatalf("status = %q, want resolved", resolution.Status)
	}
	if resolution.CategoryID != 2696 {
		t.Fatalf("category_id = %d, want 2696", resolution.CategoryID)
	}
	if resolution.SuggestedCategory != nil {
		t.Fatalf("suggested category = %+v, want nil after direct acceptance", resolution.SuggestedCategory)
	}
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

	resolution := resolver.Resolve(&BuildRequest{Text: "running shoes"}, &canonical.Product{}, &Package{})
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

func TestCategoryResolverRejectsChildrenCategoryForNonChildrenProduct(t *testing.T) {
	resolver := NewCategoryResolverWithSemanticVerifier(stubCategoryAPI{
		categoryInfoByID: map[int]*sheincategory.CategoryInfo{
			2696: {
				CategoryID:             2696,
				LevelOneCategoryID:     1,
				LevelOneCategoryName:   "Kids",
				LevelTwoCategoryID:     2,
				LevelTwoCategoryName:   "Bags",
				LevelThreeCategoryID:   2696,
				LevelThreeCategoryName: "School Backpacks",
				ProductTypeID:          9001,
			},
		},
	}, stubCategorySuggestFallback{selectedID: 2696}, nil, nil)

	resolution := resolver.Resolve(&BuildRequest{Text: "travel backpack"}, &canonical.Product{
		Title:        "Women's travel backpack with laptop compartment",
		CategoryPath: []string{"Bags", "Backpacks"},
	}, &Package{})

	if resolution.Status != "partial" {
		t.Fatalf("status = %q, want partial", resolution.Status)
	}
	if resolution.CategoryID != 0 {
		t.Fatalf("category_id = %d, want 0 after semantic rejection", resolution.CategoryID)
	}
	if resolution.SuggestedCategory == nil {
		t.Fatal("expected suggested category after semantic rejection")
	}
	if resolution.SemanticValidation == nil || resolution.SemanticValidation.Verdict != "incompatible" {
		t.Fatalf("semantic validation = %+v, want incompatible", resolution.SemanticValidation)
	}
	if len(resolution.ReviewNotes) == 0 {
		t.Fatal("expected review note")
	}
}

func TestCategoryResolverHydratesSelectedCategoryFromTreeWhenDetailFails(t *testing.T) {
	resolver := NewCategoryResolverWithTreeFallback(stubCategoryAPI{
		suggestResponse: &sheincategory.SuggestCategoryResponse{},
		categoryErr:     errors.New("detail unavailable"),
		categoryTree: &sheincategory.CategoryTreeResponse{Data: []sheincategory.CategoryTreeNode{{
			CategoryID:    10,
			CategoryName:  "Home",
			ProductTypeID: 1000,
			Children: []sheincategory.CategoryTreeNode{{
				CategoryID:    20,
				CategoryName:  "Decor",
				ProductTypeID: 2000,
				Children: []sheincategory.CategoryTreeNode{{
					CategoryID:    3105,
					CategoryName:  "Wall Clocks",
					ProductTypeID: 9988,
					LastCategory:  true,
				}},
			}},
		}}},
	}, stubCategoryTreeFallback{selectedID: 3105})

	resolution := resolver.Resolve(&BuildRequest{Text: "wall clock"}, &canonical.Product{}, &Package{})

	if resolution.Status != "resolved" {
		t.Fatalf("status = %q, want resolved", resolution.Status)
	}
	if resolution.CategoryID != 3105 {
		t.Fatalf("category_id = %d, want 3105", resolution.CategoryID)
	}
	if resolution.ProductTypeID != 9988 {
		t.Fatalf("product_type_id = %d, want 9988", resolution.ProductTypeID)
	}
	if got := strings.Join(resolution.MatchedPath, " > "); got != "Home > Decor > Wall Clocks" {
		t.Fatalf("matched path = %q", got)
	}
	if got := resolution.CategoryIDList; len(got) != 3 || got[0] != 10 || got[1] != 20 || got[2] != 3105 {
		t.Fatalf("category id list = %#v, want [10 20 3105]", got)
	}
}

func TestCategoryResolverReturnsPartialWhenCategoryTreeLoadFails(t *testing.T) {
	resolver := NewCategoryResolverWithTreeFallback(stubCategoryAPI{
		suggestResponse: &sheincategory.SuggestCategoryResponse{},
		categoryTreeErr: errors.New("tree temporarily unavailable"),
	}, stubCategoryTreeFallback{})

	resolution := resolver.Resolve(&BuildRequest{Text: "running shoes"}, &canonical.Product{}, &Package{})
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

func TestBuildCategoryQueryIncludesStructuredOutdoorCushionSignals(t *testing.T) {
	query := buildCategoryQuery(&BuildRequest{}, &canonical.Product{
		Title:        "New Women's Summer Thin Ice Silk Pajamas",
		Description:  "Outdoor garden bench cushion for hanging chair and balcony seating",
		CategoryPath: []string{"Home", "Outdoor"},
		Attributes: map[string]canonical.Attribute{
			"产品类别": {Value: "椅垫"},
			"空间":   {Value: "室外,阳台"},
			"材质":   {Value: "涤纶"},
		},
		VariantDimensions: []canonical.ScrapedVariantDimension{
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
	query := buildCategoryQuery(&BuildRequest{Text: "iCOSS Smart Toilet Seat Bidet Attachment"}, &canonical.Product{
		Title:       "Outdoor Bench Cushion",
		Description: "iCOSS Smart Toilet Seat Bidet Attachment",
		Attributes: map[string]canonical.Attribute{
			"产品类别": {Value: "椅垫"},
			"空间":   {Value: "室外,阳台"},
			"材质":   {Value: "涤纶"},
		},
		VariantDimensions: []canonical.ScrapedVariantDimension{
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
	query := buildCategorySuggestionQuery(&BuildRequest{Text: "stainless steel tumbler 420ml"}, &canonical.Product{
		Title: "Travel Cup",
	}, &Package{})

	if query != "Travel Cup" {
		t.Fatalf("query should prefer concise title seed, got %q", query)
	}
}

func TestBuildCategorySuggestionQueryFallsBackToCompactAttributes(t *testing.T) {
	query := buildCategorySuggestionQuery(&BuildRequest{}, &canonical.Product{
		Attributes: map[string]canonical.Attribute{
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
	query := buildCategorySuggestionQuery(&BuildRequest{Text: "some noisy request"}, &canonical.Product{
		CategoryPath: []string{"家居饰品", "户外用品", "户外坐垫"},
	}, &Package{})

	if query != "户外用品 户外坐垫" {
		t.Fatalf("query should prefer compact source category seed, got %q", query)
	}
}

func TestBuildCategorySuggestInputIncludesStructuredSignals(t *testing.T) {
	input := buildCategorySuggestInput(&BuildRequest{Text: "noisy request"}, &canonical.Product{
		Title:        "跨境亚马逊户外防水防晒长凳垫吊椅垫",
		CategoryPath: []string{"家居饰品", "户外用品", "户外坐垫"},
		Attributes: map[string]canonical.Attribute{
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
	canonical := &canonical.Product{
		Title:        "Outdoor bench cushion for hanging chair",
		Description:  "Garden seat cushion for balcony and patio furniture",
		CategoryPath: []string{"Outdoor", "Furniture", "Cushions"},
		Attributes: map[string]canonical.Attribute{
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
	if recommend {
		t.Fatalf("expected no rule-based category conflict review, got reason %q", reason)
	}
	if reason != "" {
		t.Fatalf("expected empty category review reason, got %q", reason)
	}
}

func intPointer(v int) *int { return &v }

func stringPointer(v string) *string { return &v }
