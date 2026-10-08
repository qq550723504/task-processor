package s3

import (
	"context"
	"net/http"

	awshttp "github.com/aws/aws-sdk-go-v2/aws/transport/http"
	awss3 "github.com/aws/aws-sdk-go-v2/service/s3"

	"task-processor/internal/knowledge"
)

// KnowledgeStore borrows the existing immutable S3 implementation. It never
// creates a public object URL or owns document lifecycle.
type KnowledgeStore struct{ uploader *Uploader }

func NewKnowledgeClient(cfg ClientConfig) (*awss3.Client, error) {
	// Keep SDK transport defaults while pinning Knowledge requests to the
	// configured endpoint. A redirect must never forward private content or a
	// signed request; operator-local HTTP must not use an environment proxy.
	transport := awshttp.NewBuildableClient().GetTransport()
	transport.Proxy = nil
	cfg.HTTPClient = &http.Client{Transport: transport, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	return NewClient(cfg)
}

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
	return knowledge.Inspection{Exists: i.Exists, SHA256: privateInspectionDigest(i), SizeBytes: i.ContentLength}
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
