package objectstore

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"hash/crc32"
	"image"
	"image/color"
	"image/png"
	"os"
	"strconv"
	"strings"
	"testing"

	"task-processor/internal/core/logger"
	"task-processor/internal/imageagent"
	"task-processor/internal/infra/storage"
	"task-processor/internal/pkg/imagex"

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
		Assets:   []ArtifactInput{validAsset(t, 3, 2)},
	})
	if err != nil {
		t.Fatalf("PrepareSlotArtifacts() error = %v", err)
	}

	wantSHA := sha256Hex(validAsset(t, 3, 2).Bytes)
	if got, want := prepared.Manifest.Assets[0].ObjectKey, "image-agent/staging/tenant-a/run-1/3/slot-1/2/0-"+wantSHA+".png"; got != want {
		t.Fatalf("object key = %q, want %q", got, want)
	}
	encoded, err := json.Marshal(prepared)
	if err != nil {
		t.Fatalf("marshal prepared artifacts: %v", err)
	}
	for _, sentinel := range []string{"local_path", "C:/worker", "https://transient.example", "authorization=secret", "png bytes"} {
		if strings.Contains(string(encoded), sentinel) {
			t.Fatalf("persisted JSON leaked transient material %q: %s", sentinel, encoded)
		}
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
	api.objects[ref.ObjectKey] = fakeObject{contentType: ref.ContentType, contentLength: ref.SizeBytes, metadata: map[string]string{"sha256": strings.Repeat("a", 64), "size-bytes": strconv.FormatInt(ref.SizeBytes, 10)}}

	if err := store.EnsureStaged(context.Background(), prepared); err == nil {
		t.Fatal("EnsureStaged() error = nil, want conflicting object error")
	}
	if api.putCalls != 0 {
		t.Fatalf("put calls = %d, want 0 for an existing conflicting object", api.putCalls)
	}
}

func TestEnsureStagedAcceptsMatchingExistingObjectWithoutPut(t *testing.T) {
	t.Parallel()

	api := &fakeS3API{objects: map[string]fakeObject{}}
	store := newTestStore(t, api)
	prepared := mustPrepare(t, store)
	ref := prepared.Manifest.Assets[0]
	api.objects[ref.ObjectKey] = fakeObject{contentType: ref.ContentType, contentLength: ref.SizeBytes, metadata: map[string]string{"sha256": ref.SHA256, "size-bytes": strconv.FormatInt(ref.SizeBytes, 10)}}

	if err := store.EnsureStaged(context.Background(), prepared); err != nil {
		t.Fatalf("EnsureStaged() error = %v", err)
	}
	if api.putCalls != 0 {
		t.Fatalf("put calls = %d, want 0 for a matching existing object", api.putCalls)
	}
}

func TestFinalizeUsesDeterministicPublicKey(t *testing.T) {
	t.Parallel()

	api := &fakeS3API{objects: map[string]fakeObject{}}
	store := newTestStore(t, api)
	prepared := mustPrepare(t, store)
	staged := prepared.Manifest.Assets[0]
	api.objects[staged.ObjectKey] = fakeObject{contentType: staged.ContentType, contentLength: staged.SizeBytes, metadata: map[string]string{"sha256": staged.SHA256, "size-bytes": strconv.FormatInt(staged.SizeBytes, 10)}}

	final, err := store.Finalize(context.Background(), prepared.Manifest)
	if err != nil {
		t.Fatalf("Finalize() error = %v", err)
	}
	var _ imageagent.PublishedAssetRef = final.Assets[0]
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
	api.objects[ref.ObjectKey] = fakeObject{contentType: ref.ContentType, contentLength: ref.SizeBytes, eTag: ref.SHA256, metadata: map[string]string{"sha256": strings.Repeat("b", 64), "size-bytes": strconv.FormatInt(ref.SizeBytes, 10)}}

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
		{name: "oversize", input: prepareInputWith(testIdentity(), ArtifactInput{Bytes: bytes.Repeat([]byte("x"), 2049), ContentType: "image/png", Width: 1, Height: 1, SourceAssetID: "source-1"})},
		{name: "unsupported type", input: prepareInputWith(testIdentity(), ArtifactInput{Bytes: validAsset(t, 1, 1).Bytes, ContentType: "image/gif", Width: 1, Height: 1, SourceAssetID: "source-1"})},
		{name: "fake image", input: prepareInputWith(testIdentity(), ArtifactInput{Bytes: []byte("not an image"), ContentType: "image/png", Width: 1, Height: 1, SourceAssetID: "source-1"})},
		{name: "declared dimensions differ", input: prepareInputWith(testIdentity(), ArtifactInput{Bytes: validAsset(t, 2, 1).Bytes, ContentType: "image/png", Width: 1, Height: 1, SourceAssetID: "source-1"})},
		{name: "escaping run ID", input: prepareInputWith(imageagent.SlotExternalEffectIdentity{RunScope: imageagent.RunScope{TenantID: "tenant-a", OwnerUserID: "user-a", RunID: "../run"}, PlanRevision: 3, SlotID: "slot-1", Attempt: 2}, validAsset(t, 1, 1))},
		{name: "local provider receipt", input: prepareInputWith(testIdentity(), withReceipt(validAsset(t, 1, 1), "C:/worker/generated.png"))},
		{name: "URL operation", input: prepareInputWith(testIdentity(), withOperations(validAsset(t, 1, 1), "https://transient.example/image"))},
		{name: "credential operation", input: prepareInputWith(testIdentity(), withOperations(validAsset(t, 1, 1), "authorization=secret"))},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := store.PrepareSlotArtifacts(tc.input); err == nil {
				t.Fatal("PrepareSlotArtifacts() error = nil, want validation failure")
			}
		})
	}
}

func TestPrepareAllowsCanonicalOpaqueIDsContainingTokenOrSecret(t *testing.T) {
	t.Parallel()
	store := newTestStore(t, &fakeS3API{})
	asset := validAsset(t, 1, 1)
	asset.SourceAssetID = "tokenized-source-1"
	asset.ProviderReceiptID = "secretary-receipt-1"
	if _, err := store.PrepareSlotArtifacts(prepareInputWith(testIdentity(), asset)); err != nil {
		t.Fatalf("PrepareSlotArtifacts() error = %v, want canonical opaque IDs to remain valid", err)
	}
}

func TestPrepareRejectsHostileDimensionsBeforeFullDecode(t *testing.T) {
	t.Parallel()
	store := newTestStore(t, &fakeS3API{})
	fullDecodeCalled := false
	store.inspectImage = func([]byte) (*imagex.ImageInfo, error) {
		fullDecodeCalled = true
		return nil, errors.New("full decode must not run")
	}
	asset := validAsset(t, 1, 1)
	asset.Bytes = hostilePNGHeader(8193, 1)
	asset.Width = 8193
	asset.Height = 1
	if _, err := store.PrepareSlotArtifacts(prepareInputWith(testIdentity(), asset)); err == nil {
		t.Fatal("PrepareSlotArtifacts() error = nil, want hostile dimensions rejection")
	}
	if fullDecodeCalled {
		t.Fatal("full image decode ran after DecodeConfig exceeded the configured dimension cap")
	}
}

func newTestStore(t *testing.T, api *fakeS3API) *S3DurableArtifactStore {
	t.Helper()
	uploader := storage.NewS3UploaderWithAPI(api, storage.S3UploaderOptions{Bucket: "assets", ArtifactCapabilities: storage.ArtifactStorageCapabilities{Mode: storage.ArtifactStorageModeAWS}})
	store, err := NewS3DurableArtifactStore(uploader, S3DurableArtifactStoreConfig{MaxArtifactBytes: 2048, MaxArtifactCount: 2, MaxAggregateBytes: 3072})
	if err != nil {
		t.Fatalf("NewS3DurableArtifactStore() error = %v", err)
	}
	return store
}

func mustPrepare(t *testing.T, store *S3DurableArtifactStore) PreparedSlotArtifacts {
	t.Helper()
	prepared, err := store.PrepareSlotArtifacts(prepareInputWith(testIdentity(), ArtifactInput{Bytes: validPNG(t, 3, 2), ContentType: "image/png", Width: 3, Height: 2, SourceAssetID: "source-1", Operations: []string{"extract_subject"}, ProviderReceiptID: "receipt-1"}))
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

func validAsset(t *testing.T, width, height int) ArtifactInput {
	t.Helper()
	return ArtifactInput{Bytes: validPNG(t, width, height), ContentType: "image/png", Width: width, Height: height, SourceAssetID: "source-1", Operations: []string{"extract_subject"}, ProviderReceiptID: "receipt-1"}
}

func validPNG(t *testing.T, width, height int) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, width, height))
	img.Set(0, 0, color.RGBA{R: 0x40, G: 0x80, B: 0xc0, A: 0xff})
	var encoded bytes.Buffer
	if err := png.Encode(&encoded, img); err != nil {
		t.Fatalf("encode PNG: %v", err)
	}
	return encoded.Bytes()
}

func hostilePNGHeader(width, height uint32) []byte {
	data := make([]byte, 8+4+4+13+4)
	copy(data, "\x89PNG\r\n\x1a\n")
	binary.BigEndian.PutUint32(data[8:12], 13)
	copy(data[12:16], "IHDR")
	binary.BigEndian.PutUint32(data[16:20], width)
	binary.BigEndian.PutUint32(data[20:24], height)
	data[24] = 8
	data[25] = 2
	binary.BigEndian.PutUint32(data[29:33], crc32.ChecksumIEEE(data[12:29]))
	return data
}

func withReceipt(asset ArtifactInput, receipt string) ArtifactInput {
	asset.ProviderReceiptID = receipt
	return asset
}

func withOperations(asset ArtifactInput, operations ...string) ArtifactInput {
	asset.Operations = operations
	return asset
}

func TestPrepareSlotArtifactsRejectsAggregateAndCountLimits(t *testing.T) {
	t.Parallel()
	store := newTestStore(t, &fakeS3API{})
	asset := validAsset(t, 1, 1)
	asset.Bytes = bytes.Repeat([]byte("x"), 1600)
	if _, err := store.PrepareSlotArtifacts(PrepareSlotArtifactsInput{Identity: testIdentity(), Assets: []ArtifactInput{asset, asset}}); err == nil {
		t.Fatal("PrepareSlotArtifacts() error = nil, want aggregate limit failure")
	}
	if _, err := store.PrepareSlotArtifacts(PrepareSlotArtifactsInput{Identity: testIdentity(), Assets: []ArtifactInput{validAsset(t, 1, 1), validAsset(t, 1, 1), validAsset(t, 1, 1)}}); err == nil {
		t.Fatal("PrepareSlotArtifacts() error = nil, want count limit failure")
	}
}

func TestRecoveryRejectsNonCanonicalPersistedStagingKeysBeforeSDKCalls(t *testing.T) {
	t.Parallel()
	for _, mutate := range []func(*imageagent.StagedAssetRef){
		func(ref *imageagent.StagedAssetRef) {
			ref.ObjectKey = "image-agent/staging/tenant-a/run-1/03/slot-1/2/0-" + ref.SHA256 + ".png"
		},
		func(ref *imageagent.StagedAssetRef) {
			ref.ObjectKey = "image-agent/staging/tenant-a/run-1/3/slot-1/2/1-" + ref.SHA256 + ".png"
		},
		func(ref *imageagent.StagedAssetRef) {
			ref.ObjectKey = "image-agent/staging/tenant-a/run-1/3/slot-1/2/0-" + strings.ToUpper(ref.SHA256) + ".png"
		},
		func(ref *imageagent.StagedAssetRef) {
			ref.ObjectKey = "image-agent/staging/tenant-a/run-1/3/slot-1/2/0-" + ref.SHA256 + ".jpg"
		},
		func(ref *imageagent.StagedAssetRef) {
			ref.ObjectKey = "image-agent/staging/tenant a/run-1/3/slot-1/2/0-" + ref.SHA256 + ".png"
		},
		func(ref *imageagent.StagedAssetRef) {
			ref.Operations = []string{"provider=https://transient.example"}
		},
	} {
		api := &fakeS3API{objects: map[string]fakeObject{}}
		store := newTestStore(t, api)
		prepared := mustPrepare(t, store)
		mutate(&prepared.Manifest.Assets[0])
		if err := store.EnsureStaged(context.Background(), prepared); err == nil {
			t.Fatal("EnsureStaged() error = nil, want malformed manifest rejection")
		}
		if api.headCalls != 0 || api.putCalls != 0 || api.copyCalls != 0 {
			t.Fatalf("SDK calls occurred for malformed manifest: head=%d put=%d copy=%d", api.headCalls, api.putCalls, api.copyCalls)
		}
	}
}

func TestPersistedManifestPreflightsEveryAssetBeforeStorageCalls(t *testing.T) {
	t.Parallel()
	for _, operation := range []struct {
		name string
		run  func(context.Context, *S3DurableArtifactStore, PreparedSlotArtifacts) error
	}{
		{name: "ensure", run: func(ctx context.Context, store *S3DurableArtifactStore, prepared PreparedSlotArtifacts) error {
			return store.EnsureStaged(ctx, prepared)
		}},
		{name: "finalize", run: func(ctx context.Context, store *S3DurableArtifactStore, prepared PreparedSlotArtifacts) error {
			_, err := store.Finalize(ctx, prepared.Manifest)
			return err
		}},
	} {
		for _, mutate := range []struct {
			name  string
			apply func(*imageagent.StagedAssetRef)
		}{
			{name: "malformed later key", apply: func(ref *imageagent.StagedAssetRef) {
				ref.ObjectKey = "image-agent/staging/tenant-a/run-1/03/slot-1/2/1-" + ref.SHA256 + ".png"
			}},
			{name: "unsafe later operation", apply: func(ref *imageagent.StagedAssetRef) { ref.Operations = []string{"provider=https://transient.example"} }},
		} {
			t.Run(operation.name+"/"+mutate.name, func(t *testing.T) {
				api := &fakeS3API{objects: map[string]fakeObject{}}
				store := newTestStore(t, api)
				prepared, err := store.PrepareSlotArtifacts(PrepareSlotArtifactsInput{Identity: testIdentity(), Assets: []ArtifactInput{validAsset(t, 1, 1), validAsset(t, 2, 1)}})
				if err != nil {
					t.Fatalf("PrepareSlotArtifacts() error = %v", err)
				}
				mutate.apply(&prepared.Manifest.Assets[1])
				if err := operation.run(context.Background(), store, prepared); err == nil {
					t.Fatal("storage call error = nil, want preflight validation failure")
				}
				if api.headCalls != 0 || api.putCalls != 0 || api.copyCalls != 0 {
					t.Fatalf("storage calls occurred before complete manifest preflight: head=%d put=%d copy=%d", api.headCalls, api.putCalls, api.copyCalls)
				}
			})
		}
	}
}

func TestFinalizeRejectsNonCanonicalPersistedStagingKeysBeforeSDKCalls(t *testing.T) {
	t.Parallel()
	api := &fakeS3API{objects: map[string]fakeObject{}}
	store := newTestStore(t, api)
	prepared := mustPrepare(t, store)
	prepared.Manifest.Assets[0].ObjectKey = "image-agent/staging/tenant-a/run-1/03/slot-1/2/0-" + prepared.Manifest.Assets[0].SHA256 + ".png"
	if _, err := store.Finalize(context.Background(), prepared.Manifest); err == nil {
		t.Fatal("Finalize() error = nil, want malformed manifest rejection")
	}
	if api.headCalls != 0 || api.putCalls != 0 || api.copyCalls != 0 {
		t.Fatalf("SDK calls occurred for malformed manifest: head=%d put=%d copy=%d", api.headCalls, api.putCalls, api.copyCalls)
	}
}

func TestFinalizeReconcilesLostCopyAndRejectsConflictingFinal(t *testing.T) {
	t.Parallel()
	t.Run("lost copy", func(t *testing.T) {
		api := &fakeS3API{objects: map[string]fakeObject{}}
		store := newTestStore(t, api)
		prepared := mustPrepare(t, store)
		staged := prepared.Manifest.Assets[0]
		api.objects[staged.ObjectKey] = fakeObject{contentType: staged.ContentType, contentLength: staged.SizeBytes, metadata: map[string]string{"sha256": staged.SHA256, "size-bytes": strconv.FormatInt(staged.SizeBytes, 10)}}
		api.copy = func(input *s3.CopyObjectInput) error { api.saveCopy(input); return errors.New("copy response lost") }
		if _, err := store.Finalize(context.Background(), prepared.Manifest); err != nil {
			t.Fatalf("Finalize() error = %v", err)
		}
	})
	t.Run("conflicting final", func(t *testing.T) {
		api := &fakeS3API{objects: map[string]fakeObject{}}
		store := newTestStore(t, api)
		prepared := mustPrepare(t, store)
		staged := prepared.Manifest.Assets[0]
		api.objects[staged.ObjectKey] = fakeObject{contentType: staged.ContentType, contentLength: staged.SizeBytes, metadata: map[string]string{"sha256": staged.SHA256, "size-bytes": strconv.FormatInt(staged.SizeBytes, 10)}}
		api.objects[strings.Replace(staged.ObjectKey, "image-agent/staging/", "image-agent/public/", 1)] = fakeObject{contentType: staged.ContentType, contentLength: staged.SizeBytes, metadata: map[string]string{"sha256": strings.Repeat("a", 64), "size-bytes": strconv.FormatInt(staged.SizeBytes, 10)}}
		if _, err := store.Finalize(context.Background(), prepared.Manifest); err == nil {
			t.Fatal("Finalize() error = nil, want conflicting final error")
		}
	})
}

func TestEnsureStagedRejectsMissingRetryBytes(t *testing.T) {
	t.Parallel()
	store := newTestStore(t, &fakeS3API{objects: map[string]fakeObject{}})
	prepared := mustPrepare(t, store)
	encoded, err := json.Marshal(prepared)
	if err != nil {
		t.Fatalf("marshal prepared: %v", err)
	}
	var recovered PreparedSlotArtifacts
	if err := json.Unmarshal(encoded, &recovered); err != nil {
		t.Fatalf("unmarshal prepared: %v", err)
	}
	if err := store.EnsureStaged(context.Background(), recovered); !errors.Is(err, ErrArtifactUnavailable) {
		t.Fatalf("EnsureStaged() error = %v, want ErrArtifactUnavailable", err)
	}
}

func TestPreparedJSONExcludesEveryTransientSentinel(t *testing.T) {
	t.Parallel()
	store := newTestStore(t, &fakeS3API{})
	prepared := mustPrepare(t, store)
	prepared.contents[prepared.Manifest.Assets[0].ObjectKey] = []byte("C:/worker/private.png https://transient.example authorization=secret")
	prepared.Manifest.ProviderMetadata = map[string]string{"provider_metadata": "C:/worker/private.png https://transient.example authorization=secret"}

	encoded, err := json.Marshal(prepared)
	if err != nil {
		t.Fatalf("marshal prepared: %v", err)
	}
	for _, sentinel := range []string{"C:/worker", "https://transient.example", "authorization=secret", "provider_metadata"} {
		if strings.Contains(string(encoded), sentinel) {
			t.Fatalf("persisted JSON leaked %q: %s", sentinel, encoded)
		}
	}
}

type fakeS3API struct {
	objects   map[string]fakeObject
	put       func(*s3.PutObjectInput) error
	copy      func(*s3.CopyObjectInput) error
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
	if f.copy != nil {
		if err := f.copy(input); err != nil {
			return nil, err
		}
	}
	f.saveCopy(input)
	return &s3.CopyObjectOutput{}, nil
}

func (f *fakeS3API) saveCopy(input *s3.CopyObjectInput) {
	if _, ok := f.objects[aws.ToString(input.Key)]; ok {
		return
	}
	source := strings.TrimPrefix(aws.ToString(input.CopySource), "assets/")
	object, ok := f.objects[source]
	if !ok {
		return
	}
	object.contentType = aws.ToString(input.ContentType)
	object.metadata = input.Metadata
	f.objects[aws.ToString(input.Key)] = object
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
