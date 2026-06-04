package sheinsync

import (
	"context"
	"fmt"
	"strconv"

	sheinproduct "task-processor/internal/shein/api/product"
)

type resolvedSheinCost struct {
	CostPrice *float64
	Currency  string
}

type SheinCostResolver interface {
	ResolveAutoCosts(ctx context.Context, product sheinproduct.ProductListItem) (map[string]resolvedSheinCost, error)
}

type sheinProductCostResolver struct {
	productAPI sheinproduct.ProductAPI
}

func NewSheinCostResolver(productAPI sheinproduct.ProductAPI) SheinCostResolver {
	return &sheinProductCostResolver{productAPI: productAPI}
}

func (r *sheinProductCostResolver) ResolveAutoCosts(_ context.Context, product sheinproduct.ProductListItem) (map[string]resolvedSheinCost, error) {
	if r == nil || r.productAPI == nil {
		return nil, fmt.Errorf("SHEIN product API is required for cost resolution")
	}

	skcNames := make([]string, 0, len(product.SkcInfoList))
	for _, skc := range product.SkcInfoList {
		if skc.SkcName == "" {
			continue
		}
		skcNames = append(skcNames, skc.SkcName)
	}
	if len(skcNames) == 0 {
		return map[string]resolvedSheinCost{}, nil
	}

	response, err := r.productAPI.QueryCostPrice(product.SpuName, skcNames)
	if err != nil {
		return nil, fmt.Errorf("query SHEIN cost price for spu %s: %w", product.SpuName, err)
	}
	if response == nil {
		return nil, fmt.Errorf("query SHEIN cost price for spu %s returned nil response", product.SpuName)
	}

	resolved := make(map[string]resolvedSheinCost, len(response.Info.Data))
	for _, item := range response.Info.Data {
		var (
			bestPrice float64
			hasPrice  bool
			currency  string
		)
		for _, skuCost := range item.SkuCostInfoList {
			parsedPrice, parseErr := strconv.ParseFloat(skuCost.CostPriceInfo.CostPrice, 64)
			if parseErr != nil || parsedPrice <= 0 {
				continue
			}
			if !hasPrice || parsedPrice > bestPrice {
				bestPrice = parsedPrice
				hasPrice = true
				currency = skuCost.CostPriceInfo.Currency
			}
		}
		if !hasPrice {
			continue
		}

		price := bestPrice
		resolved[item.SkcName] = resolvedSheinCost{
			CostPrice: &price,
			Currency:  currency,
		}
	}

	return resolved, nil
}
