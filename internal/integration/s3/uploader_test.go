package s3

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/aws/smithy-go"
)

func TestUploaderRejectsEmptyBucket(t *testing.T) {
	t.Parallel()

	_, err := NewUploaderWithOptions(&fakeS3API{}, UploaderOptions{})
	if err == nil || !strings.Contains(err.Error(), "bucket") {
		t.Fatalf("NewUploaderWithOptions() error = %v, want bucket validation", err)
	}
}

func TestUploaderRejectsNilClient(t *testing.T) {
	t.Parallel()

	_, err := NewUploaderWithOptions(nil, UploaderOptions{Bucket: "assets"})
	if err == nil || !strings.Contains(err.Error(), "client") {
		t.Fatalf("NewUploaderWithOptions() error = %v, want client validation", err)
	}
	var typedNil *s3.Client
	_, err = NewUploaderWithOptions(typedNil, UploaderOptions{Bucket: "assets"})
	if err == nil || !strings.Contains(err.Error(), "client") {
		t.Fatalf("NewUploaderWithOptions(typed nil) error = %v, want client validation", err)
	}
}

func TestUploadMultipleRejectsEmptyInputWithoutPanic(t *testing.T) {
	t.Parallel()

	uploader := mustNewUploader(t, &fakeS3API{}, UploaderOptions{Bucket: "assets"})
	urls, err := uploader.UploadMultiple(context.Background(), "batch", nil)
	if err == nil || urls != nil {
		t.Fatalf("UploadMultiple() = (%v, %v), want nil URLs and deterministic error", urls, err)
	}
}

func TestUploaderUsesConfiguredStructuredLogger(t *testing.T) {
	t.Parallel()

	recorder := &recordingLogger{}
	uploader := mustNewUploader(t, &fakeS3API{}, UploaderOptions{Bucket: "assets", Logger: recorder})
	if _, err := uploader.Upload(context.Background(), "folder/image.png", []byte("image"), "image/png"); err != nil {
		t.Fatalf("Upload() error = %v", err)
	}
	if len(recorder.infos) != 2 {
		t.Fatalf("logger info calls = %d, want 2", len(recorder.infos))
	}
	if got := recorder.infos[0].fields["bucket"]; got != "assets" {
		t.Fatalf("first log bucket = %v, want assets", got)
	}
	if got := recorder.infos[1].fields["object_key"]; got != "folder/image.png" {
		t.Fatalf("success log object_key = %v, want folder/image.png", got)
	}
}

func mustNewUploader(t *testing.T, client S3ObjectAPI, opts UploaderOptions) *Uploader {
	t.Helper()
	uploader, err := NewUploaderWithOptions(client, opts)
	if err != nil {
		t.Fatalf("NewUploaderWithOptions() error = %v", err)
	}
	return uploader
}

type recordedLog struct {
	message string
	fields  map[string]any
}

type recordingLogger struct {
	infos []recordedLog
}

func (*recordingLogger) Debug(string, map[string]any) {}
func (l *recordingLogger) Info(message string, fields map[string]any) {
	l.infos = append(l.infos, recordedLog{message: message, fields: fields})
}
func (*recordingLogger) Warn(string, map[string]any)  {}
func (*recordingLogger) Error(string, map[string]any) {}

func TestUploaderResolvedURLPrefersPublicBase(t *testing.T) {
	t.Parallel()

	uploader := mustNewUploader(t, &fakeS3API{}, UploaderOptions{
		Bucket:     "listingkit-assets",
		PublicBase: "http://127.0.0.1:9100/listingkit-assets",
	})

	got := uploader.resolveObjectURL("20260419/example.jpg")
	want := "http://127.0.0.1:9100/listingkit-assets/20260419/example.jpg"
	if got != want {
		t.Fatalf("resolveObjectURL() = %q, want %q", got, want)
	}
}

func TestUploaderResolvedURLSupportsPathStyleEndpoint(t *testing.T) {
	t.Parallel()

	uploader := mustNewUploader(t, &fakeS3API{}, UploaderOptions{
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

	uploader := mustNewUploader(t, &fakeS3API{headErr: &types.NotFound{}}, UploaderOptions{Bucket: "listingkit-assets", ArtifactCapabilities: ArtifactStorageCapabilities{Mode: ArtifactStorageModeAWS}})
	inspection, err := uploader.InspectObject(context.Background(), "missing.png")
	if err != nil {
		t.Fatalf("InspectObject() error = %v", err)
	}
	if inspection.Exists {
		t.Fatal("InspectObject().Exists = true, want false")
	}

	uploader = mustNewUploader(t, &fakeS3API{headErr: errors.New("access denied")}, UploaderOptions{Bucket: "listingkit-assets", ArtifactCapabilities: ArtifactStorageCapabilities{Mode: ArtifactStorageModeAWS}})
	if _, err := uploader.InspectObject(context.Background(), "missing.png"); err == nil {
		t.Fatal("InspectObject() error = nil, want non-not-found HEAD error")
	}
}

func TestInspectObjectClassifiesWrappedSmithyErrorsPrecisely(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name    string
		headErr error
		missing bool
	}{
		{name: "wrapped no such key", headErr: fmt.Errorf("wrapped: %w", &smithy.GenericAPIError{Code: "NoSuchKey"}), missing: true},
		{name: "wrapped not found", headErr: fmt.Errorf("wrapped: %w", &smithy.GenericAPIError{Code: "NotFound"}), missing: true},
		{name: "wrapped access denied", headErr: fmt.Errorf("wrapped: %w", &smithy.GenericAPIError{Code: "AccessDenied"})},
		{name: "wrapped server error", headErr: fmt.Errorf("wrapped: %w", &smithy.GenericAPIError{Code: "InternalError"})},
	} {
		t.Run(tc.name, func(t *testing.T) {
			uploader := mustNewUploader(t, &fakeS3API{headErr: tc.headErr}, UploaderOptions{Bucket: "listingkit-assets", ArtifactCapabilities: ArtifactStorageCapabilities{Mode: ArtifactStorageModeAWS}})
			inspection, err := uploader.InspectObject(context.Background(), "missing.png")
			if tc.missing {
				if err != nil || inspection.Exists {
					t.Fatalf("InspectObject() = (%+v, %v), want missing without error", inspection, err)
				}
				return
			}
			if err == nil {
				t.Fatal("InspectObject() error = nil, want propagated error")
			}
		})
	}
}

func TestExistsPreservesLegacyPlainHeadAndErrorSemantics(t *testing.T) {
	t.Parallel()
	api := &fakeS3API{headErr: errors.New("legacy access error")}
	uploader := mustNewUploader(t, api, UploaderOptions{Bucket: "listingkit-assets"})
	exists, err := uploader.Exists(context.Background(), "legacy/key.png")
	if err != nil || exists {
		t.Fatalf("Exists() = (%t, %v), want (false, nil)", exists, err)
	}
	if api.headInput == nil || api.headInput.ChecksumMode != "" {
		t.Fatalf("Exists() sent strict checksum mode: %+v", api.headInput)
	}
}

func TestArtifactStorageCapabilityModesSerializeRequiredHeaders(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name         string
		capabilities ArtifactStorageCapabilities
		wantChecksum bool
		wantCOS      bool
	}{
		{name: "aws", capabilities: ArtifactStorageCapabilities{Mode: ArtifactStorageModeAWS}, wantChecksum: true},
		{name: "cos", capabilities: ArtifactStorageCapabilities{Mode: ArtifactStorageModeCOS, COSImmutableNonVersionedBucketPolicy: true}, wantCOS: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			capture := &requestCapture{}
			uploader := mustNewUploader(t, capturingS3Client(capture), UploaderOptions{Bucket: "assets", ArtifactCapabilities: tc.capabilities})
			object := immutableObject("folder/source.png")
			if err := uploader.PutImmutable(context.Background(), object); err != nil {
				t.Fatalf("PutImmutable() error = %v", err)
			}
			if _, err := uploader.InspectObject(context.Background(), object.Key); err != nil {
				t.Fatalf("InspectObject() error = %v", err)
			}
			if err := uploader.CopyImmutable(context.Background(), ImmutableObjectCopy{SourceKey: object.Key, Destination: immutableObject("folder/destination.png")}); err != nil {
				t.Fatalf("CopyImmutable() error = %v", err)
			}
			put := capture.firstWithoutHeader("X-Amz-Copy-Source")
			head := capture.byMethod("HEAD")
			copy := capture.byHeader("X-Amz-Copy-Source")
			if put == nil || head == nil || copy == nil {
				t.Fatalf("captured requests missing: %+v", capture.requests)
			}
			if got := put.Header.Get("If-None-Match"); got != "*" {
				t.Fatalf("Put If-None-Match = %q, want *", got)
			}
			if tc.wantChecksum {
				if put.Header.Get("X-Amz-Checksum-Sha256") == "" || put.Header.Get("X-Amz-Sdk-Checksum-Algorithm") != "SHA256" || copy.Header.Get("X-Amz-Checksum-Algorithm") != "SHA256" || head.Header.Get("X-Amz-Checksum-Mode") != "ENABLED" {
					t.Fatalf("AWS checksum headers missing: put=%v head=%v copy=%v", put.Header, head.Header, copy.Header)
				}
				if put.Header.Get("X-Cos-Forbid-Overwrite") != "" {
					t.Fatalf("AWS request unexpectedly contains COS header: %v", put.Header)
				}
			} else {
				for _, request := range []*http.Request{put, head, copy} {
					if request.Header.Get("X-Amz-Checksum-Sha256") != "" || request.Header.Get("X-Amz-Sdk-Checksum-Algorithm") != "" || request.Header.Get("X-Amz-Checksum-Mode") != "" {
						t.Fatalf("COS request contains AWS checksum extension: %v", request.Header)
					}
					if request.Method != "HEAD" && request.Header.Get("X-Cos-Forbid-Overwrite") != "true" {
						t.Fatalf("COS immutable request lacks forbid-overwrite: %v", request.Header)
					}
				}
			}
		})
	}
}

func TestArtifactCOSFailsClosedWithoutImmutableNonVersionedPolicy(t *testing.T) {
	t.Parallel()
	api := &fakeS3API{}
	uploader := mustNewUploader(t, api, UploaderOptions{Bucket: "assets", ArtifactCapabilities: ArtifactStorageCapabilities{Mode: ArtifactStorageModeCOS}})
	if err := uploader.PutImmutable(context.Background(), immutableObject("object.png")); err == nil {
		t.Fatal("PutImmutable() error = nil, want explicit COS policy rejection")
	}
	if api.putCalls != 0 {
		t.Fatalf("PutObject calls = %d, want 0", api.putCalls)
	}
}

func TestCopySourceEscapesReservedAndUnicodeSegments(t *testing.T) {
	t.Parallel()
	got := copySource("bucket name", "nested/空 格+?#%.png")
	want := "bucket%20name/nested/%E7%A9%BA%20%E6%A0%BC%2B%3F%23%25.png"
	if got != want {
		t.Fatalf("copySource() = %q, want %q", got, want)
	}

	capture := &requestCapture{}
	uploader := mustNewUploader(t, capturingS3Client(capture), UploaderOptions{Bucket: "assets", ArtifactCapabilities: ArtifactStorageCapabilities{Mode: ArtifactStorageModeAWS}})
	if err := uploader.CopyImmutable(context.Background(), ImmutableObjectCopy{SourceKey: "nested/空 格+?#%.png", Destination: immutableObject("destination.png")}); err != nil {
		t.Fatalf("CopyImmutable() error = %v", err)
	}
	request := capture.byMethod("PUT")
	if request == nil || request.Header.Get("X-Amz-Copy-Source") != "assets/nested/%E7%A9%BA%20%E6%A0%BC%2B%3F%23%25.png" {
		t.Fatalf("serialized CopySource header = %q", request.Header.Get("X-Amz-Copy-Source"))
	}
}

type fakeS3API struct {
	headErr   error
	headInput *s3.HeadObjectInput
	putCalls  int
	putErr    error
}

func (f *fakeS3API) PutObject(context.Context, *s3.PutObjectInput, ...func(*s3.Options)) (*s3.PutObjectOutput, error) {
	f.putCalls++
	return &s3.PutObjectOutput{}, f.putErr
}

func (f *fakeS3API) HeadObject(_ context.Context, input *s3.HeadObjectInput, _ ...func(*s3.Options)) (*s3.HeadObjectOutput, error) {
	f.headInput = input
	return nil, f.headErr
}

func (f *fakeS3API) GetObject(context.Context, *s3.GetObjectInput, ...func(*s3.Options)) (*s3.GetObjectOutput, error) {
	return nil, errors.New("not implemented")
}

func (f *fakeS3API) CopyObject(context.Context, *s3.CopyObjectInput, ...func(*s3.Options)) (*s3.CopyObjectOutput, error) {
	return nil, errors.New("not implemented")
}

func (f *fakeS3API) DeleteObject(context.Context, *s3.DeleteObjectInput, ...func(*s3.Options)) (*s3.DeleteObjectOutput, error) {
	return nil, errors.New("not implemented")
}

type requestCapture struct {
	requests []*http.Request
}

func (c *requestCapture) RoundTrip(request *http.Request) (*http.Response, error) {
	captured := request.Clone(request.Context())
	captured.Header = request.Header.Clone()
	c.requests = append(c.requests, captured)
	headers := make(http.Header)
	if request.Method == http.MethodHead {
		headers.Set("Content-Length", "3")
		headers.Set("Content-Type", "image/png")
	}
	return &http.Response{StatusCode: http.StatusOK, Status: "200 OK", Header: headers, Body: io.NopCloser(strings.NewReader("<CopyObjectResult/>")), Request: request}, nil
}

func (c *requestCapture) byMethod(method string) *http.Request {
	for _, request := range c.requests {
		if request.Method == method {
			return request
		}
	}
	return nil
}

func (c *requestCapture) byHeader(name string) *http.Request {
	for _, request := range c.requests {
		if request.Header.Get(name) != "" {
			return request
		}
	}
	return nil
}

func (c *requestCapture) firstWithoutHeader(name string) *http.Request {
	for _, request := range c.requests {
		if request.Method == http.MethodPut && request.Header.Get(name) == "" {
			return request
		}
	}
	return nil
}

func capturingS3Client(c *requestCapture) *s3.Client {
	return s3.NewFromConfig(aws.Config{Region: "us-east-1", Credentials: aws.NewCredentialsCache(credentials.NewStaticCredentialsProvider("access", "secret", "")), HTTPClient: &http.Client{Transport: c}}, func(options *s3.Options) {
		options.BaseEndpoint = aws.String("https://objects.example")
		options.UsePathStyle = true
	})
}

func immutableObject(key string) ImmutableObjectPut {
	data := []byte("abc")
	sum := sha256.Sum256(data)
	return ImmutableObjectPut{Key: key, Data: data, ContentType: "image/png", SHA256: hex.EncodeToString(sum[:]), SizeBytes: int64(len(data))}
}
