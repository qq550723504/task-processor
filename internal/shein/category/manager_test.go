package category

import (
	"context"
	"strings"
	"testing"

	sheinapicategory "task-processor/internal/shein/api/category"
)

type stubAISelector struct {
	coreItemInput CoreItemInput
	coreItem      string
}

func (s *stubAISelector) SelectLevelOneCategoryByAI(context.Context, string, []int, map[int]string) (int, error) {
	return 0, nil
}

func (s *stubAISelector) SelectCategoryByAI(context.Context, string, []int, map[int]string) (int, error) {
	return 0, nil
}

func (s *stubAISelector) ExtractCoreItemByAI(_ context.Context, input CoreItemInput) (string, error) {
	s.coreItemInput = input
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

	categoryID, err := manager.GetCategoryIDBySuggest(context.Background(), CoreItemInput{
		Title:        "跨境亚马逊户外防水防晒长凳垫吊椅垫",
		ProductType:  "椅垫",
		CategoryPath: []string{"家居饰品", "户外用品", "户外坐垫"},
	}, api, nil)
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
