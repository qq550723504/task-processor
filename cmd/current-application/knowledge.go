package main

import (
	"context"
	"github.com/sirupsen/logrus"
	"gorm.io/gorm"
	"task-processor/internal/app/runtime/currentapplication"
	"task-processor/internal/integration/documentparser/tika"
	store "task-processor/internal/integration/persistence/knowledge"
	"task-processor/internal/integration/s3"
	"task-processor/internal/knowledge"
)

func prepareKnowledge(ctx context.Context, db *gorm.DB, cfg *currentapplication.KnowledgeConfig, logger *logrus.Logger) (*knowledge.Service, *knowledge.Processor, error) {
	if err := store.VerifyRuntime(ctx, db); err != nil {
		return nil, nil, err
	}
	repo, err := store.NewRepository(ctx, db)
	if err != nil {
		return nil, nil, err
	}
	c := cfg.Storage
	client, err := s3.NewKnowledgeClient(s3.ClientConfig{Region: c.Region, Endpoint: c.Endpoint, AccessKeyID: c.AccessKeyID, SecretAccessKey: c.SecretAccessKey, UsePathStyle: true})
	if err != nil {
		return nil, nil, err
	}
	uploader, err := s3.NewUploaderWithOptions(client, s3.UploaderOptions{Logger: s3.AdaptLogrus(logger.WithField("component", "knowledge-objects")), Bucket: c.Bucket, Endpoint: c.Endpoint, UsePathStyle: true, ArtifactCapabilities: s3.ArtifactStorageCapabilities{Mode: s3.ArtifactStorageMode(c.Mode), COSImmutableNonVersionedBucketPolicy: c.COSImmutableNonVersionedBucketPolicy}})
	if err != nil {
		return nil, nil, err
	}
	objects, err := s3.NewKnowledgeStore(uploader)
	if err != nil {
		return nil, nil, err
	}
	parser, err := tika.New(cfg.ParserEndpoint)
	if err != nil {
		return nil, nil, err
	}
	service, err := knowledge.NewService(repo, objects)
	if err != nil {
		return nil, nil, err
	}
	processor, err := knowledge.NewProcessor(repo, objects, parser)
	return service, processor, err
}
