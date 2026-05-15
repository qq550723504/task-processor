package listingkit

import (
	"bytes"
	"context"
	"io"
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

func TestS3ImageUploadStoreSaveUsesPublicBase(t *testing.T) {
	t.Parallel()

	store, err := NewS3ImageUploadStore(S3ImageUploadStoreConfig{
		Bucket:     "listingkit-inputs",
		PublicBase: "https://cdn.example.com/listingkit-inputs",
		Uploader: &stubS3ImageUploadUploader{
			url: "https://listingkit-inputs.s3.amazonaws.com/20260419/image.jpg",
		},
		Reader:  &stubS3ImageUploadReader{},
		Deleter: &stubS3ImageUploadDeleter{},
	})
	if err != nil {
		t.Fatalf("NewS3ImageUploadStore() error = %v", err)
	}

	file, err := store.Save(context.Background(), &ImageUploadInput{
		Filename:    "shirt.png",
		ContentType: "image/png",
		Data:        []byte{0x89, 0x50, 0x4E, 0x47},
	})
	if err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	if file == nil {
		t.Fatal("Save() returned nil file")
	}
	if file.Key == "" {
		t.Fatal("Save() returned empty key")
	}
	if file.ContentType != "image/png" {
		t.Fatalf("content type = %q, want image/png", file.ContentType)
	}
	if file.PublicURL == "" {
		t.Fatal("PublicURL = empty, want CDN URL")
	}
	if !strings.HasPrefix(file.PublicURL, "https://cdn.example.com/listingkit-inputs/") {
		t.Fatalf("PublicURL = %q, want publicBase-derived URL", file.PublicURL)
	}
}

func TestS3ImageUploadStoreOpenReadsObjectData(t *testing.T) {
	t.Parallel()

	store, err := NewS3ImageUploadStore(S3ImageUploadStoreConfig{
		Bucket: "listingkit-inputs",
		Uploader: &stubS3ImageUploadUploader{
			url: "https://listingkit-inputs.s3.amazonaws.com/20260419/image.jpg",
		},
		Reader: &stubS3ImageUploadReader{
			output: &s3.GetObjectOutput{
				Body:          io.NopCloser(bytes.NewReader([]byte("image-bytes"))),
				ContentType:   aws.String("image/jpeg"),
				ContentLength: aws.Int64(11),
			},
		},
		Deleter: &stubS3ImageUploadDeleter{},
	})
	if err != nil {
		t.Fatalf("NewS3ImageUploadStore() error = %v", err)
	}

	opened, err := store.Open(context.Background(), "20260419/example.jpg")
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	if opened == nil {
		t.Fatal("Open() returned nil file")
	}
	if string(opened.Data) != "image-bytes" {
		t.Fatalf("Data = %q, want image-bytes", string(opened.Data))
	}
	if opened.ContentType != "image/jpeg" {
		t.Fatalf("content type = %q, want image/jpeg", opened.ContentType)
	}
}

func TestS3ImageUploadStoreDeleteDeletesObject(t *testing.T) {
	t.Parallel()

	deleter := &stubS3ImageUploadDeleter{}
	store, err := NewS3ImageUploadStore(S3ImageUploadStoreConfig{
		Bucket:   "listingkit-inputs",
		Uploader: &stubS3ImageUploadUploader{},
		Reader:   &stubS3ImageUploadReader{},
		Deleter:  deleter,
	})
	if err != nil {
		t.Fatalf("NewS3ImageUploadStore() error = %v", err)
	}

	if err := store.Delete(context.Background(), "/20260419/example.jpg"); err != nil {
		t.Fatalf("Delete() error = %v", err)
	}
	if deleter.deletedKey != "20260419/example.jpg" {
		t.Fatalf("deleted key = %q, want normalized key", deleter.deletedKey)
	}
}

type stubS3ImageUploadUploader struct {
	url      string
	lastKey  string
	lastData []byte
}

func (s *stubS3ImageUploadUploader) Upload(_ context.Context, key string, data []byte, _ string) (string, error) {
	s.lastKey = key
	s.lastData = append([]byte(nil), data...)
	return s.url, nil
}

type stubS3ImageUploadReader struct {
	output *s3.GetObjectOutput
}

func (s *stubS3ImageUploadReader) GetObject(_ context.Context, _ *s3.GetObjectInput, _ ...func(*s3.Options)) (*s3.GetObjectOutput, error) {
	return s.output, nil
}

type stubS3ImageUploadDeleter struct {
	deletedKey string
}

func (s *stubS3ImageUploadDeleter) DeleteObject(_ context.Context, input *s3.DeleteObjectInput, _ ...func(*s3.Options)) (*s3.DeleteObjectOutput, error) {
	if input.Key != nil {
		s.deletedKey = *input.Key
	}
	return &s3.DeleteObjectOutput{}, nil
}
