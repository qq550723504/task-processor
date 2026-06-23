package authorizedbrand

import (
	"context"
	"strings"

	sheinproduct "task-processor/internal/shein/api/product"
	"task-processor/internal/shein/sherr"
)

type ProductAPI interface {
	QueryBrandList() (*sheinproduct.BrandListResponse, error)
}

type Resolver struct {
	productAPI ProductAPI
}

func NewResolver(productAPI ProductAPI) *Resolver {
	return &Resolver{productAPI: productAPI}
}

func (r *Resolver) Resolve(_ context.Context, cfg Config) (*Resolved, error) {
	if !cfg.Enabled {
		return nil, nil
	}

	resp, err := r.queryBrandList()
	if err != nil {
		return nil, err
	}

	code := trimMatchKey(cfg.Code)
	name := trimMatchKey(cfg.Name)
	if code == "" && name == "" {
		return nil, sherr.NewNonRetryableError("authorized brand config is empty", nil)
	}

	return resolveFromItems(resp.Info.Data, code, name)
}

func resolveFromItems(items []sheinproduct.BrandItem, code, name string) (*Resolved, error) {
	if resolved := findByCode(items, code); resolved != nil {
		return resolved, nil
	}
	if resolved := findByName(items, name); resolved != nil {
		return resolved, nil
	}

	return nil, sherr.NewNonRetryableError("configured authorized brand was not found in SHEIN brand list", nil)
}

func (r *Resolver) ResolveForProductBrand(_ context.Context, cfg Config, productBrand string) (*Resolved, error) {
	if !cfg.Enabled {
		return nil, nil
	}

	productBrand = trimMatchKey(productBrand)
	if productBrand == "" {
		return nil, nil
	}

	resp, err := r.queryBrandList()
	if err != nil {
		return nil, err
	}

	if resolved := findByName(resp.Info.Data, productBrand); resolved != nil {
		return resolved, nil
	}
	return nil, nil
}

func (r *Resolver) queryBrandList() (*sheinproduct.BrandListResponse, error) {
	resp, err := r.productAPI.QueryBrandList()
	if err != nil {
		return nil, sherr.NewRetryableError("query authorized brand list failed", err)
	}
	if resp == nil {
		return nil, sherr.NewNonRetryableError("SHEIN brand list response is empty", nil)
	}
	return resp, nil
}

func findByCode(items []sheinproduct.BrandItem, code string) *Resolved {
	if code == "" {
		return nil
	}
	for _, item := range items {
		if trimMatchKey(item.BrandCode) == code {
			return newResolved(item)
		}
	}
	return nil
}

func findByName(items []sheinproduct.BrandItem, name string) *Resolved {
	if name == "" {
		return nil
	}
	for _, item := range items {
		if trimMatchKey(item.BrandName) == name || trimMatchKey(item.BrandNameEn) == name {
			return newResolved(item)
		}
	}
	return nil
}

func newResolved(item sheinproduct.BrandItem) *Resolved {
	return &Resolved{
		Enabled: true,
		Code:    strings.TrimSpace(item.BrandCode),
		Name:    strings.TrimSpace(item.BrandName),
		NameEn:  strings.TrimSpace(item.BrandNameEn),
	}
}

func trimMatchKey(value string) string {
	return strings.TrimSpace(value)
}
