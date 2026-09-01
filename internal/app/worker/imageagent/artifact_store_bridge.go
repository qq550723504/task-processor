package imageagentworker

import (
	"context"

	"task-processor/internal/imageagent/objectstore"
	s3integration "task-processor/internal/integration/s3"
)

type s3ImmutableUploader interface {
	PublicURL(string) string
	InspectObject(context.Context, string) (s3integration.ObjectInspection, error)
	ReadObject(context.Context, string, int64) ([]byte, s3integration.ObjectInspection, error)
	PutImmutable(context.Context, s3integration.ImmutableObjectPut) error
	CopyImmutable(context.Context, s3integration.ImmutableObjectCopy) error
}

type workerArtifactStore struct {
	uploader s3ImmutableUploader
}

func (s workerArtifactStore) PublicURL(key string) string {
	return s.uploader.PublicURL(key)
}

func (s workerArtifactStore) InspectObject(ctx context.Context, key string) (objectstore.ObjectInspection, error) {
	inspection, err := s.uploader.InspectObject(ctx, key)
	return objectstore.ObjectInspection{
		Exists: inspection.Exists, ContentLength: inspection.ContentLength, ContentType: inspection.ContentType,
		Metadata: inspection.Metadata, ServerChecksumSHA256: inspection.ServerChecksumSHA256, ETag: inspection.ETag,
	}, err
}

func (s workerArtifactStore) ReadObject(ctx context.Context, key string, maxBytes int64) ([]byte, objectstore.ObjectInspection, error) {
	data, inspection, err := s.uploader.ReadObject(ctx, key, maxBytes)
	return data, objectstore.ObjectInspection{
		Exists: inspection.Exists, ContentLength: inspection.ContentLength, ContentType: inspection.ContentType,
		Metadata: inspection.Metadata, ServerChecksumSHA256: inspection.ServerChecksumSHA256, ETag: inspection.ETag,
	}, err
}

func (s workerArtifactStore) PutImmutable(ctx context.Context, object objectstore.ImmutableObjectPut) error {
	return s.uploader.PutImmutable(ctx, s3integration.ImmutableObjectPut{
		Key: object.Key, Data: object.Data, ContentType: object.ContentType, SHA256: object.SHA256, SizeBytes: object.SizeBytes,
	})
}

func (s workerArtifactStore) CopyImmutable(ctx context.Context, object objectstore.ImmutableObjectCopy) error {
	return s.uploader.CopyImmutable(ctx, s3integration.ImmutableObjectCopy{
		SourceKey: object.SourceKey,
		Destination: s3integration.ImmutableObjectPut{
			Key: object.Destination.Key, Data: object.Destination.Data, ContentType: object.Destination.ContentType,
			SHA256: object.Destination.SHA256, SizeBytes: object.Destination.SizeBytes,
		},
	})
}

var _ objectstore.ImmutableObjectStore = workerArtifactStore{}
