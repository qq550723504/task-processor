package storage

import (
	"context"
	"errors"
	"os"
	"testing"

	"task-processor/internal/core/logger"

	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
)

func TestMain(m *testing.M) {
	logger.InitGlobalLogger(&logger.LogConfig{Level: "error", Console: false})
	os.Exit(m.Run())
}

func TestS3UploaderResolvedURLPrefersPublicBase(t *testing.T) {
	t.Parallel()

	uploader := NewS3UploaderWithOptions(nil, S3UploaderOptions{
		Bucket:     "listingkit-assets",
		PublicBase: "http://127.0.0.1:9100/listingkit-assets",
	})

	got := uploader.resolveObjectURL("20260419/example.jpg")
	want := "http://127.0.0.1:9100/listingkit-assets/20260419/example.jpg"
	if got != want {
		t.Fatalf("resolveObjectURL() = %q, want %q", got, want)
	}
}

func TestS3UploaderResolvedURLSupportsPathStyleEndpoint(t *testing.T) {
	t.Parallel()

	uploader := NewS3UploaderWithOptions(nil, S3UploaderOptions{
		Bucket:       "listingkit-assets",
		Endpoint:     "http://127.0.0.1:9100",
		UsePathStyle: true,
	})

	got := uploader.resolveObjectURL("20260419/example.jpg")
	want := "http://127.0.0.1:9100/listingkit-assets/20260419/example.jpg"
	if got != want {
		t.Fatalf("resolveObjectURL() = %q, want %q", got, want)
	}
}

func TestInspectObjectOnlyTreatsTypedNotFoundAsMissing(t *testing.T) {
	t.Parallel()

	uploader := NewS3UploaderWithAPI(&fakeS3API{headErr: &types.NotFound{}}, S3UploaderOptions{Bucket: "listingkit-assets"})
	inspection, err := uploader.InspectObject(context.Background(), "missing.png")
	if err != nil {
		t.Fatalf("InspectObject() error = %v", err)
	}
	if inspection.Exists {
		t.Fatal("InspectObject().Exists = true, want false")
	}

	uploader = NewS3UploaderWithAPI(&fakeS3API{headErr: errors.New("access denied")}, S3UploaderOptions{Bucket: "listingkit-assets"})
	if _, err := uploader.InspectObject(context.Background(), "missing.png"); err == nil {
		t.Fatal("InspectObject() error = nil, want non-not-found HEAD error")
	}
}

type fakeS3API struct {
	headErr error
}

func (f *fakeS3API) PutObject(context.Context, *s3.PutObjectInput, ...func(*s3.Options)) (*s3.PutObjectOutput, error) {
	return nil, errors.New("not implemented")
}

func (f *fakeS3API) HeadObject(context.Context, *s3.HeadObjectInput, ...func(*s3.Options)) (*s3.HeadObjectOutput, error) {
	return nil, f.headErr
}

func (f *fakeS3API) CopyObject(context.Context, *s3.CopyObjectInput, ...func(*s3.Options)) (*s3.CopyObjectOutput, error) {
	return nil, errors.New("not implemented")
}

func (f *fakeS3API) DeleteObject(context.Context, *s3.DeleteObjectInput, ...func(*s3.Options)) (*s3.DeleteObjectOutput, error) {
	return nil, errors.New("not implemented")
}
