package workspace

import (
	"errors"
	"testing"

	common "task-processor/internal/publishing/common"
	sheinpub "task-processor/internal/publishing/shein"
)

func TestBuildPackageTemplateValidationReportsReadyPackage(t *testing.T) {
	t.Parallel()

	productTypeID := 2001
	valueID := 3001
	pkg := &sheinpub.Package{
		CategoryID:    1001,
		ProductTypeID: &productTypeID,
		CategoryResolution: &sheinpub.CategoryResolution{
			Status: "resolved",
		},
		AttributeResolution: &sheinpub.AttributeResolution{
			Status:        "resolved",
			ResolvedCount: 1,
		},
		ResolvedAttributes: []sheinpub.ResolvedAttribute{{AttributeID: 10}},
		SaleAttributeResolution: &sheinpub.SaleAttributeResolution{
			Status:             "resolved",
			PrimaryAttributeID: 40,
		},
		DraftPayload: &sheinpub.RequestDraft{SKCList: []sheinpub.SKCRequestDraft{{
			SaleAttribute: &sheinpub.ResolvedSaleAttribute{AttributeID: 40, AttributeValueID: &valueID},
			SKUList:       []sheinpub.SKUDraft{{SupplierSKU: "SKU-1"}},
		}}},
	}

	validation := BuildPackageTemplateValidation(pkg, nil)

	if !validation.CategoryReady || !validation.CategoryReviewReady || !validation.AttributeReady || !validation.SaleAttributeReady || !validation.SubmitPayloadReady {
		t.Fatalf("validation = %+v, want all ready", validation)
	}
}

func TestBuildPackageTemplateValidationReportsBlockingInputs(t *testing.T) {
	t.Parallel()

	pkg := &sheinpub.Package{
		CategoryID: 1001,
		CategoryResolution: &sheinpub.CategoryResolution{
			Status: "resolved",
		},
		AttributeResolution: &sheinpub.AttributeResolution{
			Status:            "resolved",
			ResolvedCount:     1,
			PendingAttributes: []common.Attribute{{Name: "Material"}},
		},
		SaleAttributeResolution: &sheinpub.SaleAttributeResolution{
			RecommendCategoryReview: true,
		},
	}
	err := errors.New("SHEIN publish blocked: SKC[0] SKU[0] is missing package_type")

	validation := BuildPackageTemplateValidation(pkg, err)

	if validation.CategoryReady {
		t.Fatal("CategoryReady = true, want false without product_type_id")
	}
	if validation.CategoryReviewReady {
		t.Fatal("CategoryReviewReady = true, want false when category review pending")
	}
	if validation.AttributeReady {
		t.Fatal("AttributeReady = true, want false with pending attribute")
	}
	if validation.SaleAttributeReady {
		t.Fatal("SaleAttributeReady = true, want false with unresolved sale attributes")
	}
	if validation.SubmitPayloadReady {
		t.Fatal("SubmitPayloadReady = true, want false with prepared payload error")
	}
	if validation.SubmitPayloadMessage != err.Error() {
		t.Fatalf("SubmitPayloadMessage = %q, want %q", validation.SubmitPayloadMessage, err.Error())
	}
}
