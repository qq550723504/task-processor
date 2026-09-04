package catalog

import (
	"errors"
	"reflect"
	"testing"

	"task-processor/internal/product/catalog/canonical"
)

func TestNormalizeProducesDeterministicSnapshot(t *testing.T) {
	t.Parallel()

	input := &canonical.Product{
		Title:       "Bottle",
		Description: "Steel bottle",
		FieldTraces: map[string]canonical.FieldTrace{
			"title": {
				Sources: []canonical.Source{{Type: canonical.SourceUserText, Detail: "title evidence"}},
			},
			"description": {
				Sources: []canonical.Source{{Type: canonical.SourceDerived, Detail: "description evidence"}},
			},
		},
		Attributes: map[string]canonical.Attribute{
			"material": {
				Value: "steel",
				Trace: canonical.FieldTrace{
					NeedsReview: true,
					Sources:     []canonical.Source{{Type: canonical.SourceUserImage, Detail: "material evidence"}},
				},
			},
			"color": {
				Value: "black",
				Trace: canonical.FieldTrace{
					NeedsReview: true,
					Sources:     []canonical.Source{{Type: canonical.SourceUserImage, Detail: "color evidence"}},
				},
			},
		},
	}

	first, err := Normalize(input)
	if err != nil {
		t.Fatalf("first Normalize: %v", err)
	}
	second, err := Normalize(input)
	if err != nil {
		t.Fatalf("second Normalize: %v", err)
	}
	if !reflect.DeepEqual(first, second) {
		t.Fatalf("Normalize results differ:\nfirst:  %+v\nsecond: %+v", first, second)
	}
	if got, want := attributeNames(first.Attributes), []string{"color", "material"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("attribute names = %v, want %v", got, want)
	}
	if got, want := first.Review.Reasons, []string{"属性待确认: color", "属性待确认: material"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("review reasons = %v, want %v", got, want)
	}
	wantSources := []SourceRecord{
		{Type: "derived", Detail: "description evidence"},
		{Type: "user_image", Detail: "color evidence"},
		{Type: "user_image", Detail: "material evidence"},
		{Type: "user_text", Detail: "title evidence"},
	}
	if !reflect.DeepEqual(first.Sources, wantSources) {
		t.Fatalf("sources = %+v, want %+v", first.Sources, wantSources)
	}
}

func TestNormalizeRejectsNilWithStableError(t *testing.T) {
	t.Parallel()

	snapshot, err := Normalize(nil)
	if snapshot != nil {
		t.Fatalf("snapshot = %+v, want nil", snapshot)
	}
	if !errors.Is(err, ErrInvalidSnapshot) {
		t.Fatalf("error = %v, want ErrInvalidSnapshot", err)
	}
}

func TestNormalizeBuildsCatalogSnapshot(t *testing.T) {
	t.Parallel()

	snapshot, err := Normalize(&canonical.Product{
		Title:         "Wireless Earbuds",
		Brand:         "Acme",
		CategoryPath:  []string{"Electronics", "Audio"},
		Description:   "ANC earbuds",
		SellingPoints: []string{"ANC", "Bluetooth"},
		SEOKeywords:   []string{"earbuds"},
		FieldTraces: map[string]canonical.FieldTrace{
			"title": {
				Sources: []canonical.Source{{Type: canonical.SourceUserText, Detail: "user text"}},
			},
		},
		Attributes: map[string]canonical.Attribute{
			"color": {
				Value: "Black",
				Trace: canonical.FieldTrace{
					NeedsReview: true,
					Sources: []canonical.Source{{
						Type:   canonical.SourceUserImage,
						Detail: "uploaded image",
					}},
				},
			},
		},
		Images: []canonical.Image{{
			URL:  "https://example.com/1.jpg",
			Role: "primary",
		}},
		Variants: []canonical.Variant{{
			SKU: "SKU-1",
			Attributes: map[string]canonical.Attribute{
				"size": {Value: "M"},
			},
			Price: &canonical.PriceInfo{
				Currency: "USD",
				Amount:   29.9,
			},
		}},
	})
	if err != nil {
		t.Fatalf("Normalize: %v", err)
	}
	if snapshot == nil {
		t.Fatal("expected product snapshot")
	}
	if snapshot.Title != "Wireless Earbuds" {
		t.Fatalf("title = %q", snapshot.Title)
	}
	if len(snapshot.Attributes) != 1 || snapshot.Attributes[0].Name != "color" {
		t.Fatalf("attributes = %+v", snapshot.Attributes)
	}
	if len(snapshot.Variants) != 1 || snapshot.Variants[0].SKU != "SKU-1" {
		t.Fatalf("variants = %+v", snapshot.Variants)
	}
	if snapshot.Review == nil || !snapshot.Review.NeedsReview {
		t.Fatalf("review = %+v, want needs review", snapshot.Review)
	}
	if len(snapshot.Sources) == 0 {
		t.Fatalf("sources = %+v, want collected sources", snapshot.Sources)
	}
}

func attributeNames(attributes []Attribute) []string {
	names := make([]string, 0, len(attributes))
	for _, attribute := range attributes {
		names = append(names, attribute.Name)
	}
	return names
}
