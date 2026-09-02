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
	"path/filepath"
	"strings"

	"task-processor/internal/pkg/downloader"
	productasset "task-processor/internal/product/asset"
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

func (s *Service) prepareUploadRequestFromApprovedAsset(asset productasset.ApprovedAsset) (design.UploadRequest, error) {
	content, fileName, err := s.readAssetContent(asset)
	if err != nil {
		return design.UploadRequest{}, err
	}

	width := asset.Width
	height := asset.Height
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

	return buildUploadRequest(content, fileName, "", width, height), nil
}

func (s *Service) syncDesignFromApprovedAsset(ctx context.Context, input SyncInput, approved productasset.ApprovedAsset) (*SyncResult, error) {
	upload, err := s.prepareUploadRequestFromApprovedAsset(approved)
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
		VariantID:              input.VariantID,
		RelatedVariantIDs:      append([]int64(nil), input.RelatedVariantIDs...),
		RelatedVariantLayerIDs: cloneRelatedVariantLayerIDs(input.RelatedVariantLayerIDs),
		ParentProductID:        input.ParentProductID,
		PrototypeGroupID:       input.PrototypeGroupID,
		MerchantResultID:       input.MerchantResultID,
		DesignType:             input.DesignType,
		LayerID:                input.LayerID,
		FitLevel:               input.FitLevel,
		ResizeMode:             input.ResizeMode,
		BlankDesignURL:         input.BlankDesignURL,
	}, upload)
	if err != nil {
		return nil, err
	}

	return &SyncResult{
		UploadRequest: upload,
		DesignResult:  result,
	}, nil
}

func cloneRelatedVariantLayerIDs(values map[int64]string) map[int64]string {
	if len(values) == 0 {
		return nil
	}
	cloned := make(map[int64]string, len(values))
	for variantID, layerID := range values {
		cloned[variantID] = layerID
	}
	return cloned
}

func (s *Service) readAssetContent(asset productasset.ApprovedAsset) ([]byte, string, error) {
	remoteURL := strings.TrimSpace(asset.URL)
	if remoteURL == "" {
		return nil, "", fmt.Errorf("approved asset has no readable URL")
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
