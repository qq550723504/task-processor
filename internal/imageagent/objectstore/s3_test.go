package objectstore

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"strings"
	"testing"

	"task-processor/internal/core/logger"
	"task-processor/internal/imageagent"
	"task-processor/internal/infra/storage"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
)

func TestMain(m *testing.M) {
	logger.InitGlobalLogger(&logger.LogConfig{Level: "error", Console: false})
	os.Exit(m.Run())
}

func TestPrepareSlotArtifactsBuildsContentAddressedManifestWithoutLocalPath(t *testing.T) {
	t.Parallel()

	store := newTestStore(t, &fakeS3API{})
	prepared, err := store.PrepareSlotArtifacts(PrepareSlotArtifactsInput{
		Identity: testIdentity(),
		Assets: []ArtifactInput{{
			Bytes:             []byte("png bytes"),
			ContentType:       "image/png",
			Width:             1200,
			Height:            800,
			SourceAssetID:     "source-1",
			Operations:        []string{"resize"},
			ProviderReceiptID: "receipt-1",
		}},
	})
	if err != nil {
		t.Fatalf("PrepareSlotArtifacts() error = %v", err)
	}

	wantSHA := sha256Hex([]byte("png bytes"))
	if got, want := prepared.Manifest.Assets[0].ObjectKey, "image-agent/staging/tenant-a/run-1/3/slot-1/2/0-"+wantSHA+".png"; got != want {
		t.Fatalf("object key = %q, want %q", got, want)
	}
	encoded, err := json.Marshal(prepared)
	if err != nil {
		t.Fatalf("marshal prepared artifacts: %v", err)
	}
	if strings.Contains(string(encoded), "png bytes") || strings.Contains(string(encoded), "local_path") {
		t.Fatalf("persisted JSON leaked transient material: %s", encoded)
	}
}

func TestEnsureStagedReconcilesLostPutResponseWithHead(t *testing.T) {
	t.Parallel()

	api := &fakeS3API{objects: map[string]fakeObject{}}
	api.put = func(input *s3.PutObjectInput) error {
		api.savePut(input)
		return errors.New("response lost after object write")
	}
	store := newTestStore(t, api)
	prepared := mustPrepare(t, store)

	if err := store.EnsureStaged(context.Background(), prepared); err != nil {
		t.Fatalf("EnsureStaged() error = %v", err)
	}
	if api.putCalls != 1 || api.headCalls < 1 {
		t.Fatalf("calls: put=%d head=%d, want a put followed by HEAD reconciliation", api.putCalls, api.headCalls)
	}
}

func TestEnsureStagedRejectsSameKeyWithDifferentMetadata(t *testing.T) {
	t.Parallel()

	api := &fakeS3API{objects: map[string]fakeObject{}}
	store := newTestStore(t, api)
	prepared := mustPrepare(t, store)
	ref := prepared.Manifest.Assets[0]
	api.objects[ref.ObjectKey] = fakeObject{contentType: ref.ContentType, contentLength: ref.SizeBytes, metadata: map[string]string{"sha256": strings.Repeat("a", 64), "size-bytes": "9"}}

	if err := store.EnsureStaged(context.Background(), prepared); err == nil {
		t.Fatal("EnsureStaged() error = nil, want conflicting object error")
	}
	if api.putCalls != 0 {
		t.Fatalf("put calls = %d, want 0 for an existing conflicting object", api.putCalls)
	}
}

func TestFinalizeUsesDeterministicPublicKey(t *testing.T) {
	t.Parallel()

	api := &fakeS3API{objects: map[string]fakeObject{}}
	store := newTestStore(t, api)
	prepared := mustPrepare(t, store)
	staged := prepared.Manifest.Assets[0]
	api.objects[staged.ObjectKey] = fakeObject{contentType: staged.ContentType, contentLength: staged.SizeBytes, metadata: map[string]string{"sha256": staged.SHA256, "size-bytes": "9"}}

	final, err := store.Finalize(context.Background(), prepared.Manifest)
	if err != nil {
		t.Fatalf("Finalize() error = %v", err)
	}
	want := strings.Replace(staged.ObjectKey, "image-agent/staging/", "image-agent/public/", 1)
	if got := final.Assets[0].ObjectKey; got != want {
		t.Fatalf("final object key = %q, want %q", got, want)
	}
	if api.copyCalls != 1 {
		t.Fatalf("copy calls = %d, want 1", api.copyCalls)
	}
}

func TestInspectNeverTreatsETagAsSHA256(t *testing.T) {
	t.Parallel()

	api := &fakeS3API{objects: map[string]fakeObject{}}
	store := newTestStore(t, api)
	prepared := mustPrepare(t, store)
	ref := prepared.Manifest.Assets[0]
	api.objects[ref.ObjectKey] = fakeObject{contentType: ref.ContentType, contentLength: ref.SizeBytes, eTag: ref.SHA256, metadata: map[string]string{"sha256": strings.Repeat("b", 64), "size-bytes": "9"}}

	if err := store.EnsureStaged(context.Background(), prepared); err == nil {
		t.Fatal("EnsureStaged() error = nil, want ETag to be ignored as a SHA-256 proof")
	}
}

func TestPrepareRejectsOversizeUnsupportedOrEscapingArtifacts(t *testing.T) {
	t.Parallel()

	store := newTestStore(t, &fakeS3API{})
	for _, tc := range []struct {
		name  string
		input PrepareSlotArtifactsInput
	}{
		{name: "oversize", input: prepareInputWith(testIdentity(), ArtifactInput{Bytes: []byte("0123456789"), ContentType: "image/png", Width: 1, Height: 1, SourceAssetID: "source-1"})},
		{name: "unsupported type", input: prepareInputWith(testIdentity(), ArtifactInput{Bytes: []byte("jpeg"), ContentType: "image/gif", Width: 1, Height: 1, SourceAssetID: "source-1"})},
		{name: "escaping run ID", input: prepareInputWith(imageagent.SlotExternalEffectIdentity{RunScope: imageagent.RunScope{TenantID: "tenant-a", OwnerUserID: "user-a", RunID: "../run"}, PlanRevision: 3, SlotID: "slot-1", Attempt: 2}, ArtifactInput{Bytes: []byte("png bytes"), ContentType: "image/png", Width: 1, Height: 1, SourceAssetID: "source-1"})},
		{name: "local provider receipt", input: prepareInputWith(testIdentity(), ArtifactInput{Bytes: []byte("png bytes"), ContentType: "image/png", Width: 1, Height: 1, SourceAssetID: "source-1", ProviderReceiptID: "C:/worker/generated.png"})},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := store.PrepareSlotArtifacts(tc.input); err == nil {
				t.Fatal("PrepareSlotArtifacts() error = nil, want validation failure")
			}
		})
	}
}

func newTestStore(t *testing.T, api *fakeS3API) *S3DurableArtifactStore {
	t.Helper()
	uploader := storage.NewS3UploaderWithAPI(api, storage.S3UploaderOptions{Bucket: "assets"})
	store, err := NewS3DurableArtifactStore(uploader, S3DurableArtifactStoreConfig{MaxArtifactBytes: 9})
	if err != nil {
		t.Fatalf("NewS3DurableArtifactStore() error = %v", err)
	}
	return store
}

func mustPrepare(t *testing.T, store *S3DurableArtifactStore) PreparedSlotArtifacts {
	t.Helper()
	prepared, err := store.PrepareSlotArtifacts(prepareInputWith(testIdentity(), ArtifactInput{Bytes: []byte("png bytes"), ContentType: "image/png", Width: 1200, Height: 800, SourceAssetID: "source-1", Operations: []string{"resize"}, ProviderReceiptID: "receipt-1"}))
	if err != nil {
		t.Fatalf("PrepareSlotArtifacts() error = %v", err)
	}
	return prepared
}

func prepareInputWith(identity imageagent.SlotExternalEffectIdentity, asset ArtifactInput) PrepareSlotArtifactsInput {
	return PrepareSlotArtifactsInput{Identity: identity, Assets: []ArtifactInput{asset}}
}

func testIdentity() imageagent.SlotExternalEffectIdentity {
	return imageagent.SlotExternalEffectIdentity{RunScope: imageagent.RunScope{TenantID: "tenant-a", OwnerUserID: "user-a", RunID: "run-1"}, PlanRevision: 3, SlotID: "slot-1", Attempt: 2}
}

func sha256Hex(value []byte) string {
	sum := sha256.Sum256(value)
	return hex.EncodeToString(sum[:])
}

type fakeS3API struct {
	objects   map[string]fakeObject
	put       func(*s3.PutObjectInput) error
	putCalls  int
	headCalls int
	copyCalls int
}

type fakeObject struct {
	contentType   string
	contentLength int64
	metadata      map[string]string
	checksumSHA   string
	eTag          string
}

func (f *fakeS3API) PutObject(_ context.Context, input *s3.PutObjectInput, _ ...func(*s3.Options)) (*s3.PutObjectOutput, error) {
	f.putCalls++
	if f.put != nil {
		if err := f.put(input); err != nil {
			return nil, err
		}
	}
	f.savePut(input)
	return &s3.PutObjectOutput{}, nil
}

func (f *fakeS3API) HeadObject(_ context.Context, input *s3.HeadObjectInput, _ ...func(*s3.Options)) (*s3.HeadObjectOutput, error) {
	f.headCalls++
	object, ok := f.objects[aws.ToString(input.Key)]
	if !ok {
		return nil, &types.NotFound{}
	}
	return &s3.HeadObjectOutput{ContentType: aws.String(object.contentType), ContentLength: aws.Int64(object.contentLength), Metadata: object.metadata, ChecksumSHA256: aws.String(object.checksumSHA), ETag: aws.String(object.eTag)}, nil
}

func (f *fakeS3API) CopyObject(_ context.Context, input *s3.CopyObjectInput, _ ...func(*s3.Options)) (*s3.CopyObjectOutput, error) {
	f.copyCalls++
	if _, ok := f.objects[aws.ToString(input.Key)]; ok {
		return nil, errors.New("destination exists")
	}
	source := strings.TrimPrefix(aws.ToString(input.CopySource), "assets/")
	object, ok := f.objects[source]
	if !ok {
		return nil, &types.NoSuchKey{}
	}
	object.contentType = aws.ToString(input.ContentType)
	object.metadata = input.Metadata
	f.objects[aws.ToString(input.Key)] = object
	return &s3.CopyObjectOutput{}, nil
}

func (f *fakeS3API) DeleteObject(context.Context, *s3.DeleteObjectInput, ...func(*s3.Options)) (*s3.DeleteObjectOutput, error) {
	return nil, errors.New("not implemented")
}

func (f *fakeS3API) savePut(input *s3.PutObjectInput) {
	if f.objects == nil {
		f.objects = make(map[string]fakeObject)
	}
	f.objects[aws.ToString(input.Key)] = fakeObject{contentType: aws.ToString(input.ContentType), contentLength: aws.ToInt64(input.ContentLength), metadata: input.Metadata, checksumSHA: aws.ToString(input.ChecksumSHA256)}
}
