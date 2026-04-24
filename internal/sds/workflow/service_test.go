package workflow

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"task-processor/internal/productimage"
	"task-processor/internal/sds/design"
)

type stubDesignService struct {
	lastInput  design.PrepareSyncDesignInput
	lastUpload design.UploadRequest
	result     *design.PrepareSyncDesignResult
	err        error
}

func (s *stubDesignService) PrepareAndSyncDesign(_ context.Context, input design.PrepareSyncDesignInput, upload design.UploadRequest) (*design.PrepareSyncDesignResult, error) {
	s.lastInput = input
	s.lastUpload = upload
	if s.err != nil {
		return nil, s.err
	}
	if s.result != nil {
		return s.result, nil
	}
	return &design.PrepareSyncDesignResult{}, nil
}

type stubDownloader struct {
	content  []byte
	fileName string
	err      error
	lastURL  string
}

func (d *stubDownloader) DownloadImage(imageURL string) ([]byte, string, error) {
	d.lastURL = imageURL
	return d.content, d.fileName, d.err
}

func TestPrepareUploadRequestFromAssetUsesLocalPath(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	localPath := filepath.Join(tmpDir, "design.png")
	content := []byte{
		0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a,
		0x00, 0x00, 0x00, 0x0d, 0x49, 0x48, 0x44, 0x52,
		0x00, 0x00, 0x00, 0x01, 0x00, 0x00, 0x00, 0x01,
		0x08, 0x02, 0x00, 0x00, 0x00, 0x90, 0x77, 0x53,
		0xde, 0x00, 0x00, 0x00, 0x0c, 0x49, 0x44, 0x41,
		0x54, 0x08, 0xd7, 0x63, 0xf8, 0xcf, 0xc0, 0x00,
		0x00, 0x03, 0x01, 0x01, 0x00, 0xc9, 0xfe, 0x92,
		0xef, 0x00, 0x00, 0x00, 0x00, 0x49, 0x45, 0x4e,
		0x44, 0xae, 0x42, 0x60, 0x82,
	}
	if err := os.WriteFile(localPath, content, 0o644); err != nil {
		t.Fatalf("write local asset: %v", err)
	}

	svc := newServiceWithDeps(&stubDesignService{}, &stubDownloader{})
	upload, err := svc.PrepareUploadRequestFromAsset(context.Background(), AssetSource{
		Asset: &productimage.ImageAsset{
			URL: filepath.ToSlash(localPath),
			Metadata: map[string]string{
				"local_path":   localPath,
				"content_type": "image/png",
			},
		},
	})
	if err != nil {
		t.Fatalf("PrepareUploadRequestFromAsset() error = %v", err)
	}
	if upload.FileName != "design.png" {
		t.Fatalf("upload.FileName = %q, want design.png", upload.FileName)
	}
	if upload.ContentType != "image/png" {
		t.Fatalf("upload.ContentType = %q, want image/png", upload.ContentType)
	}
	if upload.Width != 1 || upload.Height != 1 {
		t.Fatalf("upload size = %dx%d, want 1x1", upload.Width, upload.Height)
	}
}

func TestSyncDesignFromURLBuildsUploadAndDelegates(t *testing.T) {
	t.Parallel()

	png := []byte{
		0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a,
		0x00, 0x00, 0x00, 0x0d, 0x49, 0x48, 0x44, 0x52,
		0x00, 0x00, 0x00, 0x01, 0x00, 0x00, 0x00, 0x01,
		0x08, 0x02, 0x00, 0x00, 0x00, 0x90, 0x77, 0x53,
		0xde, 0x00, 0x00, 0x00, 0x0c, 0x49, 0x44, 0x41,
		0x54, 0x08, 0xd7, 0x63, 0xf8, 0xcf, 0xc0, 0x00,
		0x00, 0x03, 0x01, 0x01, 0x00, 0xc9, 0xfe, 0x92,
		0xef, 0x00, 0x00, 0x00, 0x00, 0x49, 0x45, 0x4e,
		0x44, 0xae, 0x42, 0x60, 0x82,
	}

	designStub := &stubDesignService{}
	downloaderStub := &stubDownloader{
		content:  png,
		fileName: "remote.png",
	}
	svc := newServiceWithDeps(designStub, downloaderStub)

	result, err := svc.SyncDesignFromURL(context.Background(), SyncInput{
		VariantID:        89764,
		PrototypeGroupID: 14555,
		LayerID:          "698744758333792256",
		FitLevel:         1,
	}, ImageSource{
		URL: "https://example.com/design.png",
	})
	if err != nil {
		t.Fatalf("SyncDesignFromURL() error = %v", err)
	}
	if result == nil {
		t.Fatalf("SyncDesignFromURL() result is nil")
	}
	if downloaderStub.lastURL != "https://example.com/design.png" {
		t.Fatalf("downloader last url = %q", downloaderStub.lastURL)
	}
	if designStub.lastInput.VariantID != 89764 {
		t.Fatalf("last variant id = %d, want 89764", designStub.lastInput.VariantID)
	}
	if designStub.lastUpload.FileName != "remote.png" {
		t.Fatalf("last upload filename = %q, want remote.png", designStub.lastUpload.FileName)
	}
	if designStub.lastUpload.Width != 1 || designStub.lastUpload.Height != 1 {
		t.Fatalf("last upload size = %dx%d, want 1x1", designStub.lastUpload.Width, designStub.lastUpload.Height)
	}
}

func TestPrepareUploadRequestFromFileReadsLocalImage(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	localPath := filepath.Join(tmpDir, "local.png")
	content := []byte{
		0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a,
		0x00, 0x00, 0x00, 0x0d, 0x49, 0x48, 0x44, 0x52,
		0x00, 0x00, 0x00, 0x01, 0x00, 0x00, 0x00, 0x01,
		0x08, 0x02, 0x00, 0x00, 0x00, 0x90, 0x77, 0x53,
		0xde, 0x00, 0x00, 0x00, 0x0c, 0x49, 0x44, 0x41,
		0x54, 0x08, 0xd7, 0x63, 0xf8, 0xcf, 0xc0, 0x00,
		0x00, 0x03, 0x01, 0x01, 0x00, 0xc9, 0xfe, 0x92,
		0xef, 0x00, 0x00, 0x00, 0x00, 0x49, 0x45, 0x4e,
		0x44, 0xae, 0x42, 0x60, 0x82,
	}
	if err := os.WriteFile(localPath, content, 0o644); err != nil {
		t.Fatalf("write local file: %v", err)
	}

	svc := newServiceWithDeps(&stubDesignService{}, &stubDownloader{})
	upload, err := svc.PrepareUploadRequestFromFile(context.Background(), FileSource{Path: localPath})
	if err != nil {
		t.Fatalf("PrepareUploadRequestFromFile() error = %v", err)
	}
	if upload.FileName != "local.png" {
		t.Fatalf("upload.FileName = %q, want local.png", upload.FileName)
	}
	if upload.Width != 1 || upload.Height != 1 {
		t.Fatalf("upload size = %dx%d, want 1x1", upload.Width, upload.Height)
	}
}

func TestSelectDesignAssetPrefersWhiteBackground(t *testing.T) {
	t.Parallel()

	main := &productimage.ImageAsset{URL: "main.jpg", Type: productimage.AssetTypeMainImage}
	white := &productimage.ImageAsset{URL: "white.jpg", Type: productimage.AssetTypeWhiteBgImage}
	subject := &productimage.ImageAsset{URL: "subject.png", Type: productimage.AssetTypeSubjectCutout}

	got, err := SelectDesignAsset(&productimage.ImageProcessResult{
		MainImage:     main,
		WhiteBgImage:  white,
		SubjectCutout: subject,
	})
	if err != nil {
		t.Fatalf("SelectDesignAsset() error = %v", err)
	}
	if got != white {
		t.Fatalf("SelectDesignAsset() got %v, want white background image", got)
	}
}

func TestSyncDesignFromProcessResultUsesSelectedAsset(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	localPath := filepath.Join(tmpDir, "white.png")
	content := []byte{
		0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a,
		0x00, 0x00, 0x00, 0x0d, 0x49, 0x48, 0x44, 0x52,
		0x00, 0x00, 0x00, 0x01, 0x00, 0x00, 0x00, 0x01,
		0x08, 0x02, 0x00, 0x00, 0x00, 0x90, 0x77, 0x53,
		0xde, 0x00, 0x00, 0x00, 0x0c, 0x49, 0x44, 0x41,
		0x54, 0x08, 0xd7, 0x63, 0xf8, 0xcf, 0xc0, 0x00,
		0x00, 0x03, 0x01, 0x01, 0x00, 0xc9, 0xfe, 0x92,
		0xef, 0x00, 0x00, 0x00, 0x00, 0x49, 0x45, 0x4e,
		0x44, 0xae, 0x42, 0x60, 0x82,
	}
	if err := os.WriteFile(localPath, content, 0o644); err != nil {
		t.Fatalf("write local asset: %v", err)
	}

	designStub := &stubDesignService{}
	svc := newServiceWithDeps(designStub, &stubDownloader{})

	_, err := svc.SyncDesignFromProcessResult(context.Background(), SyncInput{
		VariantID: 89764,
	}, &productimage.ImageProcessResult{
		MainImage: &productimage.ImageAsset{
			URL: "main.jpg",
		},
		WhiteBgImage: &productimage.ImageAsset{
			URL:    localPath,
			Width:  1,
			Height: 1,
			Metadata: map[string]string{
				"local_path":   localPath,
				"content_type": "image/png",
			},
		},
	})
	if err != nil {
		t.Fatalf("SyncDesignFromProcessResult() error = %v", err)
	}
	if designStub.lastUpload.FileName != "white.png" {
		t.Fatalf("last upload filename = %q, want white.png", designStub.lastUpload.FileName)
	}
}
