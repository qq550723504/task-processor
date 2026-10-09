package productsourcing

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"github.com/stretchr/testify/require"
	"image"
	"image/png"
	s3integration "task-processor/internal/integration/s3"
	"task-processor/internal/product/collection"
	"testing"
)

type mediaAuth struct {
	scope collection.Scope
	err   error
}

func (a *mediaAuth) Authorize(context.Context, string) (collection.Scope, error) {
	return a.scope, a.err
}

type mediaStorage struct {
	values    map[string]s3integration.ImmutableObjectPut
	puts      int
	lost      bool
	readError error
}

func (s *mediaStorage) PutImmutable(_ context.Context, v s3integration.ImmutableObjectPut) error {
	s.puts++
	if _, ok := s.values[v.Key]; !ok {
		s.values[v.Key] = v
	}
	if s.lost {
		return errors.New("response lost")
	}
	return nil
}
func (s *mediaStorage) ReadObject(_ context.Context, key string, _ int64) ([]byte, s3integration.ObjectInspection, error) {
	if s.readError != nil {
		return nil, s3integration.ObjectInspection{}, s.readError
	}
	v, ok := s.values[key]
	return v.Data, s3integration.ObjectInspection{Exists: ok, ContentLength: v.SizeBytes, ContentType: v.ContentType}, nil
}
func (s *mediaStorage) PublicURL(key string) string { return "https://public.example.test/" + key }
func TestSourceFileIntegrityLostResponseAndPrivateRecovery(t *testing.T) {
	ctx := context.Background()
	var b bytes.Buffer
	require.NoError(t, png.Encode(&b, image.NewRGBA(image.Rect(0, 0, 2, 2))))
	raw := b.Bytes()
	hash := sha256.Sum256(raw)
	id := collection.MediaIdentity{Hash: hex.EncodeToString(hash[:]), Bytes: int64(len(raw))}
	auth := &mediaAuth{scope: collection.Scope{OrganizationID: "org", ActorID: "actor", MemberID: "member"}}
	storage := &mediaStorage{values: map[string]s3integration.ImmutableObjectPut{}, lost: true}
	service := SourceMedia{Storage: storage, Authorization: auth}
	result, err := service.Upload(ctx, id, raw)
	require.NoError(t, err)
	require.Equal(t, id, result.MediaIdentity)
	originalURL := result.URL
	_, err = service.Upload(ctx, id, []byte("not the original image"))
	require.Error(t, err)
	require.Equal(t, 1, storage.puts)
	storage.readError = errors.New("403 is not absent")
	_, err = service.Upload(ctx, id, raw)
	require.ErrorIs(t, err, collection.ErrUnknown)
	_, err = service.Read(ctx, id)
	require.ErrorIs(t, err, collection.ErrUnavailable)
	storage.readError = nil
	result, err = service.Read(ctx, id)
	require.NoError(t, err)
	require.Equal(t, originalURL, result.URL)
	auth.scope.ActorID = "other"
	_, err = service.Read(ctx, id)
	require.ErrorIs(t, err, collection.ErrNotFound)
	other, err := service.Upload(ctx, id, raw)
	require.NoError(t, err)
	require.NotEqual(t, originalURL, other.URL)
	auth.err = collection.ErrForbidden
	_, err = service.Read(ctx, id)
	require.ErrorIs(t, err, collection.ErrForbidden)
	key := mediaKey(collection.Scope{OrganizationID: "org", ActorID: "actor", MemberID: "member"}, id)
	v := storage.values[key]
	v.Data = []byte("corrupt")
	storage.values[key] = v
	auth.err = nil
	auth.scope.ActorID = "actor"
	_, err = service.Read(ctx, id)
	require.ErrorIs(t, err, collection.ErrConflict)
}
