package imageagentapp

import (
	"bytes"
	"context"
	"github.com/stretchr/testify/require"
	"image"
	"image/png"
	"os"
	"strings"
	"task-processor/internal/imageagent"
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
	_, err = r.ResolveImageSetTarget(context.Background(), asset.SourceSelection{TargetPlatform: "product"}, nil, selected)
	require.ErrorIs(t, err, asset.ErrApprovalConflict)
	selected[0].OfficialPlacement = &asset.ImageOfficialPlacement{Group: "skc", Type: 1, Sort: 1, Site: "shein-us"}
	_, err = r.ResolveImageSetTarget(context.Background(), asset.SourceSelection{TargetPlatform: "product"}, nil, selected)
	require.ErrorIs(t, err, asset.ErrInvalidApproval)
}
