package authorizedbrand

import (
	"context"
	"errors"
	"testing"

	sheinproduct "task-processor/internal/shein/api/product"
	"task-processor/internal/shein/sherr"
)

type stubProductAPI struct {
	brandResp  *sheinproduct.BrandListResponse
	brandErr   error
	brandCalls int
}

func (s *stubProductAPI) QueryBrandList() (*sheinproduct.BrandListResponse, error) {
	s.brandCalls++
	return s.brandResp, s.brandErr
}

func TestResolveAuthorizedBrand_DisabledReturnsNil(t *testing.T) {
	resolver := NewResolver(&stubProductAPI{})

	got, err := resolver.Resolve(context.Background(), Config{})
	if err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}
	if got != nil {
		t.Fatalf("Resolve() = %+v, want nil", got)
	}
}

func TestResolveAuthorizedBrand_FailsWhenConfigEmpty(t *testing.T) {
	resolver := NewResolver(&stubProductAPI{})

	_, err := resolver.Resolve(context.Background(), Config{Enabled: true})
	if err == nil {
		t.Fatal("Resolve() error = nil, want error")
	}
	if sherr.IsRetryableError(err) {
		t.Fatalf("Resolve() error retryable = true, want false: %v", err)
	}
}

func TestResolveAuthorizedBrand_PrefersCodeMatch(t *testing.T) {
	resolver := NewResolver(&stubProductAPI{
		brandResp: brandListResponse(
			sheinproduct.BrandItem{BrandCode: "other", BrandName: "Other", BrandNameEn: "Other"},
			sheinproduct.BrandItem{BrandCode: " 2fd1n ", BrandName: "Logitech罗技", BrandNameEn: "Logitech"},
		),
	})

	got, err := resolver.Resolve(context.Background(), Config{Enabled: true, Code: " 2fd1n ", Name: "Logitech"})
	if err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}
	if got == nil {
		t.Fatal("Resolve() = nil, want resolved brand")
	}
	if got.Code != "2fd1n" || got.Name != "Logitech罗技" || got.NameEn != "Logitech" || !got.Enabled {
		t.Fatalf("Resolve() = %+v", got)
	}
}

func TestResolveAuthorizedBrand_MatchesExactName(t *testing.T) {
	resolver := NewResolver(&stubProductAPI{
		brandResp: brandListResponse(
			sheinproduct.BrandItem{BrandCode: "2fd1n", BrandName: " Logitech ", BrandNameEn: "Else"},
		),
	})

	got, err := resolver.Resolve(context.Background(), Config{Enabled: true, Name: " Logitech "})
	if err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}
	if got == nil || got.Code != "2fd1n" {
		t.Fatalf("Resolve() = %+v, want brand code 2fd1n", got)
	}
}

func TestResolveAuthorizedBrand_CaseMismatchDoesNotMatch(t *testing.T) {
	resolver := NewResolver(&stubProductAPI{
		brandResp: brandListResponse(
			sheinproduct.BrandItem{BrandCode: "2Fd1N", BrandName: "LoGiTech", BrandNameEn: "LoGiTech"},
		),
	})

	_, err := resolver.Resolve(context.Background(), Config{Enabled: true, Code: "2fd1n", Name: "Logitech"})
	if err == nil {
		t.Fatal("Resolve() error = nil, want error")
	}
	if sherr.IsRetryableError(err) {
		t.Fatalf("Resolve() error retryable = true, want false: %v", err)
	}
}

func TestResolveAuthorizedBrand_FailsWhenBrandListResponseNil(t *testing.T) {
	resolver := NewResolver(&stubProductAPI{})

	_, err := resolver.Resolve(context.Background(), Config{Enabled: true, Code: "2fd1n"})
	if err == nil {
		t.Fatal("Resolve() error = nil, want error")
	}
	if sherr.IsRetryableError(err) {
		t.Fatalf("Resolve() error retryable = true, want false: %v", err)
	}
}

func TestResolveAuthorizedBrand_FailsWhenConfiguredBrandMissing(t *testing.T) {
	resolver := NewResolver(&stubProductAPI{
		brandResp: brandListResponse(
			sheinproduct.BrandItem{BrandCode: "2fd1n", BrandName: "Logitech罗技", BrandNameEn: "Logitech"},
		),
	})

	_, err := resolver.Resolve(context.Background(), Config{Enabled: true, Code: "missing", Name: "Missing"})
	if err == nil {
		t.Fatal("Resolve() error = nil, want error")
	}
	if sherr.IsRetryableError(err) {
		t.Fatalf("Resolve() error retryable = true, want false: %v", err)
	}
}

func TestResolveAuthorizedBrand_ReturnsRetryableWhenBrandListQueryFails(t *testing.T) {
	resolver := NewResolver(&stubProductAPI{brandErr: errors.New("boom")})

	_, err := resolver.Resolve(context.Background(), Config{Enabled: true, Code: "2fd1n"})
	if err == nil {
		t.Fatal("Resolve() error = nil, want error")
	}
	if !sherr.IsRetryableError(err) {
		t.Fatalf("Resolve() error retryable = false, want true: %v", err)
	}
}

func TestResolveForProductBrand_PrefersProductBrandOverStoreFallback(t *testing.T) {
	resolver := NewResolver(&stubProductAPI{
		brandResp: brandListResponse(
			sheinproduct.BrandItem{BrandCode: "2wex9", BrandName: "Skechers", BrandNameEn: "Skechers"},
			sheinproduct.BrandItem{BrandCode: "nike-1", BrandName: "Nike", BrandNameEn: "Nike"},
		),
	})

	got, err := resolver.ResolveForProductBrand(context.Background(), Config{
		Enabled: true,
		Code:    "2wex9",
		Name:    "Skechers",
	}, "Nike")
	if err != nil {
		t.Fatalf("ResolveForProductBrand() error = %v", err)
	}
	if got == nil || got.Code != "nike-1" || got.NameEn != "Nike" {
		t.Fatalf("ResolveForProductBrand() = %+v, want Nike brand", got)
	}
}

func TestResolveForProductBrand_ReturnsNilWhenProductBrandEmpty(t *testing.T) {
	productAPI := &stubProductAPI{
		brandResp: brandListResponse(
			sheinproduct.BrandItem{BrandCode: "2wex9", BrandName: "Skechers", BrandNameEn: "Skechers"},
		),
	}
	resolver := NewResolver(productAPI)

	got, err := resolver.ResolveForProductBrand(context.Background(), Config{
		Enabled: true,
		Code:    "2wex9",
		Name:    "Skechers",
	}, "")
	if err != nil {
		t.Fatalf("ResolveForProductBrand() error = %v", err)
	}
	if got != nil {
		t.Fatalf("ResolveForProductBrand() = %+v, want nil", got)
	}
}

func TestResolveForProductBrand_ReturnsNilWhenProductBrandNotAuthorized(t *testing.T) {
	resolver := NewResolver(&stubProductAPI{
		brandResp: brandListResponse(
			sheinproduct.BrandItem{BrandCode: "2wex9", BrandName: "Skechers", BrandNameEn: "Skechers"},
		),
	})

	got, err := resolver.ResolveForProductBrand(context.Background(), Config{
		Enabled: true,
		Code:    "2wex9",
		Name:    "Skechers",
	}, "Nike")
	if err != nil {
		t.Fatalf("ResolveForProductBrand() error = %v, want nil", err)
	}
	if got != nil {
		t.Fatalf("ResolveForProductBrand() = %+v, want nil", got)
	}
}

func brandListResponse(items ...sheinproduct.BrandItem) *sheinproduct.BrandListResponse {
	resp := &sheinproduct.BrandListResponse{}
	resp.Info.Data = items
	resp.Info.Meta.Count = len(items)
	return resp
}
