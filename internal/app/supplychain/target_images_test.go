package supplychainapp

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	record "task-processor/internal/listing/record/target"
	"task-processor/internal/marketplace/shein/goods"
	"task-processor/internal/product/asset"
	"task-processor/internal/product/collection"
)

func (r *uploadRecords) FindTargetCommand(context.Context, collection.Scope, string, string) (record.TargetReceipt, error) {
	return record.TargetReceipt{}, record.ErrNotFound
}
func (r *uploadRecords) SaveTarget(ctx context.Context, proof record.TargetPrepared) (record.TargetReceipt, error) {
	_, _, _, value, err := proof.Read(ctx)
	if err != nil {
		return record.TargetReceipt{}, err
	}
	r.saved = value
	return record.TargetReceipt{Record: value}, nil
}

type targetProbe struct {
	calls                 atomic.Int64
	failAsset, driftAsset string
}

func (p *targetProbe) Probe(ctx context.Context, a asset.ApprovedAsset, typ int) (goods.OfficialImageObservation, error) {
	p.calls.Add(1)
	if a.ID == p.failAsset {
		return goods.OfficialImageObservation{}, errors.New("invalid actual image")
	}
	v, err := (uploadProbe{}).Probe(ctx, a, typ)
	if a.ID == p.driftAsset {
		v.ContentHash = collection.Digest("different bytes")
	}
	return v, err
}
func TestTargetSaveUsesActualImagesWhenSourceDimensionsAreUnknown(t *testing.T) {
	uploader, scope, records, merchant, _, _, auth := uploadFixture(t)
	inventory, err := uploader.dependencies.Assets.GetApprovedInventory(context.Background(), asset.InventoryScope{})
	require.NoError(t, err)
	for i := range inventory.Assets {
		inventory.Assets[i].Width = 0
		inventory.Assets[i].Height = 0
	}
	probe := &targetProbe{}
	service, err := record.NewTargetService(record.TargetDependencies{Sources: uploader.dependencies.Sources.(record.TargetSourceSelector), ExecutionSources: uploader.dependencies.Sources, ExecutionAuthorization: auth, Products: uploader.dependencies.Products, Assets: uploadAssets{inventory}, Rules: uploader.dependencies.Rules, Records: records, Authorizer: auth, Images: probe})
	require.NoError(t, err)
	input := records.saved.Input
	input.ExpectedRevision = 1
	receipt, err := service.CreateForExecution(context.Background(), scope, uuid.NewString(), input)
	require.NoError(t, err)
	require.True(t, receipt.Record.Result.ReadyForUpload, "%v", receipt.Record.Result.Issues)
	require.Len(t, receipt.Record.ImageObservations, 3)
	require.Equal(t, 3, int(probe.calls.Load()))
	require.Zero(t, merchant.transforms)
	uploader.dependencies.Assets = uploadAssets{inventory}
	result, err := uploader.Upload(context.Background(), scope, uuid.NewString(), receipt.Record.ID)
	require.NoError(t, err)
	require.True(t, result.CurrentRecordUploaded)
}
func TestUploadChecksEveryActualImageBeforeFirstMutation(t *testing.T) {
	for _, drift := range []bool{false, true} {
		t.Run(map[bool]string{false: "later image invalid", true: "source bytes changed"}[drift], func(t *testing.T) {
			s, scope, records, merchant, _, _, _ := uploadFixture(t)
			probe := &targetProbe{}
			if drift {
				probe.driftAsset = "square"
			} else {
				probe.failAsset = "square"
			}
			s.dependencies.Images = probe
			_, err := s.Upload(context.Background(), scope, uuid.NewString(), records.saved.ID)
			require.Error(t, err)
			require.Zero(t, merchant.transforms)
			require.Zero(t, merchant.publishes)
		})
	}
}
