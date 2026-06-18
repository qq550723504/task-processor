package httpapi

import (
	"fmt"
	"strings"

	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/sirupsen/logrus"

	"task-processor/internal/core/config"
	storageinfra "task-processor/internal/infra/storage"
	productimage "task-processor/internal/productimage"
)

func buildAssetPublisher(cfg *config.Config, logger *logrus.Logger) productimage.AssetPublisher {
	if cfg == nil || !cfg.ProductImage.Publisher.Enabled {
		return nil
	}

	provider := strings.ToLower(strings.TrimSpace(cfg.ProductImage.Publisher.Provider))
	switch provider {
	case "", "local":
		publisher, err := productimage.NewLocalAssetPublisher(cfg.ProductImage.Publisher.OutputDir, cfg.ProductImage.Publisher.PublicBase)
		if err != nil {
			logger.WithError(err).Warn("local image asset publisher unavailable")
			return nil
		}
		return publisher
	case "s3":
		return buildS3AssetPublisher(cfg, logger)
	case "amazon":
		publisher, err := productimage.NewAmazonAssetPublisher(cfg)
		if err != nil {
			logger.WithError(err).Warn("amazon image asset publisher unavailable")
			return nil
		}
		return publisher
	case "hybrid":
		localPublisher, err := productimage.NewLocalAssetPublisher(cfg.ProductImage.Publisher.OutputDir, cfg.ProductImage.Publisher.PublicBase)
		if err != nil {
			logger.WithError(err).Warn("hybrid local image asset publisher unavailable")
			return nil
		}
		amazonPublisher, err := productimage.NewAmazonAssetPublisher(cfg)
		if err != nil {
			logger.WithError(err).Warn("hybrid amazon image asset publisher partially unavailable")
			return localPublisher
		}
		return productimage.NewMultiAssetPublisher(localPublisher, amazonPublisher)
	default:
		logger.Warnf("unsupported image publisher provider: %s", provider)
		return nil
	}
}

func newPublisherS3Client(cfg *config.Config) (*s3.Client, error) {
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

func buildS3AssetPublisher(cfg *config.Config, logger *logrus.Logger) productimage.AssetPublisher {
	client, err := newPublisherS3Client(cfg)
	if err != nil {
		logger.WithError(err).Warn("s3 image asset publisher unavailable")
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

	uploader := storageinfra.NewS3UploaderWithOptions(client, storageinfra.S3UploaderOptions{
		Bucket:       cfg.ProductImage.Publisher.S3.Bucket,
		PublicBase:   publicBase,
		Endpoint:     cfg.ProductImage.Publisher.S3.Endpoint,
		UsePathStyle: cfg.ProductImage.Publisher.S3.UsePathStyle,
	})
	publisher, err := productimage.NewS3AssetPublisher(productimage.S3AssetPublisherConfig{
		Uploader:   uploader,
		PublicBase: publicBase,
	})
	if err != nil {
		logger.WithError(err).Warn("s3 image asset publisher unavailable")
		return nil
	}
	return publisher
}
