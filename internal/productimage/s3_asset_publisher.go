package productimage

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"task-processor/internal/infra/storage"
)

type s3AssetUploadWriter interface {
	Upload(ctx context.Context, key string, data []byte, contentType string) (string, error)
}

type S3AssetPublisherConfig struct {
	Uploader   s3AssetUploadWriter
	PublicBase string
}

type s3AssetPublisher struct {
	uploader   s3AssetUploadWriter
	publicBase string
}

func NewS3AssetPublisher(cfg S3AssetPublisherConfig) (AssetPublisher, error) {
	if cfg.Uploader == nil {
		return nil, fmt.Errorf("uploader cannot be nil")
	}
	return &s3AssetPublisher{
		uploader:   cfg.Uploader,
		publicBase: strings.TrimRight(strings.TrimSpace(cfg.PublicBase), "/"),
	}, nil
}

func (p *s3AssetPublisher) Publish(ctx context.Context, req *ImageProcessRequest, result *ImageProcessResult) error {
	if result == nil {
		return fmt.Errorf("result cannot be nil")
	}
	taskKey := publisherTaskKey(req)
	if err := p.publishAsset(ctx, taskKey, result.MainImage, "main"); err != nil {
		return err
	}
	if err := p.publishAsset(ctx, taskKey, result.WhiteBgImage, "white-bg"); err != nil {
		return err
	}
	if err := p.publishAsset(ctx, taskKey, result.SubjectCutout, "subject"); err != nil {
		return err
	}
	for idx := range result.GalleryImages {
		if err := p.publishAsset(ctx, taskKey, &result.GalleryImages[idx], fmt.Sprintf("gallery-%d", idx+1)); err != nil {
			return err
		}
	}
	return nil
}

func (p *s3AssetPublisher) publishAsset(ctx context.Context, taskKey string, asset *ImageAsset, prefix string) error {
	if asset == nil {
		return nil
	}
	localPath := ""
	if asset.Metadata != nil {
		localPath = asset.Metadata["local_path"]
	}
	if localPath == "" {
		return nil
	}
	data, err := os.ReadFile(localPath)
	if err != nil {
		return fmt.Errorf("read asset %q: %w", localPath, err)
	}

	targetName := buildPublishedFilename(prefix, localPath)
	key := filepath.ToSlash(filepath.Join(taskKey, targetName))
	fallbackURL, err := p.uploader.Upload(ctx, key, data, detectAssetContentType(localPath, data))
	if err != nil {
		return fmt.Errorf("upload asset %q to s3: %w", localPath, err)
	}

	if asset.Metadata == nil {
		asset.Metadata = map[string]string{}
	}
	asset.Metadata["published_provider"] = "s3"
	asset.Metadata["published_path"] = localPath
	asset.Metadata["published_size_bytes"] = fmt.Sprintf("%d", len(data))
	asset.Metadata["published_key"] = key
	asset.Metadata["published_url"] = storage.ResolveObjectURL(p.publicBase, key, fallbackURL)
	asset.URL = asset.Metadata["published_url"]
	return nil
}

func detectAssetContentType(path string, data []byte) string {
	switch strings.ToLower(strings.TrimSpace(filepath.Ext(path))) {
	case ".png":
		return "image/png"
	case ".gif":
		return "image/gif"
	case ".webp":
		return "image/webp"
	default:
		_ = data
		return "image/jpeg"
	}
}
