package httpapi

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/sirupsen/logrus"

	"task-processor/internal/core/config"
	storageinfra "task-processor/internal/infra/storage"
	"task-processor/internal/listingkit"
	sheinpub "task-processor/internal/publishing/shein"
)

func BuildSheinPricingPolicy(cfg *config.Config) sheinpub.PricingPolicy {
	if cfg == nil {
		return sheinpub.PricingPolicy{}
	}
	pricing := cfg.Platforms.Shein.ListingPricing
	return sheinpub.PricingPolicy{
		Enabled:        pricing.Enabled,
		Currency:       pricing.Currency,
		MarkupRate:     pricing.MarkupRate,
		FixedMarkup:    pricing.FixedMarkup,
		ShippingCost:   pricing.ShippingCost,
		CommissionRate: pricing.CommissionRate,
		MinimumPrice:   pricing.MinimumPrice,
		RoundTo:        pricing.RoundTo,
	}
}

func BuildImageUploadStore(cfg *config.Config, logger *logrus.Logger) listingkit.ImageUploadStore {
	if cfg == nil {
		return nil
	}
	if shouldUseS3ImageUploadStore(cfg) {
		primaryStore := buildS3ImageUploadStore(cfg, logger)
		if primaryStore == nil {
			return nil
		}
		fallbackStore := buildLocalImageUploadStore(cfg, logger)
		store, err := listingkit.NewFallbackImageUploadStore(primaryStore, fallbackStore)
		if err != nil {
			logger.WithError(err).Warn("listingkit image upload store fallback unavailable")
			return primaryStore
		}
		return store
	}
	return buildLocalImageUploadStore(cfg, logger)
}

func shouldUseS3ImageUploadStore(cfg *config.Config) bool {
	return cfg != nil && strings.EqualFold(strings.TrimSpace(cfg.ProductImage.Publisher.Provider), "s3")
}

func localImageUploadRootDir(cfg *config.Config) string {
	if cfg == nil {
		return ""
	}
	return filepath.Join(cfg.ProductImage.Publisher.OutputDir, "listingkit-inputs")
}

func buildLocalImageUploadStore(cfg *config.Config, logger *logrus.Logger) listingkit.ImageUploadStore {
	rootDir := localImageUploadRootDir(cfg)
	store, err := listingkit.NewLocalImageUploadStore(rootDir)
	if err != nil {
		if logger != nil {
			logger.WithError(err).Warn("local listingkit image upload store unavailable")
		}
		return nil
	}
	return store
}

func buildS3ImageUploadStore(cfg *config.Config, logger *logrus.Logger) listingkit.ImageUploadStore {
	client, err := newProductImagePublisherS3Client(cfg)
	if err != nil {
		logger.WithError(err).Warn("s3 listingkit image upload store unavailable")
		return nil
	}

	publicBase := strings.TrimSpace(cfg.ProductImage.Publisher.PublicBase)
	if publicBase == "" {
		publicBase = storageinfra.BuildS3PublicBase(
			cfg.ProductImage.Publisher.S3.Endpoint,
			cfg.ProductImage.Publisher.S3.Bucket,
			cfg.ProductImage.Publisher.S3.UsePathStyle,
		)
	}

	store, err := listingkit.NewS3ImageUploadStore(listingkit.S3ImageUploadStoreConfig{
		Bucket:     cfg.ProductImage.Publisher.S3.Bucket,
		PublicBase: publicBase,
		Uploader: storageinfra.NewS3UploaderWithOptions(client, storageinfra.S3UploaderOptions{
			Bucket:       cfg.ProductImage.Publisher.S3.Bucket,
			PublicBase:   publicBase,
			Endpoint:     cfg.ProductImage.Publisher.S3.Endpoint,
			UsePathStyle: cfg.ProductImage.Publisher.S3.UsePathStyle,
		}),
		Reader: client,
	})
	if err != nil {
		logger.WithError(err).Warn("s3 listingkit image upload store unavailable")
		return nil
	}
	return store
}

func newProductImagePublisherS3Client(cfg *config.Config) (*s3.Client, error) {
	if cfg == nil {
		return nil, fmt.Errorf("config cannot be nil")
	}
	s3Cfg := cfg.ProductImage.Publisher.S3
	if strings.TrimSpace(s3Cfg.Bucket) == "" {
		return nil, fmt.Errorf("productimage.publisher.s3.bucket cannot be empty")
	}
	return storageinfra.NewS3Client(storageinfra.S3ClientConfig{
		Region:          s3Cfg.Region,
		Endpoint:        s3Cfg.Endpoint,
		AccessKeyID:     s3Cfg.AccessKeyID,
		SecretAccessKey: s3Cfg.SecretAccessKey,
		UsePathStyle:    s3Cfg.UsePathStyle,
	})
}
