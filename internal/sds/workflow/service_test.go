package workflow

import (
	"context"
	"errors"
	"testing"

	productasset "task-processor/internal/product/asset"
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

func TestPrepareUploadRequestFromApprovedAssetDownloadsItsURL(t *testing.T) {
	t.Parallel()

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
	downloader := &stubDownloader{content: content, fileName: "design.png"}
	svc := newServiceWithDeps(&stubDesignService{}, downloader)
	upload, err := svc.prepareUploadRequestFromApprovedAsset(productasset.ApprovedAsset{ID: "design-1", Role: productasset.RoleDesign, URL: "https://example.com/design.png"})
	if err != nil {
		t.Fatalf("prepareUploadRequestFromApprovedAsset() error = %v", err)
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
	if downloader.lastURL != "https://example.com/design.png" {
		t.Fatalf("download url = %q, want approved asset URL", downloader.lastURL)
	}
}

func TestSelectApprovedDesignAssetRejectsUnassignedGallery(t *testing.T) {
	t.Parallel()

	inventory := productasset.ApprovedAssetInventory{Assets: []productasset.ApprovedAsset{
		{ID: "gallery-1", Role: productasset.RoleGallery, URL: "https://example.com/gallery.jpg"},
	}}

	_, err := SelectApprovedDesignAsset(inventory)
	if !errors.Is(err, productasset.ErrApprovedAssetsNotReady) {
		t.Fatalf("SelectApprovedDesignAsset() error = %v, want %v", err, productasset.ErrApprovedAssetsNotReady)
	}
}

func TestSelectApprovedDesignAssetUsesExplicitRolePriority(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		assets []productasset.ApprovedAsset
		wantID string
	}{
		{
			name: "design before main and white background",
			assets: []productasset.ApprovedAsset{
				{ID: "white-1", Role: productasset.RoleWhiteBackground, URL: "https://example.com/white.jpg"},
				{ID: "main-1", Role: productasset.RoleMain, URL: "https://example.com/main.jpg"},
				{ID: "design-1", Role: productasset.RoleDesign, URL: "https://example.com/design.jpg"},
			},
			wantID: "design-1",
		},
		{
			name: "main before white background",
			assets: []productasset.ApprovedAsset{
				{ID: "white-1", Role: productasset.RoleWhiteBackground, URL: "https://example.com/white.jpg"},
				{ID: "main-1", Role: productasset.RoleMain, URL: "https://example.com/main.jpg"},
			},
			wantID: "main-1",
		},
		{
			name: "white background when explicitly assigned",
			assets: []productasset.ApprovedAsset{
				{ID: "white-1", Role: productasset.RoleWhiteBackground, URL: "https://example.com/white.jpg"},
			},
			wantID: "white-1",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, err := SelectApprovedDesignAsset(productasset.ApprovedAssetInventory{Assets: tt.assets})
			if err != nil {
				t.Fatalf("SelectApprovedDesignAsset() error = %v", err)
			}
			if got.ID != tt.wantID {
				t.Fatalf("SelectApprovedDesignAsset() id = %q, want %q", got.ID, tt.wantID)
			}
		})
	}
}

func TestSyncDesignFromApprovedAssetsUsesSelectedAsset(t *testing.T) {
	t.Parallel()

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
	designStub := &stubDesignService{}
	downloader := &stubDownloader{content: content, fileName: "design.png"}
	svc := newServiceWithDeps(designStub, downloader)

	_, err := svc.SyncDesignFromApprovedAssets(context.Background(), SyncInput{
		VariantID: 89764,
	}, productasset.ApprovedAssetInventory{
		Assets: []productasset.ApprovedAsset{
			{ID: "main-1", Role: productasset.RoleMain, URL: "https://example.com/main.jpg"},
			{ID: "design-1", Role: productasset.RoleDesign, URL: "https://example.com/design.png"},
		},
	})
	if err != nil {
		t.Fatalf("SyncDesignFromApprovedAssets() error = %v", err)
	}
	if downloader.lastURL != "https://example.com/design.png" {
		t.Fatalf("download url = %q, want selected design asset URL", downloader.lastURL)
	}
	if designStub.lastUpload.FileName != "design.png" {
		t.Fatalf("last upload filename = %q, want design.png", designStub.lastUpload.FileName)
	}
}
