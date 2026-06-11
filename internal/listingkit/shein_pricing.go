package listingkit

import (
	"math"
	"strconv"
	"strings"
	"time"

	sheinpub "task-processor/internal/publishing/shein"
	sheinproduct "task-processor/internal/shein/api/product"
)

func (s *service) currentSheinPricingRule() sheinpub.PricingRule {
	s.sheinSettingsMu.RLock()
	defer s.sheinSettingsMu.RUnlock()
	return normalizeSheinPricingRule(s.sheinSettings.Pricing, sheinpub.PricingRule{})
}

func normalizeSheinPricingRule(input sheinpub.PricingRule, fallback sheinpub.PricingRule) sheinpub.PricingRule {
	rule := fallback
	if strings.TrimSpace(rule.SourceCurrency) == "" {
		rule.SourceCurrency = "CNY"
	}
	if strings.TrimSpace(rule.TargetCurrency) == "" {
		rule.TargetCurrency = "USD"
	}
	if rule.ExchangeRate <= 0 {
		rule.ExchangeRate = 7.2
	}
	if rule.MarkupMultiplier <= 0 {
		rule.MarkupMultiplier = 2
	}
	if rule.MinimumPrice <= 0 {
		rule.MinimumPrice = 9.99
	}
	if rule.RoundTo <= 0 {
		rule.RoundTo = 0.01
	}
	if strings.TrimSpace(input.SourceCurrency) != "" {
		rule.SourceCurrency = strings.ToUpper(strings.TrimSpace(input.SourceCurrency))
	}
	if strings.TrimSpace(input.TargetCurrency) != "" {
		rule.TargetCurrency = strings.ToUpper(strings.TrimSpace(input.TargetCurrency))
	}
	if input.ExchangeRate > 0 {
		rule.ExchangeRate = input.ExchangeRate
	}
	if input.MarkupMultiplier > 0 {
		rule.MarkupMultiplier = input.MarkupMultiplier
	}
	if input.MinimumPrice > 0 {
		rule.MinimumPrice = input.MinimumPrice
	}
	if input.RoundTo > 0 {
		rule.RoundTo = input.RoundTo
	}
	if input.PriceEnding > 0 {
		rule.PriceEnding = input.PriceEnding
	}
	return rule
}

func buildSheinPricingReview(pkg *sheinpub.Package, rule sheinpub.PricingRule, overrides map[string]float64) *sheinpub.PricingReview {
	review := &sheinpub.PricingReview{
		RuleSnapshot:    &rule,
		ManualOverrides: clonePriceOverrides(overrides),
		Ready:           true,
	}
	now := time.Now()
	review.UpdatedAt = &now
	pkg = sheinpub.NormalizePackageSemanticFields(pkg)
	if pkg == nil || pkg.DraftPayload == nil {
		review.Ready = false
		return review
	}
	for _, skc := range pkg.DraftPayload.SKCList {
		for _, sku := range skc.SKUList {
			cost := parseMoney(sku.CostPrice)
			price := calculateSheinPrice(cost, rule)
			finalPrice := price
			manual := false
			if value, ok := overrides[sku.SupplierSKU]; ok && value > 0 {
				finalPrice = value
				manual = true
			}
			if finalPrice <= 0 {
				review.Ready = false
				review.MissingPriceSKUs = append(review.MissingPriceSKUs, sku.SupplierSKU)
			}
			review.SKUPrices = append(review.SKUPrices, sheinpub.SKUPriceReview{
				SupplierSKU:     sku.SupplierSKU,
				SupplierCode:    skc.SupplierCode,
				CostCNY:         cost,
				CalculatedPrice: price,
				FinalPrice:      finalPrice,
				Currency:        rule.TargetCurrency,
				Manual:          manual,
			})
		}
	}
	return review
}

func buildSheinDraftBackedPricingReview(pkg *sheinpub.Package, rule sheinpub.PricingRule, overrides map[string]float64) *sheinpub.PricingReview {
	review := &sheinpub.PricingReview{
		RuleSnapshot:    &rule,
		ManualOverrides: clonePriceOverrides(overrides),
		Ready:           true,
	}
	now := time.Now()
	review.UpdatedAt = &now
	pkg = sheinpub.NormalizePackageSemanticFields(pkg)
	if pkg == nil || pkg.DraftPayload == nil {
		review.Ready = false
		return review
	}
	for _, skc := range pkg.DraftPayload.SKCList {
		for _, sku := range skc.SKUList {
			cost := parseMoney(sku.CostPrice)
			price := existingSheinDraftPrice(sku)
			if price <= 0 {
				price = calculateSheinPrice(cost, rule)
			}
			finalPrice := price
			manual := false
			if value, ok := overrides[sku.SupplierSKU]; ok && value > 0 {
				finalPrice = value
				manual = true
			}
			if finalPrice <= 0 {
				review.Ready = false
				review.MissingPriceSKUs = append(review.MissingPriceSKUs, sku.SupplierSKU)
			}
			review.SKUPrices = append(review.SKUPrices, sheinpub.SKUPriceReview{
				SupplierSKU:     sku.SupplierSKU,
				SupplierCode:    skc.SupplierCode,
				CostCNY:         cost,
				CalculatedPrice: price,
				FinalPrice:      finalPrice,
				Currency:        normalizeSheinReviewCurrency(existingSheinDraftCurrency(sku, rule.TargetCurrency), rule),
				Manual:          manual,
			})
		}
	}
	return review
}

func applySheinPricingReview(pkg *sheinpub.Package, review *sheinpub.PricingReview) {
	pkg = sheinpub.NormalizePackageSemanticFields(pkg)
	if pkg == nil || review == nil {
		return
	}
	pkg.Pricing = review
	rule := sheinpub.PricingRule{SourceCurrency: "CNY", TargetCurrency: "USD", ExchangeRate: 7.2}
	if review.RuleSnapshot != nil {
		rule = normalizeSheinPricingRule(*review.RuleSnapshot, rule)
	}
	priceBySKU := make(map[string]sheinpub.SKUPriceReview, len(review.SKUPrices))
	for _, price := range review.SKUPrices {
		price.Currency = normalizeSheinReviewCurrency(price.Currency, rule)
		priceBySKU[price.SupplierSKU] = price
	}
	if pkg.DraftPayload != nil {
		for skcIndex := range pkg.DraftPayload.SKCList {
			for skuIndex := range pkg.DraftPayload.SKCList[skcIndex].SKUList {
				sku := &pkg.DraftPayload.SKCList[skcIndex].SKUList[skuIndex]
				if price, ok := priceBySKU[sku.SupplierSKU]; ok && price.FinalPrice > 0 {
					value := formatMoney(price.FinalPrice)
					sku.Currency = price.Currency
					sku.BasePrice = value
					if len(sku.SitePriceList) == 0 {
						sku.SitePriceList = []sheinpub.SitePrice{{SubSite: "US"}}
					}
					for i := range sku.SitePriceList {
						sku.SitePriceList[i].BasePrice = value
						sku.SitePriceList[i].Currency = price.Currency
					}
				}
			}
		}
	}
	if pkg.PreviewPayload != nil {
		applySheinPreviewProductPrices(pkg.PreviewPayload, priceBySKU, rule)
	}
}

func applySheinPreviewProductPrices(product *sheinproduct.Product, prices map[string]sheinpub.SKUPriceReview, rule sheinpub.PricingRule) {
	if product == nil {
		return
	}
	targetCurrency := strings.ToUpper(strings.TrimSpace(rule.TargetCurrency))
	if targetCurrency == "" {
		targetCurrency = "USD"
	}
	for skcIndex := range product.SKCList {
		for skuIndex := range product.SKCList[skcIndex].SKUS {
			sku := &product.SKCList[skcIndex].SKUS[skuIndex]
			price, ok := prices[sku.SupplierSKU]
			if !ok || price.FinalPrice <= 0 {
				continue
			}
			if len(sku.PriceInfoList) == 0 {
				sku.PriceInfoList = []sheinproduct.PriceInfo{{SubSite: "US"}}
			}
			for i := range sku.PriceInfoList {
				sku.PriceInfoList[i].BasePrice = price.FinalPrice
				sku.PriceInfoList[i].Currency = price.Currency
			}
			if sku.CostInfo == nil {
				sku.CostInfo = &sheinproduct.CostInfo{}
			}
			sku.CostInfo.CostPrice = formatMoney(price.FinalPrice)
			sku.CostInfo.Currency = targetCurrency
		}
	}
}

func calculateSheinPrice(costCNY float64, rule sheinpub.PricingRule) float64 {
	if costCNY <= 0 || rule.ExchangeRate <= 0 {
		return 0
	}
	price := costCNY / rule.ExchangeRate * rule.MarkupMultiplier
	if price < rule.MinimumPrice {
		price = rule.MinimumPrice
	}
	if rule.PriceEnding > 0 && rule.PriceEnding < 1 {
		base := math.Floor(price)
		candidate := base + rule.PriceEnding
		if candidate < price {
			candidate = base + 1 + rule.PriceEnding
		}
		price = candidate
	}
	if rule.RoundTo > 0 {
		price = math.Ceil(price/rule.RoundTo) * rule.RoundTo
	}
	return math.Round(price*100) / 100
}

func parseMoney(value string) float64 {
	value = strings.TrimSpace(value)
	if value == "" {
		return 0
	}
	parsed, _ := strconv.ParseFloat(value, 64)
	return parsed
}

func formatMoney(value float64) string {
	return strconv.FormatFloat(math.Round(value*100)/100, 'f', 2, 64)
}

func clonePriceOverrides(input map[string]float64) map[string]float64 {
	if len(input) == 0 {
		return nil
	}
	out := make(map[string]float64, len(input))
	for sku, price := range input {
		if strings.TrimSpace(sku) != "" && price > 0 {
			out[sku] = price
		}
	}
	return out
}

func existingSheinDraftPrice(sku sheinpub.SKUDraft) float64 {
	if value := parseMoney(sku.BasePrice); value > 0 {
		return value
	}
	for _, item := range sku.SitePriceList {
		if value := parseMoney(item.BasePrice); value > 0 {
			return value
		}
	}
	return 0
}

func existingSheinDraftCurrency(sku sheinpub.SKUDraft, fallback string) string {
	if value := strings.ToUpper(strings.TrimSpace(sku.Currency)); value != "" {
		return value
	}
	for _, item := range sku.SitePriceList {
		if value := strings.ToUpper(strings.TrimSpace(item.Currency)); value != "" {
			return value
		}
	}
	fallback = strings.ToUpper(strings.TrimSpace(fallback))
	if fallback == "" {
		return "USD"
	}
	return fallback
}

func normalizeSheinReviewCurrency(currency string, rule sheinpub.PricingRule) string {
	sourceCurrency := strings.ToUpper(strings.TrimSpace(rule.SourceCurrency))
	if sourceCurrency == "" {
		sourceCurrency = "CNY"
	}
	targetCurrency := strings.ToUpper(strings.TrimSpace(rule.TargetCurrency))
	if targetCurrency == "" {
		targetCurrency = "USD"
	}
	currency = strings.ToUpper(strings.TrimSpace(currency))
	if currency == "" || currency == sourceCurrency {
		return targetCurrency
	}
	return currency
}
