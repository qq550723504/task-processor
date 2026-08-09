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
	ctx := WithAIIdentity(context.Background(), AIIdentity{TenantID: "tenant-a", UserID: "user-a"})

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
