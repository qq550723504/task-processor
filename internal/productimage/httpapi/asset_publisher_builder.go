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

type assetPublisherOptions struct {
	enabled    bool
	provider   string
	outputDir  string
	publicBase string
	s3         s3AssetPublisherOptions
	amazon     productimage.AmazonAssetPublisherOptions
}

type s3AssetPublisherOptions struct {
	bucket          string
	region          string
	endpoint        string
	accessKeyID     string
	secretAccessKey string
	usePathStyle    bool
}

func newAssetPublisherOptions(cfg *config.Config) assetPublisherOptions {
	if cfg == nil {
		return assetPublisherOptions{}
	}

	publisherCfg := cfg.ProductImage.Publisher
	spapiCfg := cfg.Amazon.SPAPI
	return assetPublisherOptions{
		enabled:    publisherCfg.Enabled,
		provider:   publisherCfg.Provider,
		outputDir:  publisherCfg.OutputDir,
		publicBase: publisherCfg.PublicBase,
		s3: s3AssetPublisherOptions{
			bucket:          publisherCfg.S3.Bucket,
			region:          publisherCfg.S3.Region,
			endpoint:        publisherCfg.S3.Endpoint,
			accessKeyID:     publisherCfg.S3.AccessKeyID,
			secretAccessKey: publisherCfg.S3.SecretAccessKey,
			usePathStyle:    publisherCfg.S3.UsePathStyle,
		},
		amazon: productimage.AmazonAssetPublisherOptions{
			Enabled:        spapiCfg.Enabled,
			Region:         spapiCfg.Region,
			MarketplaceID:  config.ResolveAmazonMarketplaceID(spapiCfg),
			ClientID:       spapiCfg.ClientID,
			ClientSecret:   spapiCfg.ClientSecret,
			RefreshToken:   spapiCfg.RefreshToken,
			AWSAccessKeyID: spapiCfg.AWSAccessKeyID,
			AWSSecretKey:   spapiCfg.AWSSecretKey,
		},
	}
}

func buildAssetPublisher(options assetPublisherOptions, logger *logrus.Logger) productimage.AssetPublisher {
	if !options.enabled {
		return nil
	}

	provider := strings.ToLower(strings.TrimSpace(options.provider))
	switch provider {
	case "", "local":
		publisher, err := productimage.NewLocalAssetPublisher(options.outputDir, options.publicBase)
		if err != nil {
			logger.WithError(err).Warn("local image asset publisher unavailable")
			return nil
		}
		return publisher
	case "s3":
		return buildS3AssetPublisher(options, logger)
	case "amazon":
		publisher, err := productimage.NewAmazonAssetPublisher(options.amazon)
		if err != nil {
			logger.WithError(err).Warn("amazon image asset publisher unavailable")
			return nil
		}
		return publisher
	case "hybrid":
		localPublisher, err := productimage.NewLocalAssetPublisher(options.outputDir, options.publicBase)
		if err != nil {
			logger.WithError(err).Warn("hybrid local image asset publisher unavailable")
			return nil
		}
		amazonPublisher, err := productimage.NewAmazonAssetPublisher(options.amazon)
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

func newPublisherS3Client(options s3AssetPublisherOptions) (*s3.Client, error) {
	if strings.TrimSpace(options.bucket) == "" {
		return nil, fmt.Errorf("productimage.publisher.s3.bucket cannot be empty")
	}
	return storageinfra.NewS3Client(storageinfra.S3ClientConfig{
		Region:          options.region,
		Endpoint:        options.endpoint,
		AccessKeyID:     options.accessKeyID,
		SecretAccessKey: options.secretAccessKey,
		UsePathStyle:    options.usePathStyle,
	})
}

func buildS3AssetPublisher(options assetPublisherOptions, logger *logrus.Logger) productimage.AssetPublisher {
	client, err := newPublisherS3Client(options.s3)
	if err != nil {
		logger.WithError(err).Warn("s3 image asset publisher unavailable")
		return nil
	}

	publicBase := strings.TrimSpace(options.publicBase)
	if publicBase == "" {
		publicBase = storageinfra.BuildS3PublicBase(
			options.s3.endpoint,
			options.s3.bucket,
			options.s3.usePathStyle,
		)
	}

	uploader := storageinfra.NewS3UploaderWithOptions(client, storageinfra.S3UploaderOptions{
		Bucket:       options.s3.bucket,
		PublicBase:   publicBase,
		Endpoint:     options.s3.endpoint,
		UsePathStyle: options.s3.usePathStyle,
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
