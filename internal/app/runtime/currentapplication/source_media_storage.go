package currentapplication

import (
	"github.com/sirupsen/logrus"
	"net/url"
	"strings"
	"task-processor/internal/app/productsourcing"
	"task-processor/internal/core/config"
	"task-processor/internal/integration/httpimage"
	s3integration "task-processor/internal/integration/s3"
	"task-processor/internal/product/collection"
)

func ValidateSourceMediaStorage(c config.ImageAgentArtifactStoreConfig) error {
	if !c.Enabled || c.Provider != "s3" || strings.TrimSpace(c.S3.Region) == "" || strings.TrimSpace(c.S3.AccessKeyID) == "" || strings.TrimSpace(c.S3.SecretAccessKey) == "" || strings.TrimSpace(c.S3.Bucket) == "" || len(c.S3.Bucket) > 128 || c.IsolatedTrialGeneratedURLs {
		return collection.ErrUnavailable
	}
	parsed, e := url.Parse(c.PublicBase)
	if _, err := httpimage.ValidatePublicHTTPSURL(c.PublicBase); err != nil || e != nil || parsed.RawQuery != "" || parsed.Fragment != "" || parsed.Port() != "" && parsed.Port() != "443" {
		return collection.ErrUnavailable
	}
	if c.S3.Endpoint != "" {
		u, e := url.Parse(c.S3.Endpoint)
		if e != nil || u.Hostname() == "" || u.Scheme != "http" && u.Scheme != "https" || u.User != nil || u.RawQuery != "" || u.Fragment != "" {
			return collection.ErrUnavailable
		}
	}
	if c.S3.ArtifactMode != "aws" && (c.S3.ArtifactMode != "cos" || !c.S3.COSImmutableNonVersionedBucketPolicy || c.S3.Endpoint == "") {
		return collection.ErrUnavailable
	}
	return nil
}
func NewSourceMediaStorage(c config.ImageAgentArtifactStoreConfig, logger *logrus.Logger) (productsourcing.SourceMediaStorage, error) {
	if err := ValidateSourceMediaStorage(c); err != nil || logger == nil {
		return nil, collection.ErrUnavailable
	}
	client, err := s3integration.NewClient(s3integration.ClientConfig{Region: c.S3.Region, Endpoint: c.S3.Endpoint, AccessKeyID: c.S3.AccessKeyID, SecretAccessKey: c.S3.SecretAccessKey, UsePathStyle: c.S3.UsePathStyle})
	if err != nil {
		return nil, err
	}
	return s3integration.NewUploaderWithOptions(client, s3integration.UploaderOptions{Logger: s3integration.AdaptLogrus(logger.WithField("component", "product-source-media")), Bucket: c.S3.Bucket, PublicBase: c.PublicBase, Endpoint: c.S3.Endpoint, UsePathStyle: c.S3.UsePathStyle, ArtifactCapabilities: s3integration.ArtifactStorageCapabilities{Mode: s3integration.ArtifactStorageMode(c.S3.ArtifactMode), COSImmutableNonVersionedBucketPolicy: c.S3.COSImmutableNonVersionedBucketPolicy}})
}
