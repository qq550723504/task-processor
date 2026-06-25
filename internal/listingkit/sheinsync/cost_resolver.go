package sheinsync

import (
	"context"
	"fmt"
	"strconv"
	"time"

	sheinproduct "task-processor/internal/shein/api/product"
)

type resolvedSheinCost struct {
	CostPrice *float64
	Currency  string
}

type SheinCostResolver interface {
	ResolveAutoCosts(ctx context.Context, product sheinproduct.ProductListItem) (map[string]resolvedSheinCost, error)
}

var defaultSheinCostPriceRetryDelays = []time.Duration{
	30 * time.Second,
	time.Minute,
	2 * time.Minute,
}

type sheinProductCostResolver struct {
	productAPI  sheinproduct.ProductAPI
	retryDelays []time.Duration
	sleep       func(context.Context, time.Duration) error
}

func NewSheinCostResolver(productAPI sheinproduct.ProductAPI) SheinCostResolver {
	return &sheinProductCostResolver{
		productAPI:  productAPI,
		retryDelays: append([]time.Duration(nil), defaultSheinCostPriceRetryDelays...),
		sleep:       sleepSheinCostPriceRetry,
	}
}

func (r *sheinProductCostResolver) ResolveAutoCosts(ctx context.Context, product sheinproduct.ProductListItem) (map[string]resolvedSheinCost, error) {
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

	response, err := r.queryCostPriceWithRetry(ctx, product.SpuName, skcNames)
	if err != nil {
		if sheinproduct.IsCostPriceUnavailable(err) {
			return map[string]resolvedSheinCost{}, nil
		}
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

func (r *sheinProductCostResolver) queryCostPriceWithRetry(ctx context.Context, spuName string, skcNames []string) (*sheinproduct.CostPriceQueryResponse, error) {
	response, err := r.productAPI.QueryCostPrice(spuName, skcNames)
	if err == nil {
		return response, nil
	}
	if !sheinproduct.IsCostPriceUnavailable(err) {
		return nil, err
	}
	lastErr := err

	sleep := r.sleep
	if sleep == nil {
		sleep = sleepSheinCostPriceRetry
	}
	for _, delay := range r.retryDelays {
		if sleepErr := sleep(ctx, delay); sleepErr != nil {
			return nil, sleepErr
		}
		response, err = r.productAPI.QueryCostPrice(spuName, skcNames)
		if err == nil {
			return response, nil
		}
		if !sheinproduct.IsCostPriceUnavailable(err) {
			return nil, err
		}
		lastErr = err
	}
	return nil, lastErr
}

func sleepSheinCostPriceRetry(ctx context.Context, delay time.Duration) error {
	if delay <= 0 {
		return nil
	}
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}
