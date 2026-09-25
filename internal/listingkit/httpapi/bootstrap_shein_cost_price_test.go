package httpapi

import (
	"testing"

	"task-processor/internal/listingkit"
)

func TestBuildListingKitSheinDependenciesAcceptsCostPriceCalculator(t *testing.T) {
	calculator := func(float64, listingkit.SheinCostPriceInput) float64 { return 20.5 }
	dependencies := buildListingKitSheinDependencies(buildListingKitServiceConfigInput{
		repositories: &builtRepositories{},
		input:        BuildServiceInput{SheinCostPriceCalculator: calculator},
	})
	if dependencies.SheinCostPriceCalculator == nil {
		t.Fatal("SHEIN cost-price calculator is not wired")
	}
	input := listingkit.SheinCostPriceInput{
		ExchangeRate: 8, MarkupMultiplier: 2, MinimumPrice: 9,
		RoundTo: 0.5, PriceEnding: 0.49,
	}
	if got := dependencies.SheinCostPriceCalculator(80, input); got != 20.5 {
		t.Fatalf("cost price = %v, want 20.5", got)
	}
}
