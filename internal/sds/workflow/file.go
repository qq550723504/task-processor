package workflow

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"task-processor/internal/sds/design"
)

// PrepareUploadRequestFromFile 从本地文件构造 SDS 上传请求。
func (s *Service) PrepareUploadRequestFromFile(_ context.Context, source FileSource) (design.UploadRequest, error) {
	path := strings.TrimSpace(source.Path)
	if path == "" {
		return design.UploadRequest{}, fmt.Errorf("file path is required")
	}

	content, err := os.ReadFile(path)
	if err != nil {
		return design.UploadRequest{}, fmt.Errorf("read image file %q: %w", path, err)
	}

	fileName := strings.TrimSpace(source.FileName)
	if fileName == "" {
		fileName = filepath.Base(path)
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

// SyncDesignFromFile 把本地图片文件直接同步到 SDS 设计页。
func (s *Service) SyncDesignFromFile(ctx context.Context, input SyncInput, source FileSource) (*SyncResult, error) {
	upload, err := s.PrepareUploadRequestFromFile(ctx, source)
	if err != nil {
		return nil, err
	}
	return s.sync(ctx, input, upload)
}
