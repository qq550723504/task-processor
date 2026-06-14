package workspace

import (
	"testing"

	sheinpub "task-processor/internal/publishing/shein"
)

func TestInspectionReviewReasons(t *testing.T) {
	t.Parallel()

	t.Run("prefers inspection summary", func(t *testing.T) {
		t.Parallel()

		pkg := &sheinpub.Package{
			ReviewNotes: []string{"fallback review"},
			Inspection: &sheinpub.Inspection{
				NeedsReview: true,
				Summary:     []string{" inspection review ", "inspection review"},
			},
		}

		got := InspectionReviewReasons(pkg)

		if len(got) != 1 || got[0] != "inspection review" {
			t.Fatalf("InspectionReviewReasons() = %#v, want trimmed inspection summary", got)
		}
	})

	t.Run("falls back to package review notes", func(t *testing.T) {
		t.Parallel()

		pkg := &sheinpub.Package{
			ReviewNotes: []string{" package review "},
			Inspection: &sheinpub.Inspection{
				NeedsReview: true,
			},
		}

		got := InspectionReviewReasons(pkg)

		if len(got) != 1 || got[0] != "package review" {
			t.Fatalf("InspectionReviewReasons() = %#v, want package review notes fallback", got)
		}
	})

	t.Run("uses default message when review reasons missing", func(t *testing.T) {
		t.Parallel()

		pkg := &sheinpub.Package{
			Inspection: &sheinpub.Inspection{
				NeedsReview: true,
			},
		}

		got := InspectionReviewReasons(pkg)

		if len(got) != 1 || got[0] != defaultInspectionReviewReason {
			t.Fatalf("InspectionReviewReasons() = %#v, want default review reason", got)
		}
	})
}

func TestCookieUnavailableReviewNotes(t *testing.T) {
	t.Parallel()

	pkg := &sheinpub.Package{
		ReviewNotes: []string{"  SHEIN 店铺 cookie 不可用，需要重新登录  ", "other review"},
		CategoryResolution: &sheinpub.CategoryResolution{
			ReviewNotes: []string{"store cookies are unavailable", "store cookies are unavailable"},
		},
		AttributeResolution: &sheinpub.AttributeResolution{
			ReviewNotes: []string{"attribute review"},
		},
		SaleAttributeResolution: &sheinpub.SaleAttributeResolution{
			ReviewNotes: []string{"cookies are unavailable"},
		},
	}

	got := CookieUnavailableReviewNotes(pkg)

	want := []string{
		"SHEIN 店铺 cookie 不可用，需要重新登录",
		"store cookies are unavailable",
		"cookies are unavailable",
	}
	if len(got) != len(want) {
		t.Fatalf("CookieUnavailableReviewNotes() len = %d, want %d (%#v)", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("CookieUnavailableReviewNotes()[%d] = %q, want %q; all=%#v", i, got[i], want[i], got)
		}
	}
}

func TestStripCookieUnavailableReviewNotes(t *testing.T) {
	t.Parallel()

	pkg := &sheinpub.Package{
		ReviewNotes: []string{"keep", "SHEIN 店铺 cookie 不可用"},
		CategoryResolution: &sheinpub.CategoryResolution{
			ReviewNotes: []string{"store cookies are unavailable", "keep category"},
		},
		AttributeResolution: &sheinpub.AttributeResolution{
			ReviewNotes: []string{"keep attribute"},
		},
		SaleAttributeResolution: &sheinpub.SaleAttributeResolution{
			ReviewNotes: []string{"cookies are unavailable"},
		},
	}

	StripCookieUnavailableReviewNotes(pkg)

	if len(pkg.ReviewNotes) != 1 || pkg.ReviewNotes[0] != "keep" {
		t.Fatalf("pkg.ReviewNotes = %#v, want cookie notes stripped", pkg.ReviewNotes)
	}
	if len(pkg.CategoryResolution.ReviewNotes) != 1 || pkg.CategoryResolution.ReviewNotes[0] != "keep category" {
		t.Fatalf("category notes = %#v, want cookie notes stripped", pkg.CategoryResolution.ReviewNotes)
	}
	if len(pkg.AttributeResolution.ReviewNotes) != 1 || pkg.AttributeResolution.ReviewNotes[0] != "keep attribute" {
		t.Fatalf("attribute notes = %#v, want non-cookie note preserved", pkg.AttributeResolution.ReviewNotes)
	}
	if pkg.SaleAttributeResolution.ReviewNotes != nil {
		t.Fatalf("sale attribute notes = %#v, want nil after stripping all cookie notes", pkg.SaleAttributeResolution.ReviewNotes)
	}
}
