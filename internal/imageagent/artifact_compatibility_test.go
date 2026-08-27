package imageagent

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

const (
	historicalNilOperationsStagingJSON = `{"assets":[{"object_key":"image-agent/staging/tenant-a/run/asset.png","sha256":"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa","size_bytes":42,"content_type":"image/png","width":1200,"height":1200,"source_asset_id":"source-1","operations":null,"provider_receipt_id":"receipt-1"}]}`
	historicalNilOperationsFinalJSON   = `{"assets":[{"object_key":"image-agent/final/tenant-a/run/asset.png","sha256":"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa","size_bytes":42,"content_type":"image/png","width":1200,"height":1200,"source_asset_id":"source-1","operations":null,"provider_receipt_id":"receipt-1"}]}`
	historicalNilOperationsStagingFP   = "d567351ea12b329121705ff34e42e5e9b4c9c660eda7fee964a04a43bd4de3b7"
	historicalNilOperationsFinalFP     = "5376895598efa8363e8557708d1f896b8f877a0740007d4abc6991ac202f4942"
)

func TestNormalizeArtifactOperationsPreservesNilAndEmptySliceRepresentations(t *testing.T) {
	nilOperations, err := NormalizeArtifactOperations(nil)
	require.NoError(t, err)
	require.Nil(t, nilOperations)

	emptyOperations, err := NormalizeArtifactOperations([]string{})
	require.NoError(t, err)
	require.NotNil(t, emptyOperations)
	require.Empty(t, emptyOperations)

	input := []string{"resize"}
	normalized, err := NormalizeArtifactOperations(input)
	require.NoError(t, err)
	normalized[0] = "extract_subject"
	require.Equal(t, []string{"resize"}, input)
}

func TestNilOperationsRemainTask1CompatibleAcrossManifestNormalizationJSONAndFingerprints(t *testing.T) {
	staging := historicalNilOperationsStagingManifest()
	final := historicalNilOperationsFinalManifest()

	normalizedStaging, err := NormalizeStagingManifest(staging)
	require.NoError(t, err)
	require.Nil(t, normalizedStaging.Assets[0].Operations)
	encodedStaging, err := json.Marshal(normalizedStaging)
	require.NoError(t, err)
	require.JSONEq(t, historicalNilOperationsStagingJSON, string(encodedStaging))
	require.Equal(t, historicalNilOperationsStagingFP, historicalVectorFingerprint(historicalNilOperationsStagingJSON))
	stagingFingerprint, err := StagingManifestFingerprint(staging)
	require.NoError(t, err)
	require.Equal(t, historicalNilOperationsStagingFP, stagingFingerprint)

	normalizedFinal, err := NormalizeFinalManifest(final)
	require.NoError(t, err)
	require.Nil(t, normalizedFinal.Assets[0].Operations)
	encodedFinal, err := json.Marshal(normalizedFinal)
	require.NoError(t, err)
	require.JSONEq(t, historicalNilOperationsFinalJSON, string(encodedFinal))
	require.Equal(t, historicalNilOperationsFinalFP, historicalVectorFingerprint(historicalNilOperationsFinalJSON))
	finalFingerprint, err := FinalManifestFingerprint(final)
	require.NoError(t, err)
	require.Equal(t, historicalNilOperationsFinalFP, finalFingerprint)

	emptyStaging := historicalNilOperationsStagingManifest()
	emptyStaging.Assets[0].Operations = []string{}
	normalizedEmptyStaging, err := NormalizeStagingManifest(emptyStaging)
	require.NoError(t, err)
	require.NotNil(t, normalizedEmptyStaging.Assets[0].Operations)
	emptyStagingJSON, err := json.Marshal(normalizedEmptyStaging)
	require.NoError(t, err)
	require.Contains(t, string(emptyStagingJSON), `"operations":[]`)

	emptyFinal := historicalNilOperationsFinalManifest()
	emptyFinal.Assets[0].Operations = []string{}
	normalizedEmptyFinal, err := NormalizeFinalManifest(emptyFinal)
	require.NoError(t, err)
	require.NotNil(t, normalizedEmptyFinal.Assets[0].Operations)
	emptyFinalJSON, err := json.Marshal(normalizedEmptyFinal)
	require.NoError(t, err)
	require.Contains(t, string(emptyFinalJSON), `"operations":[]`)
}

func historicalNilOperationsStagingManifest() StagingManifest {
	return StagingManifest{Assets: []StagedAssetRef{historicalNilOperationsAsset("image-agent/staging/tenant-a/run/asset.png")}}
}

func historicalNilOperationsFinalManifest() FinalManifest {
	return FinalManifest{Assets: []PublishedAssetRef{historicalNilOperationsPublishedAsset("image-agent/final/tenant-a/run/asset.png")}}
}

func historicalNilOperationsAsset(objectKey string) StagedAssetRef {
	return StagedAssetRef{ObjectKey: objectKey, SHA256: strings.Repeat("a", 64), SizeBytes: 42, ContentType: "image/png", Width: 1200, Height: 1200, SourceAssetID: "source-1", ProviderReceiptID: "receipt-1"}
}

func historicalNilOperationsPublishedAsset(objectKey string) PublishedAssetRef {
	return PublishedAssetRef{ObjectKey: objectKey, SHA256: strings.Repeat("a", 64), SizeBytes: 42, ContentType: "image/png", Width: 1200, Height: 1200, SourceAssetID: "source-1", ProviderReceiptID: "receipt-1"}
}

func historicalVectorFingerprint(jsonDocument string) string {
	sum := sha256.Sum256([]byte(jsonDocument))
	return hex.EncodeToString(sum[:])
}
