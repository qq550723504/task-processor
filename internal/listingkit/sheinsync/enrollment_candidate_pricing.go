package sheinsync

import (
	"context"
	"strings"
)

type sheinEnrollmentCandidatePricingRepository interface {
	ListSyncedProducts(context.Context, *SheinSyncedProductQuery) ([]SheinSyncedProductRecord, int64, error)
}

func refreshSheinEnrollmentCandidatePricing(
	ctx context.Context,
	repo sheinEnrollmentCandidatePricingRepository,
	tenantID, storeID int64,
	candidates []SheinActivityCandidateRecord,
) ([]SheinActivityCandidateRecord, error) {
	if repo == nil || len(candidates) == 0 {
		return candidates, nil
	}

	active := true
	products := make([]SheinSyncedProductRecord, 0, len(candidates))
	for _, candidate := range candidates {
		if candidate.SKCName == "" {
			continue
		}
		rows, _, err := repo.ListSyncedProducts(ctx, &SheinSyncedProductQuery{
			TenantID: tenantID,
			StoreID:  storeID,
			SKCName:  candidate.SKCName,
			IsActive: &active,
			Page:     1,
			PageSize: 1,
		})
		if err != nil {
			return nil, err
		}
		if len(rows) > 0 {
			products = append(products, rows[0])
		}
	}
	if len(products) == 0 {
		return candidates, nil
	}

	if groupReader, ok := repo.(sheinCandidateSDSCostGroupReader); ok {
		var err error
		products, err = applySheinSDSCostGroupOverrides(ctx, groupReader, tenantID, storeID, products)
		if err != nil {
			return nil, err
		}
	}

	productBySKC := make(map[string]SheinSyncedProductRecord, len(products))
	for _, product := range products {
		if product.SKCName == "" || product.EffectiveCostPrice == nil {
			continue
		}
		productBySKC[product.SKCName] = product
	}
	if len(productBySKC) == 0 {
		return candidates, nil
	}

	out := make([]SheinActivityCandidateRecord, len(candidates))
	copy(out, candidates)
	for i := range out {
		product, ok := productBySKC[out[i].SKCName]
		if !ok {
			continue
		}
		out[i].EffectiveCostPrice = cloneSheinSyncFloat64(product.EffectiveCostPrice)
		out[i].PriceSnapshot = refreshSheinEnrollmentPriceSnapshot(out[i].PriceSnapshot, product)
		out[i].SKUCostPriceInfoList = alignSheinEnrollmentSKUCostCodes(
			out[i].PriceSnapshot,
			product.SKUCostPriceInfoList,
		)
		out[i].CalculatedProfitRate = calculateSheinCandidateProfitRate(out[i].EffectiveCostPrice, out[i].PriceSnapshot)
	}
	return out, nil
}

// alignSheinEnrollmentSKUCostCodes preserves the SKU spelling from the price snapshot.
// SDS cost identities are normalized to uppercase, while SHEIN price snapshots retain
// the upstream spelling that should also be used for display and enrollment payloads.
func alignSheinEnrollmentSKUCostCodes(
	priceSnapshot string,
	costs []SheinSKUCostPrice,
) []SheinSKUCostPrice {
	if len(costs) == 0 {
		return nil
	}

	priceSKUByKey := make(map[string]string)
	for _, price := range parsePromotionCandidatePriceSnapshot(priceSnapshot).SKUPrices {
		skuCode := strings.TrimSpace(price.SKUCode)
		if skuCode == "" {
			continue
		}
		priceSKUByKey[strings.ToUpper(skuCode)] = skuCode
	}

	out := cloneSheinSKUCostPriceList(costs)
	for i := range out {
		if skuCode, ok := priceSKUByKey[strings.ToUpper(strings.TrimSpace(out[i].SKUCode))]; ok {
			out[i].SKUCode = skuCode
		}
	}
	return out
}
