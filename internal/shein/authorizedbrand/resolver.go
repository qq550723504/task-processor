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

	code := trimMatchKey(cfg.Code)
	name := trimMatchKey(cfg.Name)
	if code == "" && name == "" {
		return nil, sherr.NewNonRetryableError("authorized brand config is empty", nil)
	}

	resp, err := r.productAPI.QueryBrandList()
	if err != nil {
		return nil, sherr.NewRetryableError("query authorized brand list failed", err)
	}
	if resp == nil {
		return nil, sherr.NewNonRetryableError("SHEIN brand list response is empty", nil)
	}

	for _, item := range resp.Info.Data {
		if code != "" && trimMatchKey(item.BrandCode) == code {
			return newResolved(item), nil
		}
	}

	if name != "" {
		for _, item := range resp.Info.Data {
			if trimMatchKey(item.BrandName) == name || trimMatchKey(item.BrandNameEn) == name {
				return newResolved(item), nil
			}
		}
	}

	return nil, sherr.NewNonRetryableError("configured authorized brand was not found in SHEIN brand list", nil)
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
