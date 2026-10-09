package submission

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
	record "task-processor/internal/listing/record/target"
	"task-processor/internal/marketplace/shein/goods"
	model "task-processor/internal/marketplace/shein/model"
)

func TestPublicationIntentMatchesImmutableRecordExceptConfirmedImageURLs(t *testing.T) {
	local := model.PublishProduct{CategoryID: 123, ProductTypeID: 456, SourceSystem: "OpenAPI", SuitFlag: "0", Names: []model.LanguageContent{{Language: "en", Name: "Approved manual title"}}, SKCs: []model.ProductSKC{{ImageInfo: model.ImageInfo{Images: []model.ProductImage{{Sort: 1, Type: 1, URL: "https://images.example.org/approved.jpg"}}}, SKUs: []model.ProductSKU{{SupplierSKU: "sku-a"}}}}}
	stored := record.TargetRecord{Result: goods.OfficialDraft{Product: local, ReadyForUpload: true}}
	raw, err := json.Marshal(local)
	require.NoError(t, err)
	var sent model.PublishProduct
	require.NoError(t, json.Unmarshal(raw, &sent))
	sent.SKCs[0].ImageInfo.Images[0].URL = "https://img.shein.com/confirmed.jpg"
	raw, err = json.Marshal(sent)
	require.NoError(t, err)
	require.True(t, MatchesPublicationRecord(stored, raw))
	sent.Names[0].Name = "Different title"
	raw, err = json.Marshal(sent)
	require.NoError(t, err)
	require.False(t, MatchesPublicationRecord(stored, raw))
	sent.Names[0].Name = local.Names[0].Name
	sent.SKCs[0].ImageInfo.Images[0].Type = 2
	raw, err = json.Marshal(sent)
	require.NoError(t, err)
	require.False(t, MatchesPublicationRecord(stored, raw), "an image effect cannot replace the selected position or type")
	_, err = (OfficialIntentCommit{}).Read(context.Background())
	require.ErrorIs(t, err, ErrExecutionEvidenceRequired)
}
