package productimage

import (
	"context"
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	amazonapi "task-processor/internal/amazon/api"
	amazonimage "task-processor/internal/amazon/image"
	coreconfig "task-processor/internal/core/config"
)

type multiAssetPublisher struct {
	publishers []AssetPublisher
}

func NewMultiAssetPublisher(publishers ...AssetPublisher) AssetPublisher {
	chain := make([]AssetPublisher, 0, len(publishers))
	for _, publisher := range publishers {
		if publisher != nil {
			chain = append(chain, publisher)
		}
	}
	if len(chain) == 0 {
		return nil
	}
	if len(chain) == 1 {
		return chain[0]
	}
	return &multiAssetPublisher{publishers: chain}
}

func (p *multiAssetPublisher) Publish(ctx context.Context, req *ImageProcessRequest, result *ImageProcessResult) error {
	for _, publisher := range p.publishers {
		if err := publisher.Publish(ctx, req, result); err != nil {
			return err
		}
	}
	return nil
}

type localAssetPublisher struct {
	outputDir  string
	publicBase string
}

func NewLocalAssetPublisher(outputDir, publicBase string) (AssetPublisher, error) {
	outputDir = strings.TrimSpace(outputDir)
	if outputDir == "" {
		return nil, fmt.Errorf("output dir cannot be empty")
	}
	if err := os.MkdirAll(outputDir, 0o755); err != nil {
		return nil, fmt.Errorf("create output dir: %w", err)
	}
	return &localAssetPublisher{
		outputDir:  outputDir,
		publicBase: strings.TrimRight(strings.TrimSpace(publicBase), "/"),
	}, nil
}

func (p *localAssetPublisher) Publish(_ context.Context, req *ImageProcessRequest, result *ImageProcessResult) error {
	if result == nil {
		return fmt.Errorf("result cannot be nil")
	}
	taskKey := publisherTaskKey(req)
	if err := p.publishAsset(taskKey, result.MainImage, "main"); err != nil {
		return err
	}
	if err := p.publishAsset(taskKey, result.WhiteBgImage, "white-bg"); err != nil {
		return err
	}
	if err := p.publishAsset(taskKey, result.SubjectCutout, "subject"); err != nil {
		return err
	}
	for idx := range result.GalleryImages {
		if err := p.publishAsset(taskKey, &result.GalleryImages[idx], fmt.Sprintf("gallery-%d", idx+1)); err != nil {
			return err
		}
	}
	return nil
}

func (p *localAssetPublisher) publishAsset(taskKey string, asset *ImageAsset, prefix string) error {
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
	sourceInfo, err := os.Stat(localPath)
	if err != nil {
		return fmt.Errorf("stat asset %q: %w", localPath, err)
	}
	taskDir := filepath.Join(p.outputDir, taskKey)
	if err := os.MkdirAll(taskDir, 0o755); err != nil {
		return fmt.Errorf("create asset dir: %w", err)
	}
	targetName := buildPublishedFilename(prefix, localPath)
	targetPath := filepath.Join(taskDir, targetName)
	if samePath(localPath, targetPath) {
		p.applyPublishedMetadata(asset, localPath, taskKey, targetName, sourceInfo.Size())
		return nil
	}
	data, err := os.ReadFile(localPath)
	if err != nil {
		return fmt.Errorf("read asset %q: %w", localPath, err)
	}
	if err := os.WriteFile(targetPath, data, 0o644); err != nil {
		return fmt.Errorf("write published asset %q: %w", targetPath, err)
	}
	p.applyPublishedMetadata(asset, targetPath, taskKey, targetName, int64(len(data)))
	return nil
}

func (p *localAssetPublisher) applyPublishedMetadata(asset *ImageAsset, targetPath, taskKey, targetName string, size int64) {
	if asset.Metadata == nil {
		asset.Metadata = map[string]string{}
	}
	asset.Metadata["published_provider"] = "local"
	asset.Metadata["published_path"] = targetPath
	asset.Metadata["published_size_bytes"] = fmt.Sprintf("%d", size)
	asset.Metadata["published_key"] = filepath.ToSlash(filepath.Join(taskKey, targetName))
	if p.publicBase != "" {
		asset.Metadata["published_url"] = p.publicBase + "/" + asset.Metadata["published_key"]
		asset.URL = asset.Metadata["published_url"]
		return
	}
	asset.URL = targetPath
}

type amazonAssetPublisher struct {
	service       *amazonimage.ImageManagementService
	marketplaceID string
}

func NewAmazonAssetPublisher(cfg *coreconfig.Config) (AssetPublisher, error) {
	if cfg == nil {
		return nil, fmt.Errorf("config cannot be nil")
	}
	if !cfg.Amazon.SPAPI.Enabled {
		return nil, fmt.Errorf("amazon SP-API is not enabled")
	}
	apiClient := amazonapi.NewClient(&amazonapi.Config{
		Region:         cfg.Amazon.SPAPI.Region,
		MarketplaceID:  resolveMarketplaceID(cfg),
		ClientID:       cfg.Amazon.SPAPI.ClientID,
		ClientSecret:   cfg.Amazon.SPAPI.ClientSecret,
		RefreshToken:   cfg.Amazon.SPAPI.RefreshToken,
		AWSAccessKeyID: cfg.Amazon.SPAPI.AWSAccessKeyID,
		AWSSecretKey:   cfg.Amazon.SPAPI.AWSSecretKey,
	})
	return &amazonAssetPublisher{
		service:       amazonimage.NewImageManagementService(apiClient),
		marketplaceID: resolveMarketplaceID(cfg),
	}, nil
}

func (p *amazonAssetPublisher) Publish(ctx context.Context, _ *ImageProcessRequest, result *ImageProcessResult) error {
	if result == nil {
		return fmt.Errorf("result cannot be nil")
	}
	if err := p.publishAsset(ctx, result.MainImage, "main"); err != nil {
		return err
	}
	if err := p.publishAsset(ctx, result.WhiteBgImage, "white-bg"); err != nil {
		return err
	}
	if err := p.publishAsset(ctx, result.SubjectCutout, "subject"); err != nil {
		return err
	}
	for idx := range result.GalleryImages {
		if err := p.publishAsset(ctx, &result.GalleryImages[idx], fmt.Sprintf("gallery-%d", idx+1)); err != nil {
			return err
		}
	}
	return nil
}

func (p *amazonAssetPublisher) publishAsset(ctx context.Context, asset *ImageAsset, prefix string) error {
	if asset == nil {
		return nil
	}
	localPath := ""
	if asset.Metadata != nil {
		localPath = asset.Metadata["published_path"]
		if localPath == "" {
			localPath = asset.Metadata["local_path"]
		}
	}
	if localPath == "" {
		return nil
	}
	data, err := os.ReadFile(localPath)
	if err != nil {
		return fmt.Errorf("read asset %q: %w", localPath, err)
	}
	filename := filepath.Base(localPath)
	if prefix != "" {
		filename = fmt.Sprintf("%s-%s", prefix, filename)
	}
	uploadResult, err := p.service.UploadImage(ctx, data, filename, p.marketplaceID)
	if err != nil {
		return fmt.Errorf("upload asset %q: %w", localPath, err)
	}
	if asset.Metadata == nil {
		asset.Metadata = map[string]string{}
	}
	asset.Metadata["uploaded_image_id"] = uploadResult.ImageID
	asset.Metadata["uploaded_url"] = uploadResult.URL
	asset.Metadata["upload_format"] = uploadResult.Format
	asset.Metadata["original_local_path"] = localPath
	asset.Metadata["published_provider"] = "amazon"
	asset.URL = uploadResult.URL
	return nil
}

func resolveMarketplaceID(cfg *coreconfig.Config) string {
	if cfg == nil {
		return ""
	}
	key := strings.TrimSpace(cfg.Amazon.SPAPI.DefaultMarketplace)
	if key != "" {
		if market, ok := cfg.Amazon.SPAPI.Marketplaces[key]; ok && market.MarketplaceID != "" {
			return market.MarketplaceID
		}
		return key
	}
	for _, market := range cfg.Amazon.SPAPI.Marketplaces {
		if market.Enabled && market.MarketplaceID != "" {
			return market.MarketplaceID
		}
	}
	return ""
}

func publisherTaskKey(req *ImageProcessRequest) string {
	raw := "manual"
	if req != nil {
		switch {
		case strings.TrimSpace(req.ProductURL) != "":
			raw = req.ProductURL
		case len(req.ImageURLs) > 0:
			raw = strings.Join(req.ImageURLs, "|")
		case strings.TrimSpace(req.Text) != "":
			raw = req.Text
		}
	}
	sum := sha1.Sum([]byte(raw))
	return hex.EncodeToString(sum[:8])
}

func buildPublishedFilename(prefix, sourcePath string) string {
	ext := filepath.Ext(sourcePath)
	if ext == "" {
		ext = ".jpg"
	}
	base := strings.TrimSuffix(filepath.Base(sourcePath), filepath.Ext(sourcePath))
	base = sanitizePathToken(base)
	if base == "" {
		base = "asset"
	}
	prefix = sanitizePathToken(prefix)
	if prefix != "" {
		return prefix + "-" + base + ext
	}
	return base + ext
}

func sanitizePathToken(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	replacer := strings.NewReplacer("\\", "-", "/", "-", " ", "-", ":", "-", "*", "-", "?", "-", "\"", "-", "<", "-", ">", "-", "|", "-", "_", "-")
	value = replacer.Replace(value)
	value = strings.Trim(value, "-.")
	return value
}

func samePath(a, b string) bool {
	aa := filepath.Clean(a)
	bb := filepath.Clean(b)
	return strings.EqualFold(aa, bb)
}
