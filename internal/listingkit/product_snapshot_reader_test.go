package listingkit

import (
	"context"
	"errors"
	"testing"

	productasset "task-processor/internal/product/asset"
	"task-processor/internal/product/catalog"
)

type recordingProductSnapshotReader struct {
	snapshot catalog.ProductSnapshot
	err      error
	calls    []ProductSnapshotQuery
}

func (r *recordingProductSnapshotReader) GetProductSnapshot(_ context.Context, query ProductSnapshotQuery) (catalog.ProductSnapshot, error) {
	r.calls = append(r.calls, query)
	return r.snapshot, r.err
}

func TestCanonicalPhaseReadsProductSnapshotOnce(t *testing.T) {
	reader := &recordingProductSnapshotReader{snapshot: catalog.ProductSnapshot{
		Title:        "Bottle",
		CategoryPath: []string{"Outdoors", "Bottles"},
	}}
	phase := standardWorkflowCanonicalPhase{snapshots: reader}

	got, err := phase.run(context.Background(), ProductSnapshotQuery{
		TenantID:   "tenant-a",
		ProductKey: "product-1",
	})
	if err != nil {
		t.Fatalf("run() error = %v", err)
	}
	if got.Title != "Bottle" || len(got.CategoryPath) != 2 || got.CategoryPath[1] != "Bottles" {
		t.Fatalf("run() snapshot = %+v, want reader snapshot", got)
	}
	if len(reader.calls) != 1 {
		t.Fatalf("GetProductSnapshot calls = %d, want 1", len(reader.calls))
	}
	if gotQuery := reader.calls[0]; gotQuery.TenantID != "tenant-a" || gotQuery.ProductKey != "product-1" {
		t.Fatalf("GetProductSnapshot query = %+v, want tenant-a/product-1", gotQuery)
	}
	got.CategoryPath[0] = "mutated"
	if reader.snapshot.CategoryPath[0] != "Outdoors" {
		t.Fatalf("reader snapshot was mutated through returned value: %+v", reader.snapshot)
	}
}

func TestProductSnapshotQueryForTaskUsesPinnedPublishedVersion(t *testing.T) {
	query := productSnapshotQueryForTask(&Task{
		TenantID: "tenant-a", SourceSnapshotVersion: 17,
		Request: &GenerateRequest{ProductKey: "product-1"},
	})
	if query.TenantID != "tenant-a" || query.ProductKey != "product-1" || query.Version != 17 {
		t.Fatalf("query = %+v, want tenant/product/version 17", query)
	}
}

func TestPrepareGenerateTaskPersistsPinnedPublishedVersion(t *testing.T) {
	_, task, err := (&taskLifecycleService{}).prepareGenerateTask(context.Background(), &GenerateRequest{
		TenantID: "tenant-a", UserID: "user-a", ProductKey: "product-1", Platforms: []string{"amazon"}, SourceSnapshotVersion: 17,
	})
	if err != nil {
		t.Fatalf("prepareGenerateTask() error = %v", err)
	}
	if task == nil || task.SourceSnapshotVersion != 17 {
		t.Fatalf("task source snapshot version = %v, want 17", task)
	}
}

func TestCanonicalPhaseTreatsEmptyProductSnapshotAsNotReady(t *testing.T) {
	phase := standardWorkflowCanonicalPhase{snapshots: &recordingProductSnapshotReader{}}

	_, err := phase.run(context.Background(), ProductSnapshotQuery{TenantID: "tenant-a", ProductKey: "product-1"})
	if !errors.Is(err, ErrProductSnapshotNotReady) {
		t.Fatalf("run() error = %v, want ErrProductSnapshotNotReady", err)
	}
}

func TestGenerateRequestRequiresExplicitProductKey(t *testing.T) {
	if err := validateRequest(&GenerateRequest{Platforms: []string{"shein"}}); err == nil {
		t.Fatal("validateRequest() error = nil, want product_key requirement")
	}
	if err := validateRequest(&GenerateRequest{ProductKey: "product-1", Platforms: []string{"shein"}}); err != nil {
		t.Fatalf("validateRequest() error = %v, want product_key-only request accepted", err)
	}
}

func TestStandardWorkflowMarksProductSnapshotNotReadyForReview(t *testing.T) {
	reader := &recordingProductSnapshotReader{err: ErrProductSnapshotNotReady}
	svc := &service{workflowDeps: workflowDependencies{productSnapshots: reader}}
	task := &Task{
		ID:       "listing-task-1",
		TenantID: "tenant-a",
		Request: &GenerateRequest{
			ProductKey: "product-1",
		},
	}

	state, err := svc.runStandardProductWorkflow(context.Background(), task)
	if err != nil {
		t.Fatalf("runStandardProductWorkflow() error = %v, want review result", err)
	}
	if state == nil || state.result == nil || state.result.Summary == nil {
		t.Fatalf("workflow state = %+v, want result with summary", state)
	}
	if !state.result.Summary.NeedsReview || state.result.Summary.BlockingCount != 1 {
		t.Fatalf("summary = %+v, want one blocking review issue", state.result.Summary)
	}
	if len(state.result.WorkflowIssues) != 1 || state.result.WorkflowIssues[0].Code != productSnapshotNotReadyIssueCode {
		t.Fatalf("workflow issues = %+v, want %q", state.result.WorkflowIssues, productSnapshotNotReadyIssueCode)
	}
	if len(state.result.ChildTasks) != 0 {
		t.Fatalf("child tasks = %+v, want none", state.result.ChildTasks)
	}
	if len(reader.calls) != 1 {
		t.Fatalf("GetProductSnapshot calls = %d, want 1", len(reader.calls))
	}
	if !errors.Is(reader.err, ErrProductSnapshotNotReady) {
		t.Fatalf("reader error = %v, want ErrProductSnapshotNotReady", reader.err)
	}
}

func TestCanonicalProductProjectionPreservesSnapshotFacts(t *testing.T) {
	snapshot := catalog.ProductSnapshot{
		Title: "Bottle",
		Attributes: []catalog.Attribute{{
			Name:  "material",
			Value: "steel",
			Trace: catalog.Trace{Confidence: 0.9},
		}},
		Variants: []catalog.Variant{{
			SKU:        "BOTTLE-BLUE",
			Attributes: []catalog.Attribute{{Name: "color", Value: "blue"}},
			Price:      &catalog.Price{Currency: "USD", Amount: 19.5},
		}},
		Review: &catalog.ReviewState{NeedsReview: true},
	}

	got := canonicalProductFromSnapshot(snapshot)
	if got == nil || got.Title != "Bottle" || got.Attributes["material"].Value != "steel" {
		t.Fatalf("canonical projection = %+v, want snapshot facts", got)
	}
	if len(got.Variants) != 1 || got.Variants[0].SKU != "BOTTLE-BLUE" || got.Variants[0].Price == nil || got.Variants[0].Price.Amount != 19.5 {
		t.Fatalf("canonical variants = %+v, want snapshot variant", got.Variants)
	}
	if !got.NeedsReview {
		t.Fatal("canonical projection NeedsReview = false, want true")
	}
}

func TestAssemblerConsumesProductSnapshot(t *testing.T) {
	task := &Task{TenantID: "tenant-test", ID: "listing-task-2", Request: &GenerateRequest{
		ProductKey: "product-2",
		Platforms:  []string{"amazon"},
	}}
	snapshot := &catalog.ProductSnapshot{
		Title:  "Snapshot Bottle",
		Images: []catalog.Image{{URL: "https://example.test/bottle.jpg"}},
	}

	result, err := NewAssembler(stubAmazonDraftBuilder{}).Assemble(task, snapshot, &productasset.ApprovedAssetInventory{Assets: []productasset.ApprovedAsset{{ID: "main", Role: productasset.RoleMain, URL: "https://example.test/approved-bottle.jpg"}}})
	if err != nil {
		t.Fatal(err)
	}
	if result == nil || result.CatalogProduct == nil || result.CatalogProduct.Title != "Snapshot Bottle" {
		t.Fatalf("assembler result = %+v, want catalog snapshot", result)
	}
}
