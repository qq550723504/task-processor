package currentapplication

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"
	"image"
	"image/png"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"sync"
	"task-processor/internal/app/productsourcing"
	"task-processor/internal/core/config"
	"task-processor/internal/product/collection"
	"testing"
)

func TestStandaloneSourceMediaUsesActualSignedImmutableS3PutAndReadback(t *testing.T) {
	var mu sync.Mutex
	objects := map[string][]byte{}
	puts := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		if r.Header.Get("Authorization") == "" {
			w.WriteHeader(403)
			return
		}
		data, exists := objects[r.URL.Path]
		switch r.Method {
		case http.MethodPut:
			if r.Header.Get("If-None-Match") != "*" || exists {
				w.WriteHeader(412)
				return
			}
			var e error
			data, e = io.ReadAll(io.LimitReader(r.Body, collection.MaxMediaBytes+1))
			if e != nil {
				w.WriteHeader(500)
				return
			}
			objects[r.URL.Path] = data
			puts++
			w.Header().Set("ETag", "\"immutable\"")
			w.WriteHeader(200)
		case http.MethodHead, http.MethodGet:
			if !exists {
				w.WriteHeader(404)
				return
			}
			w.Header().Set("Content-Type", "image/png")
			w.Header().Set("Content-Length", strconv.Itoa(len(data)))
			w.Header().Set("ETag", "\"immutable\"")
			w.WriteHeader(200)
			if r.Method == http.MethodGet {
				_, _ = w.Write(data)
			}
		default:
			w.WriteHeader(405)
		}
	}))
	defer server.Close()
	storage, e := NewSourceMediaStorage(config.ImageAgentArtifactStoreConfig{Enabled: true, Provider: "s3", PublicBase: "https://files.example.org/images", S3: config.ImageAgentArtifactStoreS3Config{Region: "us-east-1", Bucket: "source-products", Endpoint: server.URL, AccessKeyID: "synthetic", SecretAccessKey: "synthetic", UsePathStyle: true, ArtifactMode: "aws"}}, logrus.New())
	require.NoError(t, e)
	var b bytes.Buffer
	require.NoError(t, png.Encode(&b, image.NewRGBA(image.Rect(0, 0, 2, 2))))
	raw := b.Bytes()
	hash := sha256.Sum256(raw)
	id := collection.MediaIdentity{Hash: hex.EncodeToString(hash[:]), Bytes: int64(len(raw))}
	auth := &storageMediaAuth{scope: collection.Scope{"org", "actor", "member"}}
	s := productsourcing.SourceMedia{Storage: storage, Authorization: auth}
	v, e := s.Upload(context.Background(), id, raw)
	require.NoError(t, e)
	require.Equal(t, "image/png", v.MediaType)
	require.Contains(t, v.URL, "/product-source/")
	again, e := s.Upload(context.Background(), id, raw)
	require.NoError(t, e)
	require.Equal(t, v, again)
	require.Equal(t, 1, puts)
	auth.scope.ActorID = "other"
	_, e = s.Read(context.Background(), id)
	require.ErrorIs(t, e, collection.ErrNotFound)
}

type storageMediaAuth struct{ scope collection.Scope }

func (a *storageMediaAuth) Authorize(context.Context, string) (collection.Scope, error) {
	return a.scope, nil
}
