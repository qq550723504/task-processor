package httpapi

import (
	"fmt"
	"strings"

	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/sirupsen/logrus"

	"task-processor/internal/core/config"
	s3integration "task-processor/internal/integration/s3"
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

func BuildImageUploadStore(cfg *config.Config, logger *logrus.Logger) (listingkit.ImageUploadStore, error) {
	if cfg == nil {
		return nil, fmt.Errorf("listingkit.imageUpload configuration is required")
	}
	switch strings.ToLower(strings.TrimSpace(cfg.ListingKit.ImageUpload.Provider)) {
	case "local":
		return buildLocalImageUploadStore(cfg)
	case "s3":
		return buildS3ImageUploadStore(cfg, logger)
	default:
		return nil, fmt.Errorf("listingkit.imageUpload.provider must be explicitly local or s3")
	}
}

func localImageUploadRootDir(cfg *config.Config) string {
	if cfg == nil {
		return ""
	}
	return cfg.ListingKit.ImageUpload.Local.RootDir
}

func buildLocalImageUploadStore(cfg *config.Config) (listingkit.ImageUploadStore, error) {
	rootDir := localImageUploadRootDir(cfg)
	store, err := listingkit.NewLocalImageUploadStore(rootDir)
	if err != nil {
		return nil, fmt.Errorf("build listingkit.imageUpload local store: %w", err)
	}
	return store, nil
}

func buildS3ImageUploadStore(cfg *config.Config, logger *logrus.Logger) (listingkit.ImageUploadStore, error) {
	client, err := newListingKitImageUploadS3Client(cfg)
	if err != nil {
		return nil, err
	}

	var componentLogger *logrus.Entry
	if logger != nil {
		componentLogger = logrus.NewEntry(logger).WithField("component", "listingkit-s3")
	}
	uploader, err := s3integration.NewUploaderWithOptions(client, s3integration.UploaderOptions{
		Bucket:       cfg.ListingKit.ImageUpload.S3.Bucket,
		PublicBase:   cfg.ListingKit.ImageUpload.S3.PublicBase,
		Endpoint:     cfg.ListingKit.ImageUpload.S3.Endpoint,
		UsePathStyle: cfg.ListingKit.ImageUpload.S3.UsePathStyle,
		Logger:       s3integration.AdaptLogrus(componentLogger),
	})
	if err != nil {
		return nil, fmt.Errorf("build listingkit.imageUpload S3 uploader: %w", err)
	}
	store, err := listingkit.NewS3ImageUploadStore(listingkit.S3ImageUploadStoreConfig{
		Bucket:   cfg.ListingKit.ImageUpload.S3.Bucket,
		Uploader: uploader,
		Reader:   client,
	})
	if err != nil {
		return nil, fmt.Errorf("build listingkit.imageUpload S3 store: %w", err)
	}
	return store, nil
}

func newListingKitImageUploadS3Client(cfg *config.Config) (*s3.Client, error) {
	if cfg == nil {
		return nil, fmt.Errorf("listingkit.imageUpload configuration is required")
	}
	s3Cfg := cfg.ListingKit.ImageUpload.S3
	if strings.TrimSpace(s3Cfg.Bucket) == "" {
		return nil, fmt.Errorf("listingkit.imageUpload.s3.bucket cannot be empty")
	}
	if strings.TrimSpace(s3Cfg.Region) == "" {
		return nil, fmt.Errorf("listingkit.imageUpload.s3.region cannot be empty")
	}
	if strings.TrimSpace(s3Cfg.AccessKeyID) == "" || strings.TrimSpace(s3Cfg.SecretAccessKey) == "" {
		return nil, fmt.Errorf("listingkit.imageUpload.s3 access key ID and secret access key are both required")
	}
	client, err := s3integration.NewClient(s3integration.ClientConfig{
		Region:          s3Cfg.Region,
		Endpoint:        s3Cfg.Endpoint,
		AccessKeyID:     s3Cfg.AccessKeyID,
		SecretAccessKey: s3Cfg.SecretAccessKey,
		UsePathStyle:    s3Cfg.UsePathStyle,
	})
	if err != nil {
		return nil, fmt.Errorf("build listingkit.imageUpload S3 client: %w", err)
	}
	return client, nil
}
