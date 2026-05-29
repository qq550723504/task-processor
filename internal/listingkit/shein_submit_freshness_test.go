package listingkit

import (
	"strings"
	"testing"

	sheinpub "task-processor/internal/publishing/shein"
)

func TestEvaluateSheinCategoryFreshnessDetectsDrift(t *testing.T) {
	t.Parallel()

	current := &SheinPackage{
		CategoryID:     3001,
		ProductTypeID:  intPtr(9001),
		CategoryIDList: []int{1, 2, 3001},
	}
	fresh := &sheinpub.CategoryResolution{
		Status:        "resolved",
		CategoryID:    3002,
		ProductTypeID: 9002,
	}

	ok, message := evaluateSheinCategoryFreshness(current, fresh)
	if ok {
		t.Fatal("expected category freshness drift to block")
	}
	if message == "" {
		t.Fatal("expected drift message")
	}
}

func TestEvaluateSheinAttributeFreshnessDetectsTemplateMismatch(t *testing.T) {
	t.Parallel()

	valueID := 11
	current := &SheinPackage{
		ResolvedAttributes: []sheinpub.ResolvedAttribute{{
			Name:             "Material",
			Value:            "Cotton",
			AttributeID:      101,
			AttributeValueID: &valueID,
		}},
	}
	fresh := &sheinpub.AttributeResolution{
		Status: "resolved",
		ResolvedAttributes: []sheinpub.ResolvedAttribute{{
			Name:        "Material",
			Value:       "Cotton",
			AttributeID: 999,
		}},
	}

	ok, message := evaluateSheinAttributeFreshness(current, fresh)
	if ok {
		t.Fatal("expected attribute freshness drift to block")
	}
	if message == "" {
		t.Fatal("expected drift message")
	}
}

func TestEvaluateSheinAttributeFreshnessReportsValueIDDifferenceDetails(t *testing.T) {
	t.Parallel()

	currentValueID := 1002592
	freshValueID := 1000145
	current := &SheinPackage{
		ResolvedAttributes: []sheinpub.ResolvedAttribute{{
			Name:             "Material",
			Value:            "PU",
			AttributeID:      160,
			AttributeValueID: &currentValueID,
		}},
	}
	fresh := &sheinpub.AttributeResolution{
		Status: "resolved",
		ResolvedAttributes: []sheinpub.ResolvedAttribute{{
			Name:             "Material",
			Value:            "PU",
			AttributeID:      160,
			AttributeValueID: &freshValueID,
		}},
	}

	ok, message := evaluateSheinAttributeFreshness(current, fresh)
	if ok {
		t.Fatal("expected attribute freshness drift to block")
	}
	if message == "" {
		t.Fatal("expected drift message")
	}
	if got := message; !containsAll(got,
		"attribute_value_id=1002592",
		"attribute_value_id=1000145",
		"Material",
		"PU",
	) {
		t.Fatalf("drift message = %q, want current/fresh attribute diff details", got)
	}
}

func TestEvaluateSheinSaleAttributeFreshnessDetectsTemplateMismatch(t *testing.T) {
	t.Parallel()

	valueID := 27
	current := &SheinPackage{
		SaleAttributeResolution: &sheinpub.SaleAttributeResolution{
			Status:             "resolved",
			PrimaryAttributeID: 1001,
			SKCAttributes: []sheinpub.ResolvedSaleAttribute{{
				Scope:            "skc",
				AttributeID:      1001,
				AttributeValueID: &valueID,
				Value:            "Black",
			}},
		},
	}
	fresh := &sheinpub.SaleAttributeResolution{
		Status:             "resolved",
		PrimaryAttributeID: 1001,
		SKCAttributes: []sheinpub.ResolvedSaleAttribute{{
			Scope:       "skc",
			AttributeID: 1001,
			Value:       "White",
		}},
	}

	ok, message := evaluateSheinSaleAttributeFreshness(current, fresh)
	if ok {
		t.Fatal("expected sale attribute freshness drift to block")
	}
	if message == "" {
		t.Fatal("expected drift message")
	}
}

func containsAll(haystack string, needles ...string) bool {
	for _, needle := range needles {
		if !strings.Contains(haystack, needle) {
			return false
		}
	}
	return true
}
