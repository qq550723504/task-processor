package imageagentapp

import (
	"context"
	"github.com/stretchr/testify/require"
	"strings"
	"task-processor/internal/marketplace/shein/goods"
	"task-processor/internal/product/asset"
	"testing"
)

type materialProbeFixture struct{}

func (materialProbeFixture) Probe(_ context.Context, a asset.ApprovedAsset, typ int) (goods.OfficialImageObservation, error) {
	return goods.OfficialImageObservation{AssetID: a.ID, SourceURL: a.URL, Type: typ, Width: 900, Height: 900, ContentHash: strings.Repeat("a", 64), Bytes: 123, MediaType: "image/png"}, nil
}

func TestGenericMaterialApprovalRequiresActualBytesWithoutOfficialPublishingClaims(t *testing.T) {
	r := ImageSetMaterialTargetResolver{Images: materialProbeFixture{}}
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
