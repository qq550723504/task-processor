package workspace

import (
	"testing"

	sheinpub "task-processor/internal/publishing/shein"
)

func TestBuildCategoryRecommendationMeta(t *testing.T) {
	productTypeID := 10
	meta := BuildCategoryRecommendationMeta(&sheinpub.Package{
		CategoryID:    123,
		ProductTypeID: &productTypeID,
		CategoryResolution: &sheinpub.CategoryResolution{
			Status:      "resolved",
			Source:      "suggest_category",
			ReviewNotes: []string{"check category"},
		},
	})

	if meta == nil || meta.Confidence != "high" || meta.Source != "suggest_category" {
		t.Fatalf("meta = %#v", meta)
	}
}

func TestBuildAttributeSuggestions(t *testing.T) {
	valueID := 20
	suggestions := BuildAttributeSuggestions(&sheinpub.Package{
		AttributeResolution: &sheinpub.AttributeResolution{Source: "template"},
		ResolvedAttributes: []sheinpub.ResolvedAttribute{{
			Name:             "Material",
			Value:            "Cotton",
			AttributeID:      100,
			AttributeValueID: &valueID,
		}},
	})

	if len(suggestions) != 1 || suggestions[0].Confidence != "high" {
		t.Fatalf("suggestions = %#v", suggestions)
	}
}

func TestBuildSaleCandidateSuggestions(t *testing.T) {
	suggestions := BuildSaleCandidateSuggestions(&sheinpub.Package{
		SaleAttributeResolution: &sheinpub.SaleAttributeResolution{
			Source: "sale_attribute_templates",
			Candidates: []sheinpub.SaleAttributeCandidateInfo{{
				Name:          "Color",
				AttributeID:   1,
				SelectedScope: "skc",
				PrimaryScore:  9,
			}},
		},
	})

	if len(suggestions) != 1 || suggestions[0].Confidence != "high" {
		t.Fatalf("suggestions = %#v", suggestions)
	}
}
