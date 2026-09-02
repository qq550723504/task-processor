package sourcing

import (
	"errors"
	"reflect"
	"testing"
	"time"

	"task-processor/internal/product/catalog"
)

func TestNormalizePreservesEvidenceLineageAndWarnings(t *testing.T) {
	capturedAt := time.Date(2026, time.September, 1, 12, 30, 0, 0, time.UTC)
	in := SourceEnvelope{
		Identity: SourceIdentity{
			SourceType:     " AMAZON ",
			SourcePlatform: " Amazon ",
			SourceID:       " B001 ",
			SourceVersion:  " version-1 ",
		},
		RawReference: RawSourceReference{
			ReferenceType: "amazon_product",
			ReferenceID:   "raw-1",
			URL:           "https://www.amazon.com/dp/B001",
			SnapshotID:    "snapshot-1",
			Checksum:      "sha256:abc",
			CapturedAt:    capturedAt,
			Metadata:      map[string]string{"etag": "v1", "collector": "crawler"},
		},
		Warnings: []SourceWarning{{Code: " Missing_Title ", Field: " title ", Message: " missing "}},
		Trace:    SourceTrace{SourceRunID: "run-1", RequestID: "request-1", Notes: []string{"crawler evidence"}},
	}
	wantRawReference := in.RawReference
	wantTrace := in.Trace

	out, err := Normalize(in)
	if err != nil {
		t.Fatalf("Normalize() error = %v", err)
	}
	if out.Identity.SourceType != "amazon" || out.Identity.SourcePlatform != "amazon" || out.Identity.SourceID != "B001" {
		t.Fatalf("normalized identity = %+v, want normalized source fields", out.Identity)
	}
	if out.Warnings[0] != (SourceWarning{Code: "missing_title", Field: "title", Message: "missing"}) {
		t.Fatalf("normalized warning = %+v, want normalized warning metadata", out.Warnings[0])
	}
	if !reflect.DeepEqual(out.RawReference, wantRawReference) {
		t.Fatalf("RawReference = %+v, want preserved evidence %+v", out.RawReference, wantRawReference)
	}
	if !reflect.DeepEqual(out.Trace, wantTrace) {
		t.Fatalf("Trace = %+v, want preserved lineage %+v", out.Trace, wantTrace)
	}
	if in.Warnings[0].Code != " Missing_Title " {
		t.Fatalf("Normalize() mutated input warning code to %q", in.Warnings[0].Code)
	}
	out.RawReference.Metadata["etag"] = "mutated"
	if in.RawReference.Metadata["etag"] != "v1" {
		t.Fatalf("Normalize() returned aliased source metadata: input etag = %q", in.RawReference.Metadata["etag"])
	}
}

func TestNormalizeRejectsIdentityWithoutPlatform(t *testing.T) {
	_, err := Normalize(SourceEnvelope{Identity: SourceIdentity{SourceType: "crawler", SourceID: "B001"}})
	if !errors.Is(err, ErrSourceIdentityRequired) {
		t.Fatalf("Normalize() error = %v, want ErrSourceIdentityRequired", err)
	}
}

func TestToSnapshotProducesCanonicalFactsWithoutAssets(t *testing.T) {
	in := SourceEnvelope{
		Identity: SourceIdentity{
			SourceType:     " CRAWLER ",
			SourcePlatform: " AMAZON ",
			SourceID:       " B001 ",
		},
		RawReference: RawSourceReference{ReferenceID: "raw-1"},
		ProductCandidate: ProductCandidate{
			Title:       "Test Shirt",
			Brand:       "Test Brand",
			Description: "Test description",
			Attributes:  map[string]string{"zeta": "last", "category": "Shirts", "alpha": "first"},
			Variants: []ProductVariantCandidate{{
				SourceID:   "B001-BLUE-M",
				Title:      "Blue / M",
				SKU:        "SKU-1",
				Attributes: map[string]string{"Size": "M", "Color": "Blue"},
			}},
		},
		AssetCandidates: []AssetCandidate{{URL: "https://img.example/1.jpg", Role: "primary"}},
	}

	got, err := ToSnapshot(in)
	if err != nil {
		t.Fatalf("ToSnapshot() error = %v", err)
	}
	if got.Title != "Test Shirt" || got.Brand != "Test Brand" || got.Description != "Test description" {
		t.Fatalf("ToSnapshot() facts = %+v, want title, brand, and description", got)
	}
	if len(got.Sources) != 1 || got.Sources[0].Type != "crawler" || got.Sources[0].Detail != "raw-1" {
		t.Fatalf("ToSnapshot() sources = %+v, want normalized source evidence", got.Sources)
	}
	if len(got.Images) != 0 {
		t.Fatalf("ToSnapshot() images = %+v, want asset candidates to remain in the envelope", got.Images)
	}
	if len(got.Attributes) != 3 || got.Attributes[0].Name != "alpha" || got.Attributes[1].Name != "category" || got.Attributes[2].Name != "zeta" {
		t.Fatalf("ToSnapshot() attributes = %+v, want deterministic candidate attributes", got.Attributes)
	}
	if len(got.Variants) != 1 || got.Variants[0].SKU != "SKU-1" {
		t.Fatalf("ToSnapshot() variants = %+v, want candidate variant", got.Variants)
	}
	if got.Variants[0].SourceID != "B001-BLUE-M" || got.Variants[0].Title != "Blue / M" {
		t.Fatalf("ToSnapshot() variant identity = %+v, want source ID and title", got.Variants[0])
	}
	if len(got.Variants[0].Attributes) != 2 || got.Variants[0].Attributes[0].Name != "Color" || got.Variants[0].Attributes[1].Name != "Size" {
		t.Fatalf("ToSnapshot() variant attributes = %+v, want deterministic attributes", got.Variants[0].Attributes)
	}
}

func TestToSnapshotPreservesStructuredWarningsAndRawEvidence(t *testing.T) {
	capturedAt := time.Date(2026, time.September, 2, 8, 0, 0, 0, time.UTC)
	got, err := ToSnapshot(SourceEnvelope{
		Identity: SourceIdentity{SourceType: "crawler", SourcePlatform: "amazon", SourceID: "B001", SourceVersion: "v1"},
		RawReference: RawSourceReference{
			ReferenceType: "amazon_product", ReferenceID: "raw-1", URL: "https://example.test/B001",
			SnapshotID: "snapshot-1", Checksum: "sha256:abc", CapturedAt: capturedAt,
			Metadata: map[string]string{"etag": "one"},
		},
		Trace:            SourceTrace{SourceRunID: "source-run-1", RequestID: "request-1", Notes: []string{"crawler evidence"}},
		Warnings:         []SourceWarning{{Code: "missing_brand", Field: "brand", Message: "brand unavailable"}},
		ProductCandidate: ProductCandidate{Title: "Bottle"},
	})
	if err != nil {
		t.Fatalf("ToSnapshot() error = %v", err)
	}
	if got.Review == nil || !got.Review.NeedsReview || !reflect.DeepEqual(got.Review.Reasons, []string{"brand unavailable"}) {
		t.Fatalf("snapshot review = %+v, want preserved source warning", got.Review)
	}
	if !reflect.DeepEqual(got.Warnings, []catalog.Warning{{Code: "missing_brand", Field: "brand", Message: "brand unavailable"}}) {
		t.Fatalf("snapshot warnings = %+v, want structured source warning", got.Warnings)
	}
	if len(got.Sources) != 1 {
		t.Fatalf("snapshot sources = %+v, want one structured source", got.Sources)
	}
	source := got.Sources[0]
	if source.Type != "crawler" || source.Detail != "raw-1" || source.Platform != "amazon" || source.SourceID != "B001" || source.SourceVersion != "v1" {
		t.Fatalf("snapshot source identity = %+v", source)
	}
	if source.ReferenceType != "amazon_product" || source.URL != "https://example.test/B001" || source.SnapshotID != "snapshot-1" || source.Checksum != "sha256:abc" || !source.CapturedAt.Equal(capturedAt) {
		t.Fatalf("snapshot raw evidence = %+v", source)
	}
	if source.Metadata["etag"] != "one" || source.SourceRunID != "source-run-1" || source.RequestID != "request-1" || !reflect.DeepEqual(source.Notes, []string{"crawler evidence"}) {
		t.Fatalf("snapshot lineage = %+v", source)
	}
}
