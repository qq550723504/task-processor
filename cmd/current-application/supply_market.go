package main

import (
	"context"
	"github.com/sirupsen/logrus"
	"task-processor/internal/app/runtime/currentapplication"
	"task-processor/internal/integration/s3"
	"task-processor/internal/product/supplymarket"
)

func prepareSupplyMarket(ctx context.Context, cfg *currentapplication.SupplyMarketConfig, logger *logrus.Logger) (supplymarket.PrivateFileStorage, error) {
	c := cfg.Storage
	client, e := s3.NewKnowledgeClient(s3.ClientConfig{Region: c.Region, Endpoint: c.Endpoint, AccessKeyID: c.AccessKeyID, SecretAccessKey: c.SecretAccessKey, UsePathStyle: true})
	if e != nil {
		return nil, e
	}
	uploader, e := s3.NewUploaderWithOptions(client, s3.UploaderOptions{Logger: s3.AdaptLogrus(logger.WithField("component", "supply-market-private-files")), Bucket: c.Bucket, Endpoint: c.Endpoint, UsePathStyle: true, ArtifactCapabilities: s3.ArtifactStorageCapabilities{Mode: s3.ArtifactStorageMode(c.Mode), COSImmutableNonVersionedBucketPolicy: c.COSImmutableNonVersionedBucketPolicy}})
	if e != nil {
		return nil, e
	}
	return s3.NewSupplyMarketFiles(uploader)
}
