package shein

import (
	"math"
	"strings"

	common "task-processor/internal/publishing/common"
)

const defaultPricingCurrency = "USD"

type PricingPolicy struct {
	Enabled        bool
	Currency       string
	MarkupRate     float64
	FixedMarkup    float64
	ShippingCost   float64
	CommissionRate float64
	MinimumPrice   float64
	RoundTo        float64
}

func (p PricingPolicy) Apply(source *common.Price) *common.Price {
	if source == nil {
		return nil
	}

	result := &common.Price{
		Currency:  firstPricingCurrency(p.Currency, source.Currency),
		Amount:    source.Amount,
		CostPrice: source.CostPrice,
	}
	if !p.Enabled {
		return result
	}

	cost := firstPositive(source.CostPrice, source.Amount)
	if cost <= 0 {
		return result
	}
	result.CostPrice = cost

	amount := cost + maxZero(p.ShippingCost) + maxZero(p.FixedMarkup)
	amount *= 1 + maxZero(p.MarkupRate)
	if p.CommissionRate > 0 && p.CommissionRate < 0.95 {
		amount = amount / (1 - p.CommissionRate)
	}
	if p.MinimumPrice > 0 && amount < p.MinimumPrice {
		amount = p.MinimumPrice
	}
	result.Amount = roundPriceUp(amount, p.RoundTo)
	return result
}

func firstPricingCurrency(values ...string) string {
	for _, value := range values {
		if trimmed := strings.ToUpper(strings.TrimSpace(value)); trimmed != "" {
			return trimmed
		}
	}
	return defaultPricingCurrency
}

func firstPositive(values ...float64) float64 {
	for _, value := range values {
		if value > 0 {
			return value
		}
	}
	return 0
}

func maxZero(value float64) float64 {
	if value < 0 {
		return 0
	}
	return value
}

func roundPriceUp(value, increment float64) float64 {
	if value <= 0 {
		return 0
	}
	if increment <= 0 {
		increment = 0.01
	}
	rounded := math.Ceil(value/increment) * increment
	return math.Round(rounded*1e9) / 1e9
}
