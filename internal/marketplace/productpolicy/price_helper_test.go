package productpolicy

import (
	"os"
	"os/exec"
	"strings"
	"testing"

	"task-processor/internal/model"
)

func TestNilProductHelpersReturnZeroWithoutProcessLogs(t *testing.T) {
	const childEnv = "TASK_PROCESSOR_NIL_PRODUCT_HELPERS_CHILD"
	if os.Getenv(childEnv) == "1" {
		if price := GetProductPrice(nil, "special"); price != 0 {
			t.Fatalf("GetProductPrice(nil) = %v, want 0", price)
		}
		if inventory := GetInventory(nil); inventory != 0 {
			t.Fatalf("GetInventory(nil) = %d, want 0", inventory)
		}
		return
	}

	cmd := exec.Command(os.Args[0], "-test.run=^TestNilProductHelpersReturnZeroWithoutProcessLogs$")
	cmd.Env = append(os.Environ(), childEnv+"=1")
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("nil-helper child test failed: %v\n%s", err, output)
	}
	if strings.TrimSpace(string(output)) != "PASS" {
		t.Fatalf("nil product helpers wrote process logs: %q", output)
	}
}

func TestGetProductPriceOriginalUsesListPriceFirst(t *testing.T) {
	listPrice := 29.99
	product := &model.Product{
		InitialPrice: 19.99,
		FinalPrice:   9.99,
		PricesBreakdown: model.PriceBreakdown{
			ListPrice: &listPrice,
		},
	}

	price := GetProductPrice(product, "original")
	if price != 29.99 {
		t.Fatalf("GetProductPrice(original) = %v, want 29.99", price)
	}
}

func TestGetProductPriceOriginalFallsBackToInitialPrice(t *testing.T) {
	product := &model.Product{
		InitialPrice: 19.99,
		FinalPrice:   9.99,
	}

	price := GetProductPrice(product, "original")
	if price != 19.99 {
		t.Fatalf("GetProductPrice(original) = %v, want 19.99", price)
	}
}

func TestGetProductPriceOriginalFallsBackToFinalPriceWhenOriginalMissing(t *testing.T) {
	product := &model.Product{
		InitialPrice: 0,
		FinalPrice:   6.19,
	}

	price := GetProductPrice(product, "original")
	if price != 6.19 {
		t.Fatalf("GetProductPrice(original) = %v, want 6.19", price)
	}
}
