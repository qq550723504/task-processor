package httpbootstrap

import (
	"context"
	"testing"

	"task-processor/internal/productimage"
	sdsclient "task-processor/internal/sds/client"
)

type stubImageService struct{}

func (stubImageService) CreateProcessTask(context.Context, *productimage.ImageProcessRequest) (*productimage.Task, error) {
	return nil, nil
}

func (stubImageService) GetTaskResult(context.Context, string) (*productimage.TaskResult, error) {
	return nil, nil
}

func (stubImageService) ReviewTask(context.Context, string, *productimage.ReviewTaskRequest) (*productimage.TaskResult, error) {
	return nil, nil
}

func (stubImageService) ProcessImages(context.Context, *productimage.Task) (*productimage.ImageProcessResult, error) {
	return nil, nil
}

func (stubImageService) SetTaskSubmitter(productimage.TaskSubmitter) {}

func TestNewSyncServiceReturnsServiceWithoutAuthState(t *testing.T) {
	cfg := sdsclient.DefaultConfig()
	cfg.AuthFile = t.TempDir() + "/missing-auth.json"
	cfg.CookieFile = t.TempDir() + "/missing-cookie.json"
	cfg.BaseURL = "http://127.0.0.1:1"
	cfg.AuthBootstrap = sdsclient.AuthBootstrapConfig{}

	svc, authState, err := NewSyncService(stubImageService{}, cfg)
	if err != nil {
		t.Fatalf("NewSyncService() error = %v", err)
	}
	if svc == nil {
		t.Fatal("NewSyncService() returned nil service without auth state")
	}
	if authState != nil {
		t.Fatalf("authState = %+v, want nil without bootstrap state", authState)
	}
}

func TestBaselineRemoteProviderNilDesignDegradesSafely(t *testing.T) {
	t.Parallel()

	var provider *BaselineRemoteProvider
	detail, err := provider.GetProductDetail(context.Background(), 1)
	if err != nil {
		t.Fatalf("GetProductDetail() error = %v", err)
	}
	if detail != nil {
		t.Fatal("expected nil detail when provider is nil")
	}

	page, err := provider.GetDesignProduct(context.Background(), 2)
	if err != nil {
		t.Fatalf("GetDesignProduct() error = %v", err)
	}
	if page != nil {
		t.Fatal("expected nil page when provider is nil")
	}

	groups, err := provider.GetPrototypeGroups(context.Background(), 3)
	if err != nil {
		t.Fatalf("GetPrototypeGroups() error = %v", err)
	}
	if groups != nil {
		t.Fatal("expected nil groups when provider is nil")
	}
}
