package publishing

import (
	"math"
	"strings"
)

const (
	minSubmitWeightGrams = 0.01
	maxSubmitWeightGrams = 50000000
)

// NormalizeSubmitWeightGrams converts submit weight to grams and clamps it to SHEIN submit bounds.
func NormalizeSubmitWeightGrams(value float64, unit string) float64 {
	weight := convertSubmitWeightToGrams(value, unit)
	if weight <= 0 {
		weight = minSubmitWeightGrams
	}
	if weight < minSubmitWeightGrams {
		weight = minSubmitWeightGrams
	}
	if weight > maxSubmitWeightGrams {
		weight = maxSubmitWeightGrams
	}
	return roundSubmitWeightGrams(weight)
}

func convertSubmitWeightToGrams(value float64, unit string) float64 {
	if value <= 0 {
		return 0
	}
	switch strings.ToLower(strings.TrimSpace(unit)) {
	case "", "g", "gram", "grams":
		return value
	case "kg", "kilogram", "kilograms":
		return value * 1000
	case "lb", "lbs", "pound", "pounds":
		return value * 453.59237
	case "oz", "ounce", "ounces":
		return value * 28.349523125
	case "mg", "milligram", "milligrams":
		return value / 1000
	default:
		return value
	}
}

func roundSubmitWeightGrams(value float64) float64 {
	return math.Round(value*100) / 100
}
