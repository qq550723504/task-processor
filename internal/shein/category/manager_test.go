package category

import (
	"context"
	"strings"
	"testing"

	"task-processor/internal/shein/aicache"
	sheinapicategory "task-processor/internal/shein/api/category"
)

type stubAISelector struct {
	coreItemInput CoreItemInput
	coreItem      string
	coreItemCalls int
	levelOneID    int
	categoryID    int
	leafIDs       []int
	leafMap       map[int]string
}

func (s *stubAISelector) SelectLevelOneCategoryByAI(context.Context, string, []int, map[int]string) (int, error) {
	return s.levelOneID, nil
}

func (s *stubAISelector) SelectCategoryByAI(_ context.Context, _ string, leafIDs []int, leafMap map[int]string) (int, error) {
	s.leafIDs = append([]int(nil), leafIDs...)
	s.leafMap = make(map[int]string, len(leafMap))
	for key, value := range leafMap {
		s.leafMap[key] = value
	}
	return s.categoryID, nil
}

func (s *stubAISelector) ExtractCoreItemByAI(_ context.Context, input CoreItemInput) (string, error) {
	s.coreItemInput = input
	s.coreItemCalls++
	return s.coreItem, nil
}

type stubSuggestAPI struct {
	productInfo string
	resp        *sheinapicategory.SuggestCategoryResponse
}

func (s *stubSuggestAPI) SuggestCategoryByText(productInfo string) (*sheinapicategory.SuggestCategoryResponse, error) {
	s.productInfo = productInfo
	return s.resp, nil
}

func TestBuildCoreItemPromptInputIncludesStructuredFields(t *testing.T) {
	got := buildCoreItemPromptInput(CoreItemInput{
		Title:        "跨境亚马逊户外防水防晒长凳垫吊椅垫",
		ProductType:  "椅垫",
		CategoryPath: []string{"家居饰品", "户外用品", "户外坐垫"},
		Attributes: map[string]string{
			"空间": "室外,阳台",
			"材质": "涤纶",
		},
	})

	for _, want := range []string{"商品标题:", "产品类别: 椅垫", "来源类目: 家居饰品 > 户外用品 > 户外坐垫", "空间: 室外,阳台"} {
		if !strings.Contains(got, want) {
			t.Fatalf("prompt input = %q, want contains %q", got, want)
		}
	}
}

func TestGetCategoryIDBySuggestUsesCoreItemInput(t *testing.T) {
	selector := &stubAISelector{coreItem: "户外坐垫"}
	api := &stubSuggestAPI{resp: &sheinapicategory.SuggestCategoryResponse{
		Data: []sheinapicategory.SuggestCategoryItem{{CategoryID: "8604"}},
	}}
	manager := NewCategoryManager(selector)
	categoryAPI := stubCategoryTreeAPI{
		suggestAPI: api,
		categoryInfoByID: map[int]*sheinapicategory.CategoryInfo{
			8604: {
				CategoryID:             8604,
				LevelOneCategoryID:     1,
				LevelOneCategoryName:   "Home",
				LevelTwoCategoryID:     2,
				LevelTwoCategoryName:   "Outdoor",
				LevelThreeCategoryID:   8604,
				LevelThreeCategoryName: "Cushions",
			},
		},
	}

	categoryID, err := manager.GetCategoryIDBySuggest(context.Background(), CoreItemInput{
		Title:        "跨境亚马逊户外防水防晒长凳垫吊椅垫",
		ProductType:  "椅垫",
		CategoryPath: []string{"家居饰品", "户外用品", "户外坐垫"},
	}, categoryAPI, nil)
	if err != nil {
		t.Fatalf("GetCategoryIDBySuggest error = %v", err)
	}
	if categoryID != 8604 {
		t.Fatalf("categoryID = %d, want 8604", categoryID)
	}
	if selector.coreItemInput.ProductType != "椅垫" {
		t.Fatalf("core item input = %+v", selector.coreItemInput)
	}
	if api.productInfo != "户外坐垫" {
		t.Fatalf("suggest productInfo = %q, want 户外坐垫", api.productInfo)
	}
}

func TestGetCategoryIDBySuggestSkipsChildrenCandidates(t *testing.T) {
	selector := &stubAISelector{coreItem: "casual backpack"}
	api := &stubSuggestAPI{resp: &sheinapicategory.SuggestCategoryResponse{
		Data: []sheinapicategory.SuggestCategoryItem{
			{CategoryID: "1001"},
			{CategoryID: "1002"},
		},
	}}
	manager := NewCategoryManager(selector)

	categoryAPI := stubCategoryTreeAPI{
		suggestAPI: api,
		categoryInfoByID: map[int]*sheinapicategory.CategoryInfo{
			1001: {
				CategoryID:             1001,
				LevelOneCategoryID:     1,
				LevelOneCategoryName:   "Kids",
				LevelTwoCategoryID:     11,
				LevelTwoCategoryName:   "Bags",
				LevelThreeCategoryID:   111,
				LevelThreeCategoryName: "School Bags",
			},
			1002: {
				CategoryID:             1002,
				LevelOneCategoryID:     2,
				LevelOneCategoryName:   "Women",
				LevelTwoCategoryID:     22,
				LevelTwoCategoryName:   "Accessories",
				LevelThreeCategoryID:   222,
				LevelThreeCategoryName: "Backpacks",
			},
		},
	}

	categoryID, err := manager.GetCategoryIDBySuggest(context.Background(), CoreItemInput{
		Title: "casual backpack for travel",
	}, categoryAPI, nil)
	if err != nil {
		t.Fatalf("GetCategoryIDBySuggest error = %v", err)
	}
	if categoryID != 1002 {
		t.Fatalf("categoryID = %d, want 1002", categoryID)
	}
}

func TestGetCategoryIDBySuggestUsesTitleLevelCategoryCacheBeforeCoreItemAI(t *testing.T) {
	selector := &stubAISelector{coreItem: "walking shoes"}
	manager := NewCategoryManager(selector)
	cache := aicache.New(nil)
	title := "Skechers Women's Go Walk 5 Walking Shoes"
	cache.Set(aicache.TypeCategory, aicache.HashKey(title), 4455)
	categoryAPI := stubCategoryTreeAPI{
		suggestAPI: &stubSuggestAPI{resp: &sheinapicategory.SuggestCategoryResponse{
			Data: []sheinapicategory.SuggestCategoryItem{{CategoryID: "9999"}},
		}},
	}

	categoryID, err := manager.GetCategoryIDBySuggest(context.Background(), CoreItemInput{
		Title: title,
	}, categoryAPI, cache)
	if err != nil {
		t.Fatalf("GetCategoryIDBySuggest error = %v", err)
	}
	if categoryID != 4455 {
		t.Fatalf("categoryID = %d, want 4455", categoryID)
	}
	if selector.coreItemCalls != 0 {
		t.Fatalf("ExtractCoreItemByAI call count = %d, want 0 when title cache hits", selector.coreItemCalls)
	}
}

func TestGetCategoryIDByTitleWithTreeFiltersChildrenLeafCandidates(t *testing.T) {
	selector := &stubAISelector{
		levelOneID: 10,
		categoryID: 2002,
	}
	manager := NewCategoryManager(selector)

	tree := &sheinapicategory.CategoryTreeResponse{
		Data: []sheinapicategory.CategoryTreeNode{{
			CategoryID:   10,
			CategoryName: "Bags",
			Children: []sheinapicategory.CategoryTreeNode{
				{
					CategoryID:   2001,
					CategoryName: "Kids Backpacks",
					LastCategory: true,
				},
				{
					CategoryID:   2002,
					CategoryName: "Travel Backpacks",
					LastCategory: true,
				},
			},
		}},
	}

	categoryID, err := manager.GetCategoryIDByTitleWithTree(context.Background(), "travel backpack", tree, nil)
	if err != nil {
		t.Fatalf("GetCategoryIDByTitleWithTree error = %v", err)
	}
	if categoryID != 2002 {
		t.Fatalf("categoryID = %d, want 2002", categoryID)
	}
	if len(selector.leafIDs) != 1 || selector.leafIDs[0] != 2002 {
		t.Fatalf("leafIDs = %#v, want [2002]", selector.leafIDs)
	}
	if got := selector.leafMap[2002]; got != "Travel Backpacks" {
		t.Fatalf("leaf path = %q, want Travel Backpacks", got)
	}
}

type stubCategoryTreeAPI struct {
	suggestAPI        *stubSuggestAPI
	categoryInfoByID  map[int]*sheinapicategory.CategoryInfo
	categoryLookupErr error
}

func (s stubCategoryTreeAPI) SuggestCategoryByText(productInfo string) (*sheinapicategory.SuggestCategoryResponse, error) {
	return s.suggestAPI.SuggestCategoryByText(productInfo)
}

func (s stubCategoryTreeAPI) GetCategory(categoryID int) (*sheinapicategory.CategoryInfo, error) {
	if s.categoryLookupErr != nil {
		return nil, s.categoryLookupErr
	}
	return s.categoryInfoByID[categoryID], nil
}
