package main

import (
	"context"
	"encoding/base64"
	"github.com/sirupsen/logrus"
	"gorm.io/gorm"
	"task-processor/internal/app/runtime/currentapplication"
	"task-processor/internal/integration/s3"
	"task-processor/internal/integration/servicepayments"
)

func prepareEcoservices(ctx context.Context, db *gorm.DB, cfg *currentapplication.EcoservicesConfig, logger *logrus.Logger) (*currentapplication.EcoservicesRuntime, error) {
	c := cfg.Storage
	client, err := s3.NewKnowledgeClient(s3.ClientConfig{Region: c.Region, Endpoint: c.Endpoint, AccessKeyID: c.AccessKeyID, SecretAccessKey: c.SecretAccessKey, UsePathStyle: true})
	if err != nil {
		return nil, err
	}
	uploader, err := s3.NewUploaderWithOptions(client, s3.UploaderOptions{Logger: s3.AdaptLogrus(logger.WithField("component", "ecoservices-objects")), Bucket: c.Bucket, Endpoint: c.Endpoint, UsePathStyle: true, ArtifactCapabilities: s3.ArtifactStorageCapabilities{Mode: s3.ArtifactStorageMode(c.Mode), COSImmutableNonVersionedBucketPolicy: c.COSImmutableNonVersionedBucketPolicy}})
	if err != nil {
		return nil, err
	}
	objects, err := s3.NewEcoservicesStore(uploader)
	if err != nil {
		return nil, err
	}
	channel, err := servicepayments.NewWeChat(cfg.Payments.ChannelConfig())
	if err != nil {
		return nil, err
	}
	key, err := base64.StdEncoding.DecodeString(cfg.PayloadKey)
	if err != nil {
		return nil, err
	}
	protection, err := servicepayments.NewPayloadProtection(key)
	if err != nil {
		return nil, err
	}
	merchantProtection, err := servicepayments.NewMerchantPayloadProtection(key)
	if err != nil {
		return nil, err
	}
	return &currentapplication.EcoservicesRuntime{DB: db, Objects: objects, Channel: channel, Protection: protection, MerchantProtection: merchantProtection}, nil
}
