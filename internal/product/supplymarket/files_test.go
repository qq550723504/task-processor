package supplymarket

import (
	"bytes"
	"context"
	"encoding/json"
	"image"
	"image/png"
	"task-processor/internal/product/collection"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

type privateObjects struct {
	files        map[string]PrivateObject
	loseResponse bool
}

func (s *privateObjects) PutImmutable(_ context.Context, file PrivateFile, data []byte) error {
	if s.files == nil {
		s.files = map[string]PrivateObject{}
	}
	s.files[file.ObjectKey] = PrivateObject{Exists: true, SHA256: file.SHA256, Size: file.Size, ContentType: file.ContentType, Data: append([]byte(nil), data...)}
	if s.loseResponse {
		return ErrUnknown
	}
	return nil
}
func (s *privateObjects) Inspect(_ context.Context, file PrivateFile) (PrivateObject, error) {
	r := s.files[file.ObjectKey]
	r.Data = nil
	return r, nil
}
func (s *privateObjects) ReadBounded(_ context.Context, file PrivateFile) ([]byte, error) {
	return append([]byte(nil), s.files[file.ObjectKey].Data...), nil
}

type privateFileRepository struct {
	FileRepository
	file *PrivateFile
}

func (r *privateFileRepository) SaveUpload(ctx context.Context, file PrivateFile, guard Guard) (PrivateFile, error) {
	if err := guard(ctx); err != nil {
		return PrivateFile{}, err
	}
	if r.file != nil {
		if r.file.ID != file.ID || r.file.SHA256 != file.SHA256 {
			return PrivateFile{}, ErrConflict
		}
		return *r.file, nil
	}
	r.file = &file
	return file, nil
}
func (r *privateFileRepository) ReadUpload(_ context.Context, scope collection.Scope, id string) (PrivateFile, error) {
	if r.file == nil || r.file.Owner != scope || r.file.ID != id {
		return PrivateFile{}, ErrNotFound
	}
	return *r.file, nil
}
func TestPrivateUploadCanVerifyLostStorageResponseWithoutPublishingOrMixingOwner(t *testing.T) {
	_, a, _, _, ctx, _ := serviceFixture(t)
	storage := &privateObjects{loseResponse: true}
	repository := &privateFileRepository{}
	files, err := NewFileService(a, storage, repository)
	require.NoError(t, err)
	var b bytes.Buffer
	require.NoError(t, png.Encode(&b, image.NewRGBA(image.Rect(0, 0, 2, 3))))
	key := uuid.NewString()
	saved, err := files.Upload(ctx, key, "image/png", b.Bytes())
	require.NoError(t, err)
	require.NotEmpty(t, saved.ID)
	require.Equal(t, a.scope, saved.Owner)
	require.Contains(t, saved.ObjectKey, saved.ID)
	serialized, err := json.Marshal(saved)
	require.NoError(t, err)
	require.NotContains(t, string(serialized), saved.ObjectKey)
	require.NotContains(t, string(serialized), a.scope.OrganizationID)
	_, err = files.Upload(ctx, key, "application/pdf", []byte("%PDF-1.7\n%%EOF"))
	require.ErrorIs(t, err, ErrConflict)
	a.scope = collection.Scope{"org-b", "actor-b", "member-b"}
	_, err = repository.ReadUpload(ctx, a.scope, saved.ID)
	require.ErrorIs(t, err, ErrNotFound)
}
func TestQualificationContentCannotClaimAnotherMediaType(t *testing.T) {
	_, a, _, _, ctx, _ := serviceFixture(t)
	storage := &privateObjects{}
	repository := &privateFileRepository{}
	files, err := NewFileService(a, storage, repository)
	require.NoError(t, err)
	_, err = files.Upload(ctx, uuid.NewString(), "image/png", []byte("%PDF-1.7\n%%EOF"))
	require.ErrorIs(t, err, ErrInvalid)
	require.Empty(t, storage.files)
	_, err = files.Upload(ctx, uuid.NewString(), "application/pdf", []byte("text pretending to be PDF"))
	require.ErrorIs(t, err, ErrInvalid)
	require.Empty(t, storage.files)
}
