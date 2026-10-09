package productsourcing

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"regexp"
	"time"

	"task-processor/internal/integration/httpimage"
	s3integration "task-processor/internal/integration/s3"
	"task-processor/internal/product/collection"
)

type SourceMediaStorage interface {
	PutImmutable(context.Context, s3integration.ImmutableObjectPut) error
	ReadObject(context.Context, string, int64) ([]byte, s3integration.ObjectInspection, error)
	PublicURL(string) string
}
type SourceMedia struct {
	Storage       SourceMediaStorage
	Authorization collection.Authorizer
}

var mediaHash = regexp.MustCompile(`^[a-f0-9]{64}$`)

func (m SourceMedia) scope(ctx context.Context) (collection.Scope, error) {
	if m.Authorization == nil || m.Storage == nil {
		return collection.Scope{}, collection.ErrUnavailable
	}
	s, e := m.Authorization.Authorize(ctx, collection.PermissionManage)
	if e != nil {
		return s, e
	}
	return s, s.Validate()
}
func mediaKey(scope collection.Scope, id collection.MediaIdentity) string {
	return "product-source/" + collection.Digest([]string{scope.OrganizationID, scope.ActorID}) + "/" + id.Hash
}
func inspectSourceMedia(id collection.MediaIdentity, raw []byte) (collection.MediaImage, error) {
	if !mediaHash.MatchString(id.Hash) || id.Bytes < 1 || id.Bytes > collection.MaxMediaBytes || int64(len(raw)) != id.Bytes {
		return collection.MediaImage{}, collection.ErrInvalid
	}
	sum := sha256.Sum256(raw)
	if hex.EncodeToString(sum[:]) != id.Hash {
		return collection.MediaImage{}, collection.ErrConflict
	}
	mime, width, height, err := httpimage.InspectGeneratedArtifact(raw)
	if err != nil || (mime != "image/jpeg" && mime != "image/png") || width > 10000 || height > 10000 || int64(width)*int64(height) > 40000000 {
		return collection.MediaImage{}, collection.ErrInvalid
	}
	if _, _, err := image.Decode(bytes.NewReader(raw)); err != nil {
		return collection.MediaImage{}, collection.ErrInvalid
	}
	return collection.MediaImage{MediaIdentity: id, MediaType: mime, Width: width, Height: height}, nil
}
func (m SourceMedia) read(ctx context.Context, scope collection.Scope, id collection.MediaIdentity) (collection.MediaImage, error) {
	raw, meta, err := m.Storage.ReadObject(ctx, mediaKey(scope, id), collection.MaxMediaBytes)
	if err != nil {
		return collection.MediaImage{}, collection.ErrUnavailable
	}
	if !meta.Exists {
		return collection.MediaImage{}, collection.ErrNotFound
	}
	result, err := inspectSourceMedia(id, raw)
	if err != nil {
		return collection.MediaImage{}, collection.ErrConflict
	}
	if meta.ContentLength != id.Bytes || meta.ContentType != result.MediaType {
		return collection.MediaImage{}, collection.ErrConflict
	}
	url, err := httpimage.ValidatePublicHTTPSURL(m.Storage.PublicURL(mediaKey(scope, id)))
	if err != nil {
		return collection.MediaImage{}, collection.ErrUnavailable
	}
	result.URL = url
	fresh, err := m.scope(ctx)
	if err != nil || fresh != scope {
		return collection.MediaImage{}, collection.ErrForbidden
	}
	return result, nil
}
func (m SourceMedia) Read(ctx context.Context, id collection.MediaIdentity) (collection.MediaImage, error) {
	ctx, cancel := context.WithTimeout(ctx, 8*time.Second)
	defer cancel()
	scope, err := m.scope(ctx)
	if err != nil {
		return collection.MediaImage{}, err
	}
	if !mediaHash.MatchString(id.Hash) || id.Bytes < 1 || id.Bytes > collection.MaxMediaBytes {
		return collection.MediaImage{}, collection.ErrInvalid
	}
	return m.read(ctx, scope, id)
}
func (m SourceMedia) Upload(ctx context.Context, id collection.MediaIdentity, raw []byte) (collection.MediaImage, error) {
	ctx, cancel := context.WithTimeout(ctx, 8*time.Second)
	defer cancel()
	scope, err := m.scope(ctx)
	if err != nil {
		return collection.MediaImage{}, err
	}
	result, err := inspectSourceMedia(id, raw)
	if err != nil {
		return collection.MediaImage{}, err
	}
	// Validate the configured public URL before producing an object.
	if _, err = httpimage.ValidatePublicHTTPSURL(m.Storage.PublicURL(mediaKey(scope, id))); err != nil {
		return collection.MediaImage{}, collection.ErrUnavailable
	}
	err = m.Storage.PutImmutable(ctx, s3integration.ImmutableObjectPut{Key: mediaKey(scope, id), Data: raw, ContentType: result.MediaType, SHA256: id.Hash, SizeBytes: id.Bytes})
	verified, readErr := m.read(ctx, scope, id)
	if readErr == nil {
		return verified, nil
	} // including a verified identical immutable pre-existing object
	if err != nil || readErr != nil {
		return collection.MediaImage{}, collection.ErrUnknown
	}
	return verified, nil
}
