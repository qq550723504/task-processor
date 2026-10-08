package httpapi

import (
	"context"
	"gorm.io/gorm"
	"strings"
	"task-processor/internal/app/productsourcing"
	"task-processor/internal/authz"
	"task-processor/internal/core/config"
	"task-processor/internal/integration/httpimage"
	s3integration "task-processor/internal/integration/s3"
	kernelmodule "task-processor/internal/kernel/module"
	"task-processor/internal/product/collection"
	collectionhttp "task-processor/internal/product/collection/httpapi"
	"time"
)

func buildProductCollectionModuleWithSourceMedia(ctx context.Context, db *gorm.DB, dependencies routeAuthDependencies, permissions *authz.ListingKitAuthorizer, cfg *config.Config, supply bool) (kernelmodule.Module, error) {
	service, err := buildProductCollectionService(ctx, db, dependencies, permissions, supply)
	if err != nil {
		return nil, err
	}
	if cfg != nil && cfg.ImageAgent.ArtifactStore.Enabled {
		c := cfg.ImageAgent.ArtifactStore
		if c.Provider != "s3" || strings.TrimSpace(c.S3.Region) == "" || strings.TrimSpace(c.S3.AccessKeyID) == "" || strings.TrimSpace(c.S3.SecretAccessKey) == "" {
			return nil, collection.ErrUnavailable
		}
		if _, err := httpimage.ValidatePublicHTTPSURL(c.PublicBase); err != nil {
			return nil, collection.ErrUnavailable
		}
		capabilities := s3integration.ArtifactStorageCapabilities{Mode: s3integration.ArtifactStorageMode(c.S3.ArtifactMode), COSImmutableNonVersionedBucketPolicy: c.S3.COSImmutableNonVersionedBucketPolicy}
		if capabilities.Mode != s3integration.ArtifactStorageModeAWS && (capabilities.Mode != s3integration.ArtifactStorageModeCOS || !capabilities.COSImmutableNonVersionedBucketPolicy || c.S3.Endpoint == "") {
			return nil, collection.ErrUnavailable
		}
		client, err := s3integration.NewClient(s3integration.ClientConfig{Region: c.S3.Region, Endpoint: c.S3.Endpoint, AccessKeyID: c.S3.AccessKeyID, SecretAccessKey: c.S3.SecretAccessKey, UsePathStyle: c.S3.UsePathStyle})
		if err != nil {
			return nil, err
		}
		storage, err := s3integration.NewUploaderWithOptions(client, s3integration.UploaderOptions{Bucket: c.S3.Bucket, PublicBase: c.PublicBase, Endpoint: c.S3.Endpoint, UsePathStyle: c.S3.UsePathStyle, ArtifactCapabilities: capabilities})
		if err != nil {
			return nil, err
		}
		authority, err := collection.NewContextAuthorizer(&productReviewLiveOrganizationAccess{resolver: dependencies.organizationResolver, now: time.Now}, permissions)
		if err != nil {
			return nil, err
		}
		service.WithMedia(productsourcing.SourceMedia{Storage: storage, Authorization: authority})
	}
	binder := productReviewCapabilityBinder{now: time.Now}
	return productCollectionModule{routes: collectionhttp.Routes(service, binder.Bind)}, nil
}
