package usecase

import (
	"context"
	"testing"

	"task-processor/internal/productimage"
	"task-processor/internal/sds/adapter"
	"task-processor/internal/sds/workflow"
)

type stubWorkflow struct {
	lastSync workflow.SyncInput
	lastURL  workflow.ImageSource
	lastFile workflow.FileSource
	result   *workflow.SyncResult
	err      error
}

func (s *stubWorkflow) SyncDesignFromURL(_ context.Context, input workflow.SyncInput, source workflow.ImageSource) (*workflow.SyncResult, error) {
	s.lastSync = input
	s.lastURL = source
	if s.err != nil {
		return nil, s.err
	}
	if s.result != nil {
		return s.result, nil
	}
	return &workflow.SyncResult{}, nil
}

func (s *stubWorkflow) SyncDesignFromFile(_ context.Context, input workflow.SyncInput, source workflow.FileSource) (*workflow.SyncResult, error) {
	s.lastSync = input
	s.lastFile = source
	if s.err != nil {
		return nil, s.err
	}
	if s.result != nil {
		return s.result, nil
	}
	return &workflow.SyncResult{}, nil
}

type stubAdapter struct {
	lastSync     workflow.SyncInput
	lastImageReq *productimage.ImageProcessRequest
	lastImageRes *productimage.ImageProcessResult
	result       *adapter.SyncResult
	err          error
}

func (s *stubAdapter) SyncFromImageRequest(_ context.Context, input adapter.SyncFromImageRequestInput) (*adapter.SyncResult, error) {
	s.lastSync = input.SyncInput
	s.lastImageReq = input.ImageRequest
	if s.err != nil {
		return nil, s.err
	}
	if s.result != nil {
		return s.result, nil
	}
	return &adapter.SyncResult{}, nil
}

func (s *stubAdapter) SyncFromImageResult(_ context.Context, input workflow.SyncInput, result *productimage.ImageProcessResult) (*adapter.SyncResult, error) {
	s.lastSync = input
	s.lastImageRes = result
	if s.err != nil {
		return nil, s.err
	}
	if s.result != nil {
		return s.result, nil
	}
	return &adapter.SyncResult{}, nil
}

func TestSyncFromRemoteImageDelegatesToWorkflow(t *testing.T) {
	t.Parallel()

	wf := &stubWorkflow{result: &workflow.SyncResult{}}
	svc := &service{workflow: wf}

	_, err := svc.SyncFromRemoteImage(context.Background(), RemoteImageInput{
		Sync:  SyncInput{VariantID: 89764},
		Image: workflow.ImageSource{URL: "https://example.com/a.jpg"},
	})
	if err != nil {
		t.Fatalf("SyncFromRemoteImage() error = %v", err)
	}
	if wf.lastSync.VariantID != 89764 {
		t.Fatalf("variant id = %d, want 89764", wf.lastSync.VariantID)
	}
	if wf.lastURL.URL != "https://example.com/a.jpg" {
		t.Fatalf("url = %q", wf.lastURL.URL)
	}
}

func TestSyncFromLocalFileDelegatesToWorkflow(t *testing.T) {
	t.Parallel()

	wf := &stubWorkflow{result: &workflow.SyncResult{}}
	svc := &service{workflow: wf}

	_, err := svc.SyncFromLocalFile(context.Background(), LocalFileInput{
		Sync: SyncInput{VariantID: 89765},
		File: workflow.FileSource{Path: "D:/tmp/a.png"},
	})
	if err != nil {
		t.Fatalf("SyncFromLocalFile() error = %v", err)
	}
	if wf.lastFile.Path != "D:/tmp/a.png" {
		t.Fatalf("file path = %q", wf.lastFile.Path)
	}
}

func TestSyncFromImageResultDelegatesToAdapter(t *testing.T) {
	t.Parallel()

	adp := &stubAdapter{result: &adapter.SyncResult{}}
	svc := &service{adapter: adp}
	imageResult := &productimage.ImageProcessResult{
		WhiteBgImage: &productimage.ImageAsset{URL: "white.jpg"},
	}

	_, err := svc.SyncFromImageResult(context.Background(), ImageResultInput{
		Sync:        SyncInput{VariantID: 89766},
		ImageResult: imageResult,
	})
	if err != nil {
		t.Fatalf("SyncFromImageResult() error = %v", err)
	}
	if adp.lastImageRes != imageResult {
		t.Fatalf("unexpected image result delegation")
	}
}

func TestSyncFromImageRequestDelegatesToAdapter(t *testing.T) {
	t.Parallel()

	adp := &stubAdapter{result: &adapter.SyncResult{}}
	svc := &service{adapter: adp}
	req := &productimage.ImageProcessRequest{
		ImageURLs:   []string{"https://example.com/a.jpg"},
		Marketplace: "amazon",
	}

	_, err := svc.SyncFromImageRequest(context.Background(), ImageRequestInput{
		Sync:         SyncInput{VariantID: 89767},
		ImageRequest: req,
	})
	if err != nil {
		t.Fatalf("SyncFromImageRequest() error = %v", err)
	}
	if adp.lastImageReq != req {
		t.Fatalf("unexpected image request delegation")
	}
}
