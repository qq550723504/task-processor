package shein

import (
	"context"
	"testing"

	"task-processor/internal/catalog/canonical"
	common "task-processor/internal/publishing/common"
	sharedtenantctx "task-processor/internal/shared/tenantctx"
	"task-processor/internal/shein/authorizedbrand"
)

func TestBuildPreviewProductUsesAuthorizedBrandCode(t *testing.T) {
	t.Parallel()

	pkg := &Package{
		Metadata: map[string]string{
			"authorized_brand_code": "2fd1n",
		},
		DraftPayload: &RequestDraft{
			SpuName:      "Authorized Brand Product",
			SupplierCode: "SUP-1",
		},
	}

	got := BuildPreviewProduct(pkg)
	if got == nil {
		t.Fatal("BuildPreviewProduct() = nil, want product")
	}
	if got.BrandCode == nil || *got.BrandCode != "2fd1n" {
		t.Fatalf("BrandCode = %#v, want 2fd1n", got.BrandCode)
	}
}

func TestAssemblerBuildWritesAuthorizedBrandMetadataAndPreviewBrandCode(t *testing.T) {
	t.Parallel()

	req := &BuildRequest{
		Country:   "US",
		Language:  "en",
		BrandHint: "Generic Brand",
		Context: authorizedbrand.WithResolved(context.Background(), &authorizedbrand.Resolved{
			Enabled: true,
			Code:    "2fd1n",
			Name:    "Authorized Brand",
			NameEn:  "Authorized Brand EN",
		}),
	}
	product := &canonical.Product{
		Title:  "Canvas Wall Art",
		Brand:  "Catalog Brand",
		Images: []canonical.Image{{URL: "main.jpg"}},
	}

	pkg := NewAssembler(AssemblerConfig{}).Build(req, product, nil)
	if pkg == nil {
		t.Fatal("Build() = nil, want package")
	}
	if pkg.BrandName != "Generic Brand" {
		t.Fatalf("BrandName = %q, want generic brand preserved for non-submit surfaces", pkg.BrandName)
	}
	if pkg.Metadata["authorized_brand_code"] != "2fd1n" {
		t.Fatalf("authorized_brand_code = %q, want 2fd1n", pkg.Metadata["authorized_brand_code"])
	}
	if pkg.Metadata["authorized_brand_name"] != "Authorized Brand" {
		t.Fatalf("authorized_brand_name = %q, want Authorized Brand", pkg.Metadata["authorized_brand_name"])
	}
	if pkg.PreviewPayload == nil {
		t.Fatal("PreviewPayload = nil, want final preview payload")
	}
	if pkg.PreviewPayload.BrandCode == nil || *pkg.PreviewPayload.BrandCode != "2fd1n" {
		t.Fatalf("PreviewPayload.BrandCode = %#v, want 2fd1n", pkg.PreviewPayload.BrandCode)
	}
}

func TestSanitizeSheinListingCopyPreservesAuthorizedBrandFromRuntimeContext(t *testing.T) {
	t.Parallel()

	copy := &listingCopy{
		Title:        "Amazon Basics Wireless Mouse",
		Description:  "Amazon Basics office mouse for home use",
		SKCTitleBase: "Amazon Basics Mouse",
	}

	changed := sanitizeSheinListingCopy(copy, authorizedbrand.WithResolved(context.Background(), &authorizedbrand.Resolved{
		Enabled: true,
		Name:    "Amazon Basics",
		NameEn:  "Amazon Basics",
	}), nil)
	if changed {
		t.Fatalf("changed = true, want authorized brand preserved without cleanup")
	}
	if copy.Title != "Amazon Basics Wireless Mouse" || copy.Description != "Amazon Basics office mouse for home use" || copy.SKCTitleBase != "Amazon Basics Mouse" {
		t.Fatalf("copy after sanitize = %+v", copy)
	}
}

func TestSanitizeDraftPayloadSensitiveContentPreservesAuthorizedBrandFromRuntimeContext(t *testing.T) {
	t.Parallel()

	pkg := &Package{
		ProductAttributes: []common.Attribute{
			{Name: "Material Detail", Value: "Amazon Basics acrylic"},
		},
		DraftPayload: &RequestDraft{
			MultiLanguageNameList: []LocalizedText{{Language: "en", Name: "Amazon Basics Wireless Mouse"}},
			MultiLanguageDescList: []LocalizedText{{Language: "en", Name: "Amazon Basics office mouse"}},
			ProductAttributeList:  []common.Attribute{{Name: "Material Detail", Value: "Amazon Basics acrylic"}},
			SKCList: []SKCRequestDraft{{
				SkcName:               "Amazon Basics Blue",
				MultiLanguageNameList: []LocalizedText{{Language: "en", Name: "Amazon Basics Blue"}},
			}},
		},
	}

	runtimeCtx := sharedtenantctx.WithTenantID(authorizedbrand.WithResolved(context.Background(), &authorizedbrand.Resolved{
		Enabled: true,
		Name:    "Amazon Basics",
		NameEn:  "Amazon Basics",
	}), "101")

	changed := SanitizeDraftPayloadSensitiveContent(pkg, runtimeCtx, nil)
	if changed {
		t.Fatalf("changed = true, want authorized brand preserved without cleanup")
	}
	if pkg.DraftPayload.SKCList[0].SkcName != "Amazon Basics Blue" {
		t.Fatalf("SkcName = %q, want authorized brand preserved", pkg.DraftPayload.SKCList[0].SkcName)
	}
	if pkg.DraftPayload.ProductAttributeList[0].Value != "Amazon Basics acrylic" {
		t.Fatalf("ProductAttributeList[0].Value = %q, want authorized brand preserved", pkg.DraftPayload.ProductAttributeList[0].Value)
	}
}
