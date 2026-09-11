package canonicalinspect

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"task-processor/internal/product/catalog"
)

func TestProjectionDeepCopiesAndRemovesSourceMetadata(t *testing.T) {
	snapshot := catalog.ProductSnapshot{
		Title:      "Bottle",
		Attributes: []catalog.Attribute{{Name: "material", Value: "steel", Trace: catalog.Trace{Sources: []catalog.SourceRecord{{Metadata: map[string]string{"tenant_id": "leak"}}}}}},
		Variants:   []catalog.Variant{{SKU: "sku-1", Attributes: []catalog.Attribute{{Trace: catalog.Trace{Sources: []catalog.SourceRecord{{Metadata: map[string]string{"trace_id": "leak"}}}}}}, Images: []catalog.Image{{Trace: catalog.Trace{Sources: []catalog.SourceRecord{{Metadata: map[string]string{"user_id": "leak"}}}}}}, Trace: catalog.Trace{Sources: []catalog.SourceRecord{{Metadata: map[string]string{"roles": "leak"}}}}}},
		Images:     []catalog.Image{{Trace: catalog.Trace{Sources: []catalog.SourceRecord{{Metadata: map[string]string{"permission": "leak"}}}}}},
		Sources:    []catalog.SourceRecord{{Type: "crawler", Metadata: map[string]string{"business_task_id": "leak"}, Notes: []string{"source-note"}}},
		Review:     &catalog.ReviewState{NeedsReview: true, Reasons: []string{"missing_brand"}},
		Warnings:   []catalog.Warning{{Code: "source_warning", Message: "check source"}},
	}
	published := validPublished(9007199254740993)
	published.Snapshot = snapshot

	raw, err := Project(published)
	if err != nil {
		t.Fatalf("Project() error = %v", err)
	}
	if strings.Contains(string(raw), `"metadata"`) || strings.Contains(string(raw), `"tenant_id"`) || strings.Contains(string(raw), `"business_task_id"`) {
		t.Fatalf("projection leaked reserved metadata: %s", raw)
	}
	var output Output
	if err := json.Unmarshal(raw, &output); err != nil {
		t.Fatalf("unmarshal output: %v", err)
	}
	if output.CatalogVersion != "9007199254740993" || !output.Diagnostics.NeedsReview || len(output.Diagnostics.ReviewReasons) != 1 || len(output.Diagnostics.Warnings) != 1 {
		t.Fatalf("output = %#v", output)
	}

	snapshot.Title = "mutated"
	snapshot.Review.Reasons[0] = "mutated"
	var again Output
	if err := json.Unmarshal(raw, &again); err != nil {
		t.Fatalf("unmarshal output again: %v", err)
	}
	if again.Snapshot.Title != "Bottle" || again.Diagnostics.ReviewReasons[0] != "missing_brand" {
		t.Fatalf("projection shared mutable input: %#v", again)
	}
}

func TestProjectionEnforcesExactOutputLimitWithoutTruncation(t *testing.T) {
	published := validPublished(1)
	published.Snapshot.Description = "x"
	probe, err := Project(published)
	if err != nil {
		t.Fatalf("Project(probe): %v", err)
	}
	overhead := len(probe) - 1
	published.Snapshot.Description = strings.Repeat("x", MaxOutputBytes-overhead)
	exact, err := Project(published)
	if err != nil {
		t.Fatalf("Project(exact): %v", err)
	}
	if len(exact) != MaxOutputBytes {
		t.Fatalf("exact output size = %d, want %d", len(exact), MaxOutputBytes)
	}
	published.Snapshot.Description += "x"
	if _, err := Project(published); !errors.Is(err, ErrProjectionTooLarge) {
		t.Fatalf("Project(over) error = %v", err)
	}
}

func TestProjectionSchemaFixtureIncludesAllSnapshotShapes(t *testing.T) {
	now := time.Date(2026, 9, 5, 1, 2, 3, 0, time.UTC)
	snapshot := catalog.ProductSnapshot{
		Title: "Title", Brand: "Brand", CategoryPath: []string{"A", "B"}, Description: "Description",
		SellingPoints: []string{"point"}, SEOKeywords: []string{"keyword"},
		Attributes:     []catalog.Attribute{{Name: "name", Value: "value", Trace: catalog.Trace{Confidence: .9, IsInferred: true, NeedsReview: true}}},
		Specifications: &catalog.Specifications{Dimensions: &catalog.Dimensions{Length: 1, Width: 2, Height: 3, Unit: "cm"}, Weight: &catalog.Weight{Value: 1, Unit: "kg"}, Package: &catalog.PackageInfo{Quantity: 1}, Technical: map[string]string{"power": "10W"}},
		Variants:       []catalog.Variant{{SourceID: "source", Title: "variant", SKU: "sku", Price: &catalog.Price{Currency: "USD", Amount: 1, CompareAt: 2, CostPrice: .5, WholesaleMin: 1}, Stock: 2, Barcode: "barcode", IsDefault: true}},
		Images:         []catalog.Image{{URL: "https://img.example", Role: "main", Width: 100, Height: 100}},
		Review:         &catalog.ReviewState{NeedsReview: false, Reasons: []string{}},
		Sources:        []catalog.SourceRecord{{Type: "crawler", Detail: "detail", Platform: "1688", SourceID: "id", SourceVersion: "v1", ReferenceType: "url", URL: "https://source.example", SnapshotID: "snapshot", Checksum: "sum", CapturedAt: now, SourceRunID: "run", RequestID: "request", Notes: []string{"note"}}},
		Warnings:       []catalog.Warning{{Code: "warning", Field: "brand", Message: "message"}},
	}
	published := validPublished(1)
	published.Snapshot = snapshot
	raw, err := Project(published)
	if err != nil {
		t.Fatalf("Project(): %v", err)
	}
	validateOutputAgainstRegistry(t, raw)
}
