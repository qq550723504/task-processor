package publishing

import "math"

// CostPriceRule is the numeric input to the SHEIN cost-conversion calculation.
// It does not own settings, persisted rule snapshots, or manual/draft prices.
type CostPriceRule struct {
	ExchangeRate     float64
	MarkupMultiplier float64
	MinimumPrice     float64
	RoundTo          float64
	PriceEnding      float64
}

// CalculateCostPrice applies cost conversion, minimum price, price ending,
// increment ceiling, and two-decimal rounding in that order. It deliberately
// does not supply defaults or act as a readiness/authorization check.
func CalculateCostPrice(costCNY float64, rule CostPriceRule) float64 {
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
