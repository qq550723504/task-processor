package supplychainapp

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"task-processor/internal/listing/preparation"
	"task-processor/internal/listing/submission"
)

func TestUnknownOperationReadProjectsOriginalConfirmedReceiptWithoutRewritingHistory(t *testing.T) {
	uploader, scope, records, merchant, _, official, _ := uploadFixture(t)
	merchant.loseResponse = true
	pending, err := uploader.Upload(context.Background(), scope, uuid.NewString(), records.saved.ID)
	require.NoError(t, err)
	operation := preparation.Operation{Owner: scope, Input: preparation.OperationInput{Action: preparation.OperationUpload, StoreID: records.saved.Merchant.StoreID}}
	item := preparation.OperationItem{SourceID: records.saved.Source.ID, RecordID: records.saved.ID, RecordRevision: records.saved.Revision, Status: preparation.ItemUnknown, ResultReference: pending.AttemptID}
	app := Application{Records: records, PublicationReceipts: official}
	before, err := app.operationItemView(context.Background(), operation, item)
	require.NoError(t, err)
	require.Nil(t, before.ConfirmedProduct)
	resolved, err := uploader.ResolveUpload(context.Background(), scope, ResolveUploadInput{RecordID: records.saved.ID, AttemptID: pending.AttemptID, SPU: "spu-a"})
	require.NoError(t, err)
	require.Equal(t, submission.ExecutionSucceeded, resolved.Status)
	for range 2 {
		view, err := app.operationItemView(context.Background(), operation, item)
		require.NoError(t, err)
		require.Equal(t, item, view.OperationItem)
		require.NotNil(t, view.ConfirmedProduct)
		require.Equal(t, "spu-a", view.ConfirmedProduct.SPUName)
	}
	require.Equal(t, 1, merchant.publishes)
	require.Equal(t, 1, merchant.lookups)

	for _, change := range []func(*preparation.OperationItem){
		func(i *preparation.OperationItem) { i.RecordID = uuid.NewString() },
		func(i *preparation.OperationItem) { i.SourceID = uuid.NewString() },
		func(i *preparation.OperationItem) { i.RecordRevision++ },
	} {
		other := item
		change(&other)
		_, err = app.operationItemView(context.Background(), operation, other)
		require.Error(t, err)
	}
	operation.Input.StoreID = uuid.NewString()
	_, err = app.operationItemView(context.Background(), operation, item)
	require.Error(t, err)
}
