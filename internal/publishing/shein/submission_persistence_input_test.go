package shein

import (
	"testing"

	sheinproduct "task-processor/internal/shein/api/product"
)

func TestResolveSubmissionSupplierCodePrefersProductThenPackageThenSPUName(t *testing.T) {
	t.Parallel()

	skcSupplierCode := " SKC-SUP "
	product := &sheinproduct.Product{
		SPUName:      "SPU-1",
		SupplierCode: "  ",
		SKCList: []sheinproduct.SKC{
			{SupplierCode: &skcSupplierCode},
		},
	}
	product.SupplierCode = "  "
	product.SKCList[0].SupplierCode = &skcSupplierCode

	pkg := &Package{
		SkcList: []SKCPackage{
			{SupplierCode: "PKG-SUP"},
		},
	}

	if got := ResolveSubmissionSupplierCode(product, pkg); got != "SKC-SUP" {
		t.Fatalf("ResolveSubmissionSupplierCode() = %q, want SKC-SUP", got)
	}

	product.SKCList[0].SupplierCode = nil
	if got := ResolveSubmissionSupplierCode(product, pkg); got != "PKG-SUP" {
		t.Fatalf("ResolveSubmissionSupplierCode() package fallback = %q, want PKG-SUP", got)
	}

	pkg.SkcList = nil
	if got := ResolveSubmissionSupplierCode(product, pkg); got != product.SPUName {
		t.Fatalf("ResolveSubmissionSupplierCode() SPU fallback = %q, want %q", got, product.SPUName)
	}
}

func TestPrepareSubmissionPersistenceInputPersistsSupplierCodeAndSnapshot(t *testing.T) {
	t.Parallel()

	product := &sheinproduct.Product{
		SPUName:               "SPU-1",
		SupplierCode:          "SUP-1",
		MultiLanguageNameList: []sheinproduct.LanguageContent{{Language: "en", Name: "Name"}},
		MultiLanguageDescList: []sheinproduct.LanguageContent{{Language: "en", Name: "Desc"}},
		SKCList: []sheinproduct.SKC{
			{
				MultiLanguageName:     sheinproduct.LanguageContent{Name: "SKC Name"},
				MultiLanguageNameList: []sheinproduct.LanguageContent{{Language: "en", Name: "SKC Name"}},
			},
		},
	}
	pkg := &Package{}
	record := &SubmissionRecord{
		Action:    "publish",
		Status:    SubmissionStatusRunning,
		RequestID: "req-1",
	}
	ApplySubmissionRecord(pkg, record)

	supplierCode, snapshot := PrepareSubmissionPersistenceInput(pkg, "publish", "req-1", product, nil)
	if supplierCode != "SUP-1" {
		t.Fatalf("PrepareSubmissionPersistenceInput() supplierCode = %q, want SUP-1", supplierCode)
	}
	if snapshot == nil || snapshot.SPUName != "SPU-1" {
		t.Fatalf("PrepareSubmissionPersistenceInput() snapshot = %+v, want SPU-1 snapshot", snapshot)
	}

	saved := pkg.SubmissionState.Publish
	if saved == nil {
		t.Fatal("submission record missing after PrepareSubmissionPersistenceInput()")
	}
	if saved.SupplierCode != "SUP-1" {
		t.Fatalf("saved supplier code = %q, want SUP-1", saved.SupplierCode)
	}
	if saved.SubmitSnapshot == nil || saved.SubmitSnapshot.SPUName != "SPU-1" {
		t.Fatalf("saved submit snapshot = %+v, want SPU-1", saved.SubmitSnapshot)
	}
}
