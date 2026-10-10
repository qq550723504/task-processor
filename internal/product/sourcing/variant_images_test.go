package sourcing

import (
	"github.com/stretchr/testify/require"
	"testing"
)

func TestVariantImagesRemainBoundToTheirSourceVariantAndAreBounded(t *testing.T) {
	e := SourceEnvelope{Identity: SourceIdentity{SourceType: SourceTypeWarehouseCatalog, SourcePlatform: "supply_market", SourceID: "private-source"}, ProductCandidate: ProductCandidate{Title: "两款商品", Variants: []ProductVariantCandidate{{SourceID: "red", Images: []AssetCandidate{{SourceID: "red-image", URL: "https://images.example.org/red.png", MediaType: "image", Role: "main"}}}, {SourceID: "blue", Images: []AssetCandidate{{SourceID: "blue-image", URL: "https://images.example.org/blue.png", MediaType: "image", Role: "main"}}}}}}
	n := e.Normalize()
	n.ProductCandidate.Variants[0].Images[0].URL = "changed"
	require.Equal(t, "https://images.example.org/red.png", e.ProductCandidate.Variants[0].Images[0].URL)
	snapshot, err := ToSnapshot(e)
	require.NoError(t, err)
	require.Equal(t, "https://images.example.org/red.png", snapshot.Variants[0].Images[0].URL)
	require.Equal(t, "https://images.example.org/blue.png", snapshot.Variants[1].Images[0].URL)
	require.Empty(t, snapshot.Images)
	e.ProductCandidate.Variants[0].Images = make([]AssetCandidate, MaxSourceEnvelopeCollectionItems+1)
	require.ErrorIs(t, validateSourceEnvelopePreflight(e), ErrSourcePublicationTooLarge)
}
