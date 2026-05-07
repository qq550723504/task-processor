package shein

import (
	"testing"

	sheinproduct "task-processor/internal/shein/api/product"
)

func TestPreValidateSubmitProductRejectsMissingSupplierCode(t *testing.T) {
	productTypeID := 456
	err := PreValidateSubmitProduct(&sheinproduct.Product{
		SPUName:       "Test Product",
		CategoryID:    123,
		ProductTypeID: &productTypeID,
	})
	if err == nil {
		t.Fatalf("PreValidateSubmitProduct() returned nil, want validation error")
	}
}
