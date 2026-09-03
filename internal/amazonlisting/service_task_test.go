package amazonlisting

import (
	"context"
	"testing"
	"time"

	"task-processor/internal/product/catalog"
	"task-processor/internal/shared/aiidentity"
)

type versionedTaskSnapshotReader struct{ version uint64 }

func (r versionedTaskSnapshotReader) GetProductSnapshot(context.Context, ProductSnapshotQuery) (catalog.ProductSnapshot, error) {
	return catalog.ProductSnapshot{Title: "Pinned product"}, nil
}

func (r versionedTaskSnapshotReader) GetPublishedProductSnapshot(context.Context, ProductSnapshotQuery) (catalog.PublishedSnapshot, error) {
	return catalog.PublishedSnapshot{Version: r.version, Snapshot: catalog.ProductSnapshot{Title: "Pinned product"}}, nil
}

func TestCreateGenerateTaskRequiresProductKey(t *testing.T) {
	repo := &stubRepository{}
	svc, err := NewService(&ServiceConfig{Repository: repo})
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}
	ctx := aiidentity.WithIdentity(context.Background(), aiidentity.Identity{TenantID: "tenant-a", UserID: "user-a"})
	_, err = svc.CreateGenerateTask(ctx, &GenerateRequest{Marketplace: "amazon"})
	if err == nil || repo.task != nil {
		t.Fatalf("CreateGenerateTask() error = %v persisted = %+v, want product_key rejection", err, repo.task)
	}
}

func TestCreateGenerateTaskAcceptsProductKey(t *testing.T) {
	repo := &stubRepository{}
	svc, err := NewService(&ServiceConfig{Repository: repo})
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}
	ctx := aiidentity.WithIdentity(context.Background(), aiidentity.Identity{TenantID: "tenant-a", UserID: "user-a"})
	task, err := svc.CreateGenerateTask(ctx, &GenerateRequest{Marketplace: "amazon", ProductKey: " product-1 "})
	if err != nil {
		t.Fatalf("CreateGenerateTask() error = %v", err)
	}
	if task.Request.ProductKey != "product-1" || repo.task == nil {
		t.Fatalf("created task = %+v persisted = %+v", task, repo.task)
	}
}

func TestCreateGenerateTaskPinsCurrentCatalogSnapshotVersion(t *testing.T) {
	repo := &stubRepository{}
	svc, err := NewService(&ServiceConfig{Repository: repo, ProductSnapshotReader: versionedTaskSnapshotReader{version: 7}})
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}
	ctx := aiidentity.WithIdentity(context.Background(), aiidentity.Identity{TenantID: "tenant-a", UserID: "user-a"})
	task, err := svc.CreateGenerateTask(ctx, &GenerateRequest{Marketplace: "amazon", ProductKey: "product-1"})
	if err != nil {
		t.Fatalf("CreateGenerateTask() error = %v", err)
	}
	if task.SourceSnapshotVersion != 7 || repo.task.SourceSnapshotVersion != 7 {
		t.Fatalf("source snapshot version = task:%d persisted:%d, want 7", task.SourceSnapshotVersion, repo.task.SourceSnapshotVersion)
	}
}

func TestReviewTaskApplyEditsChangesDraftWithoutMutatingSnapshotState(t *testing.T) {
	repo := &stubRepository{task: &Task{
		ID:        "task-1",
		Status:    TaskStatusNeedsReview,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
		Request:   &GenerateRequest{Marketplace: "amazon", ProductKey: "product-1"},
		Result: &AmazonListingDraft{
			TaskID:      "task-1",
			Title:       "Old title",
			Description: "Long enough product description for validation and marketplace review.",
			Images:      &AmazonImageBundle{MainImage: "https://cdn.example.com/main.jpg"},
			Pricing:     &AmazonPricingDraft{Currency: "USD"},
			ReviewItems: []AmazonReviewItem{{Field: "title", Reason: "review title", NeedsHuman: true}},
		},
	}}
	svc, err := NewService(&ServiceConfig{Repository: repo})
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}
	result, err := svc.ReviewTask(context.Background(), "task-1", &ReviewTaskRequest{
		Action: "apply_edits",
		Edits:  []DraftFieldEdit{{Field: "title", StringValue: "Updated marketplace title"}},
	})
	if err != nil {
		t.Fatalf("ReviewTask() error = %v", err)
	}
	if result.Result.Title != "Updated marketplace title" {
		t.Fatalf("title = %q", result.Result.Title)
	}
	for _, item := range result.Result.ReviewItems {
		if item.Field == "title" {
			t.Fatal("resolved title review item was retained")
		}
	}
}
