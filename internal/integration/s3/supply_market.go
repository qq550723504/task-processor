package s3

import (
	"context"
	"task-processor/internal/product/supplymarket"
)

// SupplyMarketFiles reuses immutable S3 primitives in a private bucket. It
// deliberately offers no public URL, delete, copy or general object-key API.
type SupplyMarketFiles struct{ uploader *Uploader }

func NewSupplyMarketFiles(u *Uploader) (*SupplyMarketFiles, error) {
	if u == nil || u.publicBase != "" {
		return nil, supplymarket.ErrUnavailable
	}
	if _, err := u.strictArtifactCapability(); err != nil {
		return nil, supplymarket.ErrUnavailable
	}
	return &SupplyMarketFiles{u}, nil
}
func (s *SupplyMarketFiles) PutImmutable(ctx context.Context, file supplymarket.PrivateFile, data []byte) error {
	if file.Validate() != nil {
		return supplymarket.ErrInvalid
	}
	return s.uploader.PutImmutable(ctx, ImmutableObjectPut{Key: file.ObjectKey, Data: data, ContentType: file.ContentType, SHA256: file.SHA256, SizeBytes: file.Size})
}
func (s *SupplyMarketFiles) Inspect(ctx context.Context, file supplymarket.PrivateFile) (supplymarket.PrivateObject, error) {
	if file.Validate() != nil {
		return supplymarket.PrivateObject{}, supplymarket.ErrInvalid
	}
	i, err := s.uploader.InspectObject(ctx, file.ObjectKey)
	if err != nil {
		return supplymarket.PrivateObject{}, supplymarket.ErrUnavailable
	}
	return supplymarket.PrivateObject{Exists: i.Exists, SHA256: privateInspectionDigest(i), Size: i.ContentLength, ContentType: i.ContentType}, nil
}
func (s *SupplyMarketFiles) ReadBounded(ctx context.Context, file supplymarket.PrivateFile) ([]byte, error) {
	if file.Validate() != nil {
		return nil, supplymarket.ErrInvalid
	}
	data, i, err := s.uploader.ReadObject(ctx, file.ObjectKey, supplymarket.MaxQualificationBytes)
	if err != nil || !i.Exists || i.ContentLength != file.Size || privateInspectionDigest(i) != file.SHA256 || i.ContentType != file.ContentType {
		return nil, supplymarket.ErrUnavailable
	}
	return data, nil
}
