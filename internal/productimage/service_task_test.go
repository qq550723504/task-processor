package productimage

import (
	"context"
	"testing"
)

func TestValidateRequestAcceptsNonAmazonMarketplace(t *testing.T) {
	t.Parallel()

	svc := &service{}
	req := &ImageProcessRequest{
		ProductURL:  "https://detail.1688.com/offer/123.html",
		Marketplace: "shein",
	}

	if err := svc.validateRequest(req); err != nil {
		t.Fatalf("validateRequest returned error for shein marketplace: %v", err)
	}
}

func TestValidateRequestRequiresMarketplace(t *testing.T) {
	t.Parallel()

	svc := &service{}
	req := &ImageProcessRequest{
		ProductURL: "https://detail.1688.com/offer/123.html",
	}

	if err := svc.validateRequest(req); err == nil {
		t.Fatal("expected missing marketplace to be rejected")
	}
}

func TestCreateProcessTaskPersistsAIIdentity(t *testing.T) {
	repo := &contextAwareTaskRepo{}
	svc := &service{taskRepo: repo, requireAIIdentity: true}
	ctx := WithAIIdentity(context.Background(), AIIdentity{TenantID: "tenant-a", UserID: "user-a", TraceID: "trace-a"})

	task, err := svc.CreateProcessTask(ctx, &ImageProcessRequest{
		ProductURL:  "https://example.test/product",
		Marketplace: "shein",
	})
	if err != nil {
		t.Fatalf("CreateProcessTask: %v", err)
	}
	if task.TenantID != "tenant-a" || task.UserID != "user-a" {
		t.Fatalf("task identity = %q/%q", task.TenantID, task.UserID)
	}
	if repo.task.TenantID != "tenant-a" || repo.task.UserID != "user-a" {
		t.Fatalf("persisted identity = %q/%q", repo.task.TenantID, repo.task.UserID)
	}
}

func TestWithTaskIdentityCarriesTaskIDAndExistingTraceIntoWorkerContext(t *testing.T) {
	task := &Task{ID: "task-a", TenantID: "tenant-a", UserID: "user-a"}
	ctx := WithAIIdentity(context.Background(), AIIdentity{TraceID: "trace-a"})
	identity := AIIdentityFromContext(WithTaskIdentity(ctx, task))
	if identity.TenantID != "tenant-a" || identity.UserID != "user-a" || identity.BusinessTaskID != "task-a" || identity.TraceID != "trace-a" {
		t.Fatalf("worker identity = %+v", identity)
	}
}

func TestCreateProcessTaskRejectsMissingAIIdentityWhenRequired(t *testing.T) {
	svc := &service{taskRepo: &contextAwareTaskRepo{}, requireAIIdentity: true}
	_, err := svc.CreateProcessTask(context.Background(), &ImageProcessRequest{
		ProductURL:  "https://example.test/product",
		Marketplace: "shein",
	})
	if err == nil {
		t.Fatal("CreateProcessTask error = nil, want missing identity rejection")
	}
}

func TestCreateProcessTaskAllowsMissingAIIdentityForLegacyCaller(t *testing.T) {
	repo := &contextAwareTaskRepo{}
	svc := &service{taskRepo: repo, requireAIIdentity: false}

	task, err := svc.CreateProcessTask(context.Background(), &ImageProcessRequest{
		ProductURL:  "https://example.test/product",
		Marketplace: "amazon",
	})
	if err != nil {
		t.Fatalf("CreateProcessTask: %v", err)
	}
	if task.TenantID != "" || task.UserID != "" {
		t.Fatalf("task identity = %q/%q, want empty legacy identity", task.TenantID, task.UserID)
	}
}
