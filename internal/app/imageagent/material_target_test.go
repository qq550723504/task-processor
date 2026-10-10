package imageagentapp

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"github.com/stretchr/testify/require"
	"image"
	"image/png"
	"os"
	"strings"
	"task-processor/internal/imageagent"
	"task-processor/internal/imageagent/objectstore"
	"task-processor/internal/product/asset"
	"testing"
)

func TestGenericMaterialApprovalAcceptsSupportedWebPSource(t *testing.T) {
	// Existing Go x/image test fixture; its license is retained in testdata.
	content, err := os.ReadFile("testdata/source.webp")
	require.NoError(t, err)
	r := ImageSetMaterialTargetResolver{ReadBytes: func(context.Context, imageagent.AuthorizedAsset, int64) ([]byte, error) { return content, nil }}
	selected := []asset.ApprovedAsset{{ID: "source", URL: "https://source.example/image.webp", SourceApproval: &asset.SourceApprovalProvenance{}}}
	resolved, err := r.ResolveImageSetTarget(context.Background(), asset.SourceSelection{TargetPlatform: "product"}, nil, selected)
	require.NoError(t, err, "WebP accepted by preparation must remain selectable as generic material")
	require.Nil(t, resolved.Target)
	require.Positive(t, selected[0].Width)
	require.Positive(t, selected[0].Height)
	content = content[:20]
	_, err = r.ResolveImageSetTarget(context.Background(), asset.SourceSelection{TargetPlatform: "product"}, nil, selected)
	require.ErrorIs(t, err, asset.ErrInvalidApproval)
}

func TestGenericMaterialApprovalRequiresActualBytesWithoutOfficialPublishingClaims(t *testing.T) {
	var content bytes.Buffer
	require.NoError(t, png.Encode(&content, image.NewRGBA(image.Rect(0, 0, 900, 900))))
	r := ImageSetMaterialTargetResolver{ReadBytes: func(context.Context, imageagent.AuthorizedAsset, int64) ([]byte, error) { return content.Bytes(), nil }}
	selected := []asset.ApprovedAsset{{ID: "source", URL: "https://source.example/image.png", SourceApproval: &asset.SourceApprovalProvenance{}}}
	resolved, err := r.ResolveImageSetTarget(context.Background(), asset.SourceSelection{TargetPlatform: "product"}, nil, selected)
	require.NoError(t, err)
	require.Nil(t, resolved.Target)
	require.Empty(t, resolved.RequirementDigest)
	require.Equal(t, 900, selected[0].Width)
	selected[0].SourceApproval = nil
	selected[0].GenerationEvidence = &asset.GenerationEvidence{ArtifactHash: strings.Repeat("b", 64)}
	r.ReadGeneratedBytes = func(context.Context, asset.SourceSelection, asset.ApprovedAsset, int64) ([]byte, error) {
		return content.Bytes(), nil
	}
	_, err = r.ResolveImageSetTarget(context.Background(), asset.SourceSelection{TargetPlatform: "product"}, nil, selected)
	require.ErrorIs(t, err, asset.ErrApprovalConflict)
	selected[0].OfficialPlacement = &asset.ImageOfficialPlacement{Group: "skc", Type: 1, Sort: 1, Site: "shein-us"}
	_, err = r.ResolveImageSetTarget(context.Background(), asset.SourceSelection{TargetPlatform: "product"}, nil, selected)
	require.ErrorIs(t, err, asset.ErrInvalidApproval)
}

func TestGenericMaterialSelectionUsesVerifiedLocalGeneratedOwnerBytes(t *testing.T) {
	var content bytes.Buffer
	require.NoError(t, png.Encode(&content, image.NewRGBA(image.Rect(0, 0, 900, 900))))
	hash := sha256.Sum256(content.Bytes())
	base := "https://localhost:33644/image-agent-assets/images"
	key := "image-agent/public/org/" + strings.Repeat("a", 64) + "/run/1/main/1/0-" + hex.EncodeToString(hash[:]) + ".png"
	selected := []asset.ApprovedAsset{{ID: "generated", RunID: "run", PlanRevision: 1, SlotID: "main", Attempt: 1, URL: base + "/" + key, Width: 900, Height: 900, GenerationEvidence: &asset.GenerationEvidence{ArtifactHash: hex.EncodeToString(hash[:])}}}
	objects := &materialObjectsFixture{base: base, key: key, content: content.Bytes()}
	reader := GeneratedMaterialReader{PublicBase: base, Objects: objects}
	r := ImageSetMaterialTargetResolver{ReadGeneratedBytes: reader.Read, ReadBytes: func(context.Context, imageagent.AuthorizedAsset, int64) ([]byte, error) {
		t.Fatal("generated artifact must not be downloaded by URL")
		return nil, nil
	}}
	_, err := r.ResolveImageSetTarget(context.Background(), asset.SourceSelection{TenantID: "org", TargetPlatform: "product"}, nil, selected)
	require.NoError(t, err, "already verified generated artifacts must be read from their immutable owner")
	require.Equal(t, 1, objects.reads)
}

type materialObjectsFixture struct {
	base, key string
	content   []byte
	reads     int
}

func (f *materialObjectsFixture) PublicURL(key string) string { return f.base + "/" + key }
func (f *materialObjectsFixture) ReadObject(_ context.Context, key string, _ int64) ([]byte, objectstore.ObjectInspection, error) {
	f.reads++
	if key != f.key {
		return nil, objectstore.ObjectInspection{}, asset.ErrInvalidApproval
	}
	return f.content, objectstore.ObjectInspection{Exists: true, ContentLength: int64(len(f.content))}, nil
}

func TestGeneratedMaterialReaderBindsOriginalPublishedIdentity(t *testing.T) {
	content := []byte("immutable fixture bytes")
	hash := sha256.Sum256(content)
	digest := hex.EncodeToString(hash[:])
	base := "https://localhost:33644/image-agent-assets/images"
	key := "image-agent/public/org/" + strings.Repeat("a", 64) + "/run/2/main/3/0-" + digest + ".png"
	approved := asset.ApprovedAsset{RunID: "run", PlanRevision: 2, SlotID: "main", Attempt: 3, URL: base + "/" + key, GenerationEvidence: &asset.GenerationEvidence{ArtifactHash: digest}}
	source := asset.SourceSelection{TenantID: "org", ActorID: "another-authorized-member"}
	for _, tc := range []struct {
		name   string
		mutate func(*asset.SourceSelection, *asset.ApprovedAsset)
	}{
		{"tenant", func(s *asset.SourceSelection, a *asset.ApprovedAsset) { s.TenantID = "other" }},
		{"run", func(s *asset.SourceSelection, a *asset.ApprovedAsset) { a.RunID = "other" }},
		{"revision", func(s *asset.SourceSelection, a *asset.ApprovedAsset) { a.PlanRevision++ }},
		{"slot", func(s *asset.SourceSelection, a *asset.ApprovedAsset) { a.SlotID = "other" }},
		{"attempt", func(s *asset.SourceSelection, a *asset.ApprovedAsset) { a.Attempt++ }},
		{"base", func(s *asset.SourceSelection, a *asset.ApprovedAsset) {
			a.URL = strings.Replace(a.URL, "localhost:33644", "localhost:33645", 1)
		}},
		{"noncanonical owner", func(s *asset.SourceSelection, a *asset.ApprovedAsset) {
			a.URL = strings.Replace(a.URL, strings.Repeat("a", 64), "owner", 1)
		}},
		{"traversal", func(s *asset.SourceSelection, a *asset.ApprovedAsset) { a.URL = base + "/../" + key }},
		{"unpublished", func(s *asset.SourceSelection, a *asset.ApprovedAsset) {
			a.URL = strings.Replace(a.URL, "/public/", "/staging/", 1)
		}},
		{"hash", func(s *asset.SourceSelection, a *asset.ApprovedAsset) {
			a.GenerationEvidence = &asset.GenerationEvidence{ArtifactHash: strings.Repeat("b", 64)}
		}},
		{"source", func(s *asset.SourceSelection, a *asset.ApprovedAsset) {
			a.SourceApproval = &asset.SourceApprovalProvenance{}
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s, a := source, approved
			tc.mutate(&s, &a)
			objects := &materialObjectsFixture{base: base, key: key, content: content}
			_, err := (GeneratedMaterialReader{PublicBase: base, Objects: objects}).Read(context.Background(), s, a, 1024)
			require.ErrorIs(t, err, asset.ErrInvalidApproval)
			require.Zero(t, objects.reads)
		})
	}
	objects := &materialObjectsFixture{base: base, key: key, content: content}
	reader := GeneratedMaterialReader{PublicBase: base, Objects: objects}
	actual, err := reader.Read(context.Background(), source, approved, 1024)
	require.NoError(t, err)
	require.Equal(t, content, actual, "approved reuse preserves original owner even when selecting actor differs")
	objects.content = []byte("altered")
	_, err = reader.Read(context.Background(), source, approved, 1024)
	require.ErrorIs(t, err, asset.ErrApprovalConflict)
}
