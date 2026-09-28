package s3

import (
	"context"
	"encoding/base64"
	"encoding/hex"
	"strings"

	"task-processor/internal/knowledge"
)

// KnowledgeStore borrows the existing immutable S3 implementation. It never
// creates a public object URL or owns document lifecycle.
type KnowledgeStore struct{ uploader *Uploader }

func NewKnowledgeStore(uploader *Uploader) (*KnowledgeStore, error) {
	if uploader == nil || uploader.publicBase != "" {
		return nil, knowledge.ErrInvalid
	}
	if _, err := uploader.strictArtifactCapability(); err != nil {
		return nil, err
	}
	return &KnowledgeStore{uploader: uploader}, nil
}
func (s *KnowledgeStore) PutImmutable(ctx context.Context, o knowledge.Object) error {
	return s.uploader.PutImmutable(ctx, ImmutableObjectPut{Key: o.Key, Data: o.Data, ContentType: o.ContentType, SHA256: o.SHA256, SizeBytes: o.SizeBytes})
}
func (s *KnowledgeStore) Inspect(ctx context.Context, o knowledge.Object) (knowledge.Inspection, error) {
	i, err := s.uploader.InspectObject(ctx, o.Key)
	if err != nil {
		return knowledge.Inspection{}, knowledge.ErrUnavailable
	}
	return knowledgeInspection(i), nil
}
func knowledgeInspection(i ObjectInspection) knowledge.Inspection {
	digest := strings.ToLower(i.Metadata["sha256"])
	if i.ServerChecksumSHA256 != "" {
		decoded, err := base64.StdEncoding.DecodeString(i.ServerChecksumSHA256)
		if err != nil || len(decoded) != 32 {
			return knowledge.Inspection{Exists: i.Exists, SizeBytes: i.ContentLength}
		}
		server := hex.EncodeToString(decoded)
		if digest != "" && digest != server {
			digest = ""
		} else {
			digest = server
		}
	}
	return knowledge.Inspection{Exists: i.Exists, SHA256: digest, SizeBytes: i.ContentLength}
}
func (s *KnowledgeStore) ReadBounded(ctx context.Context, o knowledge.Object, max int64) ([]byte, error) {
	data, i, err := s.uploader.ReadObject(ctx, o.Key, max)
	if err != nil {
		return nil, knowledge.ErrUnavailable
	}
	inspection := knowledgeInspection(i)
	if !inspection.Exists || inspection.SHA256 != o.SHA256 || inspection.SizeBytes != o.SizeBytes || knowledge.Digest(data) != o.SHA256 {
		return nil, knowledge.ErrIntegrity
	}
	return data, nil
}
