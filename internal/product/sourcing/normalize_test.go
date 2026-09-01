package sourcing

import (
	"errors"
	"reflect"
	"testing"
	"time"
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
}
