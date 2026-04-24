package httpapi

import (
	"context"
	"fmt"
	"testing"

	"github.com/sirupsen/logrus"

	"task-processor/internal/productimage"
	sdsadapter "task-processor/internal/sds/adapter"
	sdsclient "task-processor/internal/sds/client"
	sdsusecase "task-processor/internal/sds/usecase"
	sdsworkflow "task-processor/internal/sds/workflow"
)

type stubHTTPAPIImageService struct{}

func (s *stubHTTPAPIImageService) CreateProcessTask(ctx context.Context, req *productimage.ImageProcessRequest) (*productimage.Task, error) {
	return nil, nil
}

func (s *stubHTTPAPIImageService) GetTaskResult(ctx context.Context, taskID string) (*productimage.TaskResult, error) {
	return nil, nil
}

func (s *stubHTTPAPIImageService) ReviewTask(ctx context.Context, taskID string, req *productimage.ReviewTaskRequest) (*productimage.TaskResult, error) {
	return nil, nil
}

func (s *stubHTTPAPIImageService) ProcessImages(ctx context.Context, task *productimage.Task) (*productimage.ImageProcessResult, error) {
	return nil, nil
}

func (s *stubHTTPAPIImageService) SetTaskSubmitter(submitter productimage.TaskSubmitter) {}

type stubHTTPAPISDSSyncService struct{}

func (s *stubHTTPAPISDSSyncService) SyncFromRemoteImage(ctx context.Context, input sdsusecase.RemoteImageInput) (*sdsworkflow.SyncResult, error) {
	return nil, nil
}

func (s *stubHTTPAPISDSSyncService) SyncFromLocalFile(ctx context.Context, input sdsusecase.LocalFileInput) (*sdsworkflow.SyncResult, error) {
	return nil, nil
}

func (s *stubHTTPAPISDSSyncService) SyncFromImageResult(ctx context.Context, input sdsusecase.ImageResultInput) (*sdsadapter.SyncResult, error) {
	return nil, nil
}

func (s *stubHTTPAPISDSSyncService) SyncFromImageRequest(ctx context.Context, input sdsusecase.ImageRequestInput) (*sdsadapter.SyncResult, error) {
	return nil, nil
}

func TestBuildSDSSyncServiceReturnsNilWithoutImageService(t *testing.T) {
	logger := logrus.New()
	if svc := buildSDSSyncService(logger, &runtimeDeps{}); svc != nil {
		t.Fatalf("buildSDSSyncService() = %v, want nil", svc)
	}
}

func TestBuildSDSSyncServiceReturnsNilWithoutAuthState(t *testing.T) {
	logger := logrus.New()
	previousFactory := newSDSSyncServiceForHTTPAPI
	t.Cleanup(func() {
		newSDSSyncServiceForHTTPAPI = previousFactory
	})
	newSDSSyncServiceForHTTPAPI = func(imageSvc productimage.Service) (sdsusecase.Service, *sdsclient.AuthState, error) {
		return nil, nil, nil
	}

	svc := buildSDSSyncService(logger, &runtimeDeps{
		imageService: &stubHTTPAPIImageService{},
	})
	if svc != nil {
		t.Fatalf("buildSDSSyncService() = %v, want nil", svc)
	}
}

func TestBuildSDSSyncServiceReturnsNilOnFactoryError(t *testing.T) {
	logger := logrus.New()
	previousFactory := newSDSSyncServiceForHTTPAPI
	t.Cleanup(func() {
		newSDSSyncServiceForHTTPAPI = previousFactory
	})
	newSDSSyncServiceForHTTPAPI = func(imageSvc productimage.Service) (sdsusecase.Service, *sdsclient.AuthState, error) {
		return nil, nil, fmt.Errorf("boom")
	}

	svc := buildSDSSyncService(logger, &runtimeDeps{
		imageService: &stubHTTPAPIImageService{},
	})
	if svc != nil {
		t.Fatalf("buildSDSSyncService() = %v, want nil", svc)
	}
}

func TestBuildSDSSyncServiceReturnsServiceWithAuthState(t *testing.T) {
	logger := logrus.New()
	previousFactory := newSDSSyncServiceForHTTPAPI
	t.Cleanup(func() {
		newSDSSyncServiceForHTTPAPI = previousFactory
	})
	expected := &stubHTTPAPISDSSyncService{}
	newSDSSyncServiceForHTTPAPI = func(imageSvc productimage.Service) (sdsusecase.Service, *sdsclient.AuthState, error) {
		return expected, &sdsclient.AuthState{AccessToken: "token"}, nil
	}

	svc := buildSDSSyncService(logger, &runtimeDeps{
		imageService: &stubHTTPAPIImageService{},
	})
	if svc != expected {
		t.Fatalf("buildSDSSyncService() = %v, want %v", svc, expected)
	}
}
