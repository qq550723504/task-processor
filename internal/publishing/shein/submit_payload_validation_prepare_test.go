package shein

import (
	"reflect"
	"testing"

	sheinproduct "task-processor/internal/shein/api/product"
)

func TestPrepareProductForValidationMatchesSubmitWithoutGeneratingIdentity(t *testing.T) {
	makeProduct := func() *sheinproduct.Product {
		return &sheinproduct.Product{SPUName: "title", SKCList: []sheinproduct.SKC{{SKUS: []sheinproduct.SKU{{SupplierSKU: "sku"}}}}}
	}
	offline, submit := makeProduct(), makeProduct()
	PrepareProductForValidation(offline)
	if offline.PointKey != "" {
		t.Fatal("validation generated a submission identity")
	}
	PrepareProductForNewSubmit(submit)
	if submit.PointKey == "" {
		t.Fatal("submit must still generate its own identity")
	}
	submit.PointKey = ""
	if !reflect.DeepEqual(offline, submit) {
		t.Fatalf("validation normalization differs from submit: %+v %+v", offline, submit)
	}
	PrepareProductForValidation(nil)
}
