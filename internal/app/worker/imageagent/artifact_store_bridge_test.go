package imageagentworker

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"task-processor/internal/imageagent/objectstore"
	s3integration "task-processor/internal/integration/s3"
)

func TestWorkerArtifactStoreConvertsDomainObjectsWithoutLeakingS3Types(t *testing.T) {
	t.Parallel()

	uploader := &recordingS3Uploader{
		inspection: s3integration.ObjectInspection{Exists: true, ContentLength: 3, ContentType: "image/png", Metadata: map[string]string{"sha256": "abc"}, ServerChecksumSHA256: "checksum", ETag: "etag"},
		readData:   []byte("abc"),
	}
	bridge := workerArtifactStore{uploader: uploader}
	put := objectstore.ImmutableObjectPut{Key: "folder/source.png", Data: []byte("abc"), ContentType: "image/png", SHA256: "abc", SizeBytes: 3}
	if err := bridge.PutImmutable(context.Background(), put); err != nil {
		t.Fatalf("PutImmutable() error = %v", err)
	}
	if want := (s3integration.ImmutableObjectPut{Key: put.Key, Data: put.Data, ContentType: put.ContentType, SHA256: put.SHA256, SizeBytes: put.SizeBytes}); !reflect.DeepEqual(uploader.put, want) {
		t.Fatalf("converted put = %+v, want %+v", uploader.put, want)
	}

	inspection, err := bridge.InspectObject(context.Background(), put.Key)
	if err != nil {
		t.Fatalf("InspectObject() error = %v", err)
	}
	wantInspection := objectstore.ObjectInspection{Exists: true, ContentLength: 3, ContentType: "image/png", Metadata: map[string]string{"sha256": "abc"}, ServerChecksumSHA256: "checksum", ETag: "etag"}
	if !reflect.DeepEqual(inspection, wantInspection) {
		t.Fatalf("converted inspection = %+v, want %+v", inspection, wantInspection)
	}
	data, readInspection, err := bridge.ReadObject(context.Background(), put.Key, 64)
	if err != nil || string(data) != "abc" || !reflect.DeepEqual(readInspection, wantInspection) {
		t.Fatalf("ReadObject() = (%q, %+v, %v), want bytes and complete inspection", data, readInspection, err)
	}

	copyInput := objectstore.ImmutableObjectCopy{SourceKey: put.Key, Destination: put}
	if err := bridge.CopyImmutable(context.Background(), copyInput); err != nil {
		t.Fatalf("CopyImmutable() error = %v", err)
	}
	if uploader.copy.SourceKey != copyInput.SourceKey || uploader.copy.Destination.Key != copyInput.Destination.Key {
		t.Fatalf("converted copy = %+v, want source %q destination %q", uploader.copy, copyInput.SourceKey, copyInput.Destination.Key)
	}
	if got := bridge.PublicURL("folder/source.png"); got != "https://cdn.example.test/folder/source.png" {
		t.Fatalf("PublicURL() = %q", got)
	}
}

func TestWorkerArtifactStorePropagatesUploaderErrors(t *testing.T) {
	t.Parallel()

	want := errors.New("s3 unavailable")
	uploader := &recordingS3Uploader{err: want}
	bridge := workerArtifactStore{uploader: uploader}
	if _, err := bridge.InspectObject(context.Background(), "object"); !errors.Is(err, want) {
		t.Fatalf("InspectObject() error = %v, want %v", err, want)
	}
	if _, _, err := bridge.ReadObject(context.Background(), "object", 64); !errors.Is(err, want) {
		t.Fatalf("ReadObject() error = %v, want %v", err, want)
	}
	if err := bridge.PutImmutable(context.Background(), objectstore.ImmutableObjectPut{Key: "object"}); !errors.Is(err, want) {
		t.Fatalf("PutImmutable() error = %v, want %v", err, want)
	}
	if err := bridge.CopyImmutable(context.Background(), objectstore.ImmutableObjectCopy{SourceKey: "object"}); !errors.Is(err, want) {
		t.Fatalf("CopyImmutable() error = %v, want %v", err, want)
	}
}

type recordingS3Uploader struct {
	put        s3integration.ImmutableObjectPut
	copy       s3integration.ImmutableObjectCopy
	inspection s3integration.ObjectInspection
	readData   []byte
	err        error
}

func (*recordingS3Uploader) PublicURL(key string) string {
	return "https://cdn.example.test/" + key
}
func (u *recordingS3Uploader) InspectObject(context.Context, string) (s3integration.ObjectInspection, error) {
	return u.inspection, u.err
}
func (u *recordingS3Uploader) ReadObject(context.Context, string, int64) ([]byte, s3integration.ObjectInspection, error) {
	return u.readData, u.inspection, u.err
}
func (u *recordingS3Uploader) PutImmutable(_ context.Context, input s3integration.ImmutableObjectPut) error {
	u.put = input
	return u.err
}
func (u *recordingS3Uploader) CopyImmutable(_ context.Context, input s3integration.ImmutableObjectCopy) error {
	u.copy = input
	return u.err
}
