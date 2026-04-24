package workflow

import (
	"bytes"
	"context"
	"fmt"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"task-processor/internal/pkg/downloader"
	"task-processor/internal/productimage"
	"task-processor/internal/sds/design"
)

type designSyncService interface {
	PrepareAndSyncDesign(ctx context.Context, input design.PrepareSyncDesignInput, upload design.UploadRequest) (*design.PrepareSyncDesignResult, error)
}

type imageDownloader interface {
	DownloadImage(imageURL string) ([]byte, string, error)
}

// Service 负责把图片源转换为 SDS 设计保存请求。
type Service struct {
	design     designSyncService
	downloader imageDownloader
}

// NewService 创建 workflow 服务。
func NewService(designService *design.Service) *Service {
	return &Service{
		design:     designService,
		downloader: downloader.NewImageDownloader(),
	}
}

func newServiceWithDeps(designService designSyncService, dl imageDownloader) *Service {
	if dl == nil {
		dl = downloader.NewImageDownloader()
	}
	return &Service{
		design:     designService,
		downloader: dl,
	}
}

// PrepareUploadRequestFromURL 下载远程图片并构造 SDS 上传请求。
func (s *Service) PrepareUploadRequestFromURL(_ context.Context, source ImageSource) (design.UploadRequest, error) {
	if strings.TrimSpace(source.URL) == "" {
		return design.UploadRequest{}, fmt.Errorf("image url is required")
	}
	if s.downloader == nil {
		return design.UploadRequest{}, fmt.Errorf("image downloader is not configured")
	}

	content, fileName, err := s.downloader.DownloadImage(source.URL)
	if err != nil {
		return design.UploadRequest{}, err
	}
	if strings.TrimSpace(source.FileName) != "" {
		fileName = source.FileName
	}

	width := source.Width
	height := source.Height
	if width <= 0 || height <= 0 {
		detectedWidth, detectedHeight, detectErr := detectImageSize(content)
		if detectErr != nil {
			return design.UploadRequest{}, detectErr
		}
		if width <= 0 {
			width = detectedWidth
		}
		if height <= 0 {
			height = detectedHeight
		}
	}

	return buildUploadRequest(content, fileName, source.ContentType, width, height), nil
}

// PrepareUploadRequestFromAsset 从 productimage 资产构造 SDS 上传请求。
func (s *Service) PrepareUploadRequestFromAsset(_ context.Context, source AssetSource) (design.UploadRequest, error) {
	if source.Asset == nil {
		return design.UploadRequest{}, fmt.Errorf("asset is required")
	}

	content, fileName, err := s.readAssetContent(source.Asset)
	if err != nil {
		return design.UploadRequest{}, err
	}

	width := source.Asset.Width
	height := source.Asset.Height
	if width <= 0 || height <= 0 {
		detectedWidth, detectedHeight, detectErr := detectImageSize(content)
		if detectErr != nil {
			return design.UploadRequest{}, detectErr
		}
		if width <= 0 {
			width = detectedWidth
		}
		if height <= 0 {
			height = detectedHeight
		}
	}

	contentType := ""
	if source.Asset.Metadata != nil {
		contentType = source.Asset.Metadata["content_type"]
	}

	return buildUploadRequest(content, fileName, contentType, width, height), nil
}

// SyncDesignFromURL 把远程图片直接同步到 SDS 设计页。
func (s *Service) SyncDesignFromURL(ctx context.Context, input SyncInput, source ImageSource) (*SyncResult, error) {
	upload, err := s.PrepareUploadRequestFromURL(ctx, source)
	if err != nil {
		return nil, err
	}
	return s.sync(ctx, input, upload)
}

// SyncDesignFromAsset 把 productimage 资产同步到 SDS 设计页。
func (s *Service) SyncDesignFromAsset(ctx context.Context, input SyncInput, source AssetSource) (*SyncResult, error) {
	upload, err := s.PrepareUploadRequestFromAsset(ctx, source)
	if err != nil {
		return nil, err
	}
	return s.sync(ctx, input, upload)
}

func (s *Service) sync(ctx context.Context, input SyncInput, upload design.UploadRequest) (*SyncResult, error) {
	if s.design == nil {
		return nil, fmt.Errorf("design service is not configured")
	}

	result, err := s.design.PrepareAndSyncDesign(ctx, design.PrepareSyncDesignInput{
		VariantID:        input.VariantID,
		ParentProductID:  input.ParentProductID,
		PrototypeGroupID: input.PrototypeGroupID,
		MerchantResultID: input.MerchantResultID,
		DesignType:       input.DesignType,
		LayerID:          input.LayerID,
		FitLevel:         input.FitLevel,
		ResizeMode:       input.ResizeMode,
	}, upload)
	if err != nil {
		return nil, err
	}

	return &SyncResult{
		UploadRequest: upload,
		DesignResult:  result,
	}, nil
}

func (s *Service) readAssetContent(asset *productimage.ImageAsset) ([]byte, string, error) {
	if asset == nil {
		return nil, "", fmt.Errorf("asset is nil")
	}

	if localPath := resolveAssetLocalPath(asset); localPath != "" {
		content, err := os.ReadFile(localPath)
		if err != nil {
			return nil, "", fmt.Errorf("read local asset %q: %w", localPath, err)
		}
		return content, filepath.Base(localPath), nil
	}

	remoteURL := strings.TrimSpace(asset.SourceURL)
	if remoteURL == "" {
		remoteURL = strings.TrimSpace(asset.URL)
	}
	if remoteURL == "" {
		return nil, "", fmt.Errorf("asset has no readable source")
	}
	if s.downloader == nil {
		return nil, "", fmt.Errorf("image downloader is not configured")
	}

	content, fileName, err := s.downloader.DownloadImage(remoteURL)
	if err != nil {
		return nil, "", err
	}
	return content, fileName, nil
}

func resolveAssetLocalPath(asset *productimage.ImageAsset) string {
	if asset == nil {
		return ""
	}
	if asset.Metadata != nil {
		if publishedPath := existingLocalPath(asset.Metadata["published_path"]); publishedPath != "" {
			return publishedPath
		}
		if localPath := existingLocalPath(asset.Metadata["local_path"]); localPath != "" {
			return localPath
		}
	}
	if value := existingLocalPath(asset.URL); value != "" {
		return value
	}
	return ""
}

func existingLocalPath(value string) string {
	value = strings.TrimSpace(value)
	if value == "" || isRemoteURL(value) {
		return ""
	}
	if _, err := os.Stat(value); err != nil {
		return ""
	}
	return value
}

func isRemoteURL(value string) bool {
	lower := strings.ToLower(strings.TrimSpace(value))
	return strings.HasPrefix(lower, "http://") || strings.HasPrefix(lower, "https://")
}

func buildUploadRequest(content []byte, fileName, contentType string, width, height int) design.UploadRequest {
	fileName = strings.TrimSpace(fileName)
	if fileName == "" {
		fileName = "image.jpg"
	}
	contentType = strings.TrimSpace(contentType)
	if contentType == "" {
		contentType = http.DetectContentType(content)
	}
	if strings.EqualFold(contentType, "application/octet-stream") {
		contentType = contentTypeFromExtension(fileName)
	}

	return design.UploadRequest{
		FileName:    fileName,
		Content:     content,
		ContentType: contentType,
		Width:       width,
		Height:      height,
	}
}

func detectImageSize(content []byte) (int, int, error) {
	cfg, _, err := image.DecodeConfig(bytes.NewReader(content))
	if err != nil {
		return 0, 0, fmt.Errorf("decode image config: %w", err)
	}
	if cfg.Width <= 0 || cfg.Height <= 0 {
		return 0, 0, fmt.Errorf("image dimensions are invalid")
	}
	return cfg.Width, cfg.Height, nil
}

func contentTypeFromExtension(fileName string) string {
	switch strings.ToLower(strings.TrimSpace(filepath.Ext(fileName))) {
	case ".png":
		return "image/png"
	case ".gif":
		return "image/gif"
	case ".webp":
		return "image/webp"
	case ".jpg", ".jpeg":
		return "image/jpeg"
	default:
		return "image/jpeg"
	}
}
