package workspace

import (
	"testing"

	common "task-processor/internal/publishing/common"
	sheinpub "task-processor/internal/publishing/shein"
)

func TestApplyCategoryResolutionPatchSyncsPackageFields(t *testing.T) {
	t.Parallel()

	pkg := &sheinpub.Package{}
	categoryID := 123
	productTypeID := 456
	patch := &CategoryResolutionPatch{
		Status:         stringPtr(" resolved "),
		Source:         stringPtr(" manual "),
		QueryText:      stringPtr(" dress "),
		MatchedPath:    []string{"Women", "Dresses"},
		CategoryID:     &categoryID,
		CategoryIDList: []int{1, 12, 123},
		ProductTypeID:  &productTypeID,
		ReviewNotes:    []string{"check", "check", ""},
	}

	ApplyCategoryResolutionPatch(pkg, patch)

	if pkg.CategoryResolution == nil {
		t.Fatal("CategoryResolution is nil")
	}
	if pkg.CategoryResolution.Status != "resolved" || pkg.CategoryResolution.Source != "manual" {
		t.Fatalf("resolution status/source = %q/%q, want trimmed values", pkg.CategoryResolution.Status, pkg.CategoryResolution.Source)
	}
	if pkg.CategoryName != "Dresses" || pkg.CategoryID != 123 {
		t.Fatalf("package category = %q/%d, want Dresses/123", pkg.CategoryName, pkg.CategoryID)
	}
	if pkg.ProductTypeID == nil || *pkg.ProductTypeID != 456 {
		t.Fatalf("ProductTypeID = %#v, want 456", pkg.ProductTypeID)
	}
	if len(pkg.CategoryResolution.ReviewNotes) != 1 || pkg.CategoryResolution.ReviewNotes[0] != "check" {
		t.Fatalf("review notes = %#v, want unique non-empty note", pkg.CategoryResolution.ReviewNotes)
	}
}

func TestApplySKCRevisionPatchesSyncsDraftAndPackageSKUs(t *testing.T) {
	t.Parallel()

	pkg := &sheinpub.Package{
		DraftPayload: &sheinpub.RequestDraft{
			SKCList: []sheinpub.SKCRequestDraft{{
				SupplierCode: "SKC-1",
				SKUList: []sheinpub.SKUDraft{{
					SupplierSKU: "SKU-1",
				}},
			}},
		},
		SkcList: []sheinpub.SKCPackage{{
			SupplierCode: "SKC-1",
			SKUs: []common.Variant{{
				SKU: "SKU-1",
			}},
		}},
	}
	stock := 7
	image := " https://img.example/main.jpg "
	barcode := " 123456 "
	patches := []SKCRevisionPatch{{
		SupplierCode: " skc-1 ",
		SkcName:      stringPtr(" New SKC "),
		SaleName:     stringPtr(" Blue "),
		MainImageURL: &image,
		SKUPatches: []SKURevisionPatch{{
			SupplierSKU: " sku-1 ",
			Attributes:  map[string]string{"color": "Blue"},
			StockCount:  &stock,
			Barcode:     &barcode,
		}},
	}}

	ApplySKCRevisionPatches(pkg, patches)

	draft := &pkg.DraftPayload.SKCList[0]
	if draft.SkcName != "New SKC" || draft.SaleName != "Blue" {
		t.Fatalf("draft SKC names = %q/%q, want trimmed patch values", draft.SkcName, draft.SaleName)
	}
	if draft.ImageInfo == nil || draft.ImageInfo.MainImage != "https://img.example/main.jpg" {
		t.Fatalf("draft main image = %#v, want trimmed image", draft.ImageInfo)
	}
	if draft.SKUList[0].MainImage != "https://img.example/main.jpg" {
		t.Fatalf("first SKU main image = %q, want SKC fallback image", draft.SKUList[0].MainImage)
	}
	if draft.SKUList[0].StockCount != 7 || draft.SKUList[0].Barcode != "123456" {
		t.Fatalf("draft SKU = %+v, want stock/barcode patch", draft.SKUList[0])
	}
	if pkg.SkcList[0].MainImageURL != "https://img.example/main.jpg" || pkg.SkcList[0].SKUs[0].Stock != 7 {
		t.Fatalf("package SKC/SKU = %+v, want synced image and stock", pkg.SkcList[0])
	}
	if pkg.SkcList[0].SKUs[0].Attributes["color"] != "Blue" {
		t.Fatalf("package SKU attributes = %#v, want cloned color", pkg.SkcList[0].SKUs[0].Attributes)
	}
}

func stringPtr(value string) *string {
	return &value
}
