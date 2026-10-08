package s3

import (
	"context"
	"encoding/base64"
	"encoding/hex"
	"strings"
	e "task-processor/internal/ecoservices"
)

// EcoservicesStore reuses immutable S3 leaves without Knowledge lifecycle or
// public URL authority. The runtime credentials must cover only this prefix.
type EcoservicesStore struct{ uploader *Uploader }

func NewEcoservicesStore(u *Uploader) (*EcoservicesStore, error) {
	if u == nil || u.publicBase != "" {
		return nil, e.ErrUnavailable
	}
	if _, err := u.strictArtifactCapability(); err != nil {
		return nil, err
	}
	return &EcoservicesStore{uploader: u}, nil
}
func validServiceObject(file e.File) bool {
	return e.ValidID(file.ID) && file.ObjectKey == "ecoservices/files/"+file.ID && file.SizeBytes > 0 && file.SizeBytes <= e.MaxFileBytes
}
func (s *EcoservicesStore) Inspect(ctx context.Context, file e.File) (e.ObjectInspection, error) {
	if !validServiceObject(file) {
		return e.ObjectInspection{}, e.ErrInvalid
	}
	v, err := s.uploader.InspectObject(ctx, file.ObjectKey)
	if err != nil {
		return e.ObjectInspection{}, e.ErrUnavailable
	}
	return e.ObjectInspection{Exists: v.Exists, ContentType: v.ContentType, SHA256: privateInspectionDigest(v), SizeBytes: v.ContentLength}, nil
}
func (s *EcoservicesStore) PutImmutable(ctx context.Context, file e.File, data []byte) error {
	if !validServiceObject(file) {
		return e.ErrInvalid
	}
	return s.uploader.PutImmutable(ctx, ImmutableObjectPut{Key: file.ObjectKey, Data: data, ContentType: file.ContentType, SHA256: file.SHA256, SizeBytes: file.SizeBytes})
}
func (s *EcoservicesStore) ReadBounded(ctx context.Context, file e.File) ([]byte, error) {
	if !validServiceObject(file) {
		return nil, e.ErrInvalid
	}
	data, inspection, err := s.uploader.ReadObject(ctx, file.ObjectKey, e.MaxFileBytes)
	if err != nil {
		return nil, e.ErrUnavailable
	}
	if !inspection.Exists || inspection.ContentLength != file.SizeBytes || privateInspectionDigest(inspection) != file.SHA256 || e.FileDigest(data) != file.SHA256 {
		return nil, e.ErrConflict
	}
	return data, nil
}
func privateInspectionDigest(v ObjectInspection) string {
	digest := strings.ToLower(v.Metadata["sha256"])
	if v.ServerChecksumSHA256 != "" {
		decoded, err := base64.StdEncoding.DecodeString(v.ServerChecksumSHA256)
		if err != nil || len(decoded) != 32 {
			return ""
		}
		server := hex.EncodeToString(decoded)
		if digest != "" && digest != server {
			return ""
		}
		digest = server
	}
	return digest
}
