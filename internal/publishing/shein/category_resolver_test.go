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
	categoryInfo    *sheincategory.CategoryInfo
	categoryErr     error
}

func (s stubCategoryAPI) GetCategory(categoryID int) (*sheincategory.CategoryInfo, error) {
	return s.categoryInfo, s.categoryErr
}

func (s stubCategoryAPI) GetCategoryTree() (*sheincategory.CategoryTreeResponse, error) {
	return nil, nil
}

func (s stubCategoryAPI) SuggestCategoryByText(productInfo string) (*sheincategory.SuggestCategoryResponse, error) {
	return s.suggestResponse, s.suggestErr
}

func TestCategoryResolverReturnsPartialWhenSuggestCategoryFails(t *testing.T) {
	resolver := NewCategoryResolver(stubCategoryAPI{
		suggestErr: &sheinapi.AuthenticationExpiredError{
			Code:    "20302",
			Message: "认证已过期，需要重新登录",
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
