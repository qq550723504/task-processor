package workspace

import (
	"testing"

	sheinpub "task-processor/internal/publishing/shein"
	sheinproduct "task-processor/internal/shein/api/product"
)

func TestBuildSubmitPayloadReadinessChecksReportsMissingPayloadParts(t *testing.T) {
	t.Parallel()

	checks := BuildSubmitPayloadReadinessChecks(&sheinpub.Package{}, "publish")

	assertReadinessCheck(t, checks, "request_draft", false)
	assertReadinessCheck(t, checks, "preview_product", false)
	assertReadinessCheck(t, checks, "images", false)
	assertReadinessCheck(t, checks, "variants", false)
}

func TestBuildSubmitPayloadReadinessChecksUsesPackagePredicates(t *testing.T) {
	t.Parallel()

	pkg := &sheinpub.Package{
		DraftPayload: &sheinpub.RequestDraft{
			ImageInfo: &sheinpub.ImageDraft{MainImage: "https://img.example/main.jpg"},
			SKCList: []sheinpub.SKCRequestDraft{{
				SupplierCode: "SKC-1",
				ImageInfo:    &sheinpub.ImageDraft{MainImage: "https://img.example/skc.jpg"},
				SKUList: []sheinpub.SKUDraft{{
					SupplierSKU: "SKU-1",
					BasePrice:   "12.34",
					SitePriceList: []sheinpub.SitePrice{{
						BasePrice: "12.34",
					}},
				}},
			}},
		},
		PreviewPayload: &sheinproduct.Product{
			ImageInfo: &sheinproduct.ImageInfo{
				ImageInfoList: []sheinproduct.ImageDetail{{ImageURL: "https://img.example/main.jpg"}},
			},
		},
		Pricing: &sheinpub.PricingReview{Ready: true},
		FinalSubmissionDraft: &sheinpub.FinalDraft{
			Confirmed:          true,
			MainImageURL:       "https://img.example/main.jpg",
			ImageRoleOverrides: map[string]string{"https://img.example/skc.jpg": "skc"},
		},
	}

	checks := BuildSubmitPayloadReadinessChecks(pkg, "publish")

	assertReadinessCheck(t, checks, "request_draft", true)
	assertReadinessCheck(t, checks, "preview_product", true)
	assertReadinessCheck(t, checks, "images", true)
	assertReadinessCheck(t, checks, "final_images", true)
	assertReadinessCheck(t, checks, "variants", true)
	assertReadinessCheck(t, checks, "pricing", true)
	assertReadinessCheck(t, checks, "final_review", true)
}

func TestBuildSubmitPayloadReadinessChecksIncludesReviewAndFactsChecks(t *testing.T) {
	t.Parallel()

	pkg := &sheinpub.Package{
		ReviewNotes: []string{"需要人工复核"},
		Metadata: map[string]string{
			"source_platform":             "1688",
			"source_fact_review_required": "true",
		},
	}

	checks := BuildSubmitPayloadReadinessChecks(pkg, "publish")

	assertReadinessCheck(t, checks, "manual_notes", false)
	assertReadinessCheck(t, checks, "source_facts", false)
}

func assertReadinessCheck(t *testing.T, checks []ReadinessCheckSpec, key string, ok bool) {
	t.Helper()
	for _, check := range checks {
		if check.Key != key {
			continue
		}
		if check.OK != ok {
			t.Fatalf("check %q OK = %v, want %v; check=%+v", key, check.OK, ok, check)
		}
		return
	}
	t.Fatalf("missing readiness check %q in %+v", key, checks)
}
