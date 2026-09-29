package s3

import (
	"encoding/base64"
	"encoding/hex"
	"errors"
	"testing"

	"task-processor/internal/knowledge"
)

func TestKnowledgeStoreRequiresPrivateImmutableStorage(t *testing.T) {
	for _, options := range []UploaderOptions{
		{Bucket: "knowledge"},
		{Bucket: "knowledge", PublicBase: "https://public.invalid", ArtifactCapabilities: ArtifactStorageCapabilities{Mode: ArtifactStorageModeAWS}},
	} {
		uploader := mustNewUploader(t, &fakeS3API{}, options)
		if _, err := NewKnowledgeStore(uploader); err == nil {
			t.Fatal("accepted non-private or unqualified storage")
		}
	}
	uploader := mustNewUploader(t, &fakeS3API{}, UploaderOptions{Bucket: "knowledge", ArtifactCapabilities: ArtifactStorageCapabilities{Mode: ArtifactStorageModeAWS}})
	if _, err := NewKnowledgeStore(uploader); err != nil {
		t.Fatal(err)
	}
	if _, err := NewKnowledgeStore(nil); !errors.Is(err, knowledge.ErrInvalid) {
		t.Fatal(err)
	}
}

func TestKnowledgeInspectionRejectsContradictoryOrMalformedChecksums(t *testing.T) {
	digest := knowledge.Digest([]byte("hello"))
	bytes, _ := hex.DecodeString(digest)
	server := base64.StdEncoding.EncodeToString(bytes)
	for _, test := range []struct{ name, metadata, server, want string }{
		{"metadata", digest, "", digest},
		{"matching server", digest, server, digest},
		{"server only", "", server, digest},
		{"contradictory", knowledge.Digest([]byte("other")), server, ""},
		{"malformed", digest, "not-base64", ""},
	} {
		t.Run(test.name, func(t *testing.T) {
			got := knowledgeInspection(ObjectInspection{Exists: true, ContentLength: 5, Metadata: map[string]string{"sha256": test.metadata}, ServerChecksumSHA256: test.server})
			if !got.Exists || got.SizeBytes != 5 || got.SHA256 != test.want {
				t.Fatalf("inspection: %+v", got)
			}
		})
	}
}
