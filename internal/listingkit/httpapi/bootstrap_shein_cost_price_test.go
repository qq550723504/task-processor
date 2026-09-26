package httpapi

import "testing"

func TestBuildListingKitSheinDependenciesAcceptsCostPriceCalculator(t *testing.T) {
	calculator := func(float64, float64, float64, float64, float64, float64) float64 { return 20.5 }
	dependencies := buildListingKitSheinDependencies(buildListingKitServiceConfigInput{
		repositories: &builtRepositories{},
		input:        BuildServiceInput{SheinCostPriceCalculator: calculator},
	})
	if dependencies.SheinCostPriceCalculator == nil {
		t.Fatal("SHEIN cost-price calculator is not wired")
	}
	if got := dependencies.SheinCostPriceCalculator(80, 8, 2, 9, 0.5, 0.49); got != 20.5 {
		t.Fatalf("cost price = %v, want 20.5", got)
	}
}
