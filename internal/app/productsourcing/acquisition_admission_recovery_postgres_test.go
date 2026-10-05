package productsourcing

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	resourceadapter "task-processor/internal/integration/orgresource"
	acquisitionstore "task-processor/internal/integration/persistence/product/acquisition"
	"task-processor/internal/ledger/orgresource"
	"task-processor/internal/product/sourcing"
)

type interruptedAcquisitionReserve struct {
	orgresource.ConsumerChargePort
	fail, receiptLost bool
}

func (p *interruptedAcquisitionReserve) Reserve(ctx context.Context, id orgresource.ConsumerChargeIdentity) (orgresource.ConsumerChargeReceipt, error) {
	if p.fail {
		if p.receiptLost {
			if _, err := p.ConsumerChargePort.Reserve(ctx, id); err != nil {
				return orgresource.ConsumerChargeReceipt{}, err
			}
		}
		return orgresource.ConsumerChargeReceipt{}, orgresource.ErrConsumerChargeUnknown
	}
	return p.ConsumerChargePort.Reserve(ctx, id)
}

func acquisitionRecoveryResource(t *testing.T, db *gorm.DB) (orgresource.ConsumerChargePort, *resourceadapter.GormMemberAllocationRepository) {
	t.Helper()
	resourceDB, err := gorm.Open(sqlite.New(sqlite.Config{DriverName: "sqlite", DSN: filepath.Join(t.TempDir(), "resource.sqlite")}), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	require.NoError(t, err)
	pool, err := resourceDB.DB()
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, pool.Close()) })
	require.NoError(t, resourceadapter.AutoMigrate(resourceDB))
	require.NoError(t, resourceDB.Exec(`INSERT INTO saas_organization_resource_buckets (organization_id,resource_type,available,allocated,reserved,consumed,created_at,updated_at) VALUES ('org-charge','data_row',2,0,0,0,?,?)`, time.Now().UTC(), time.Now().UTC()).Error)
	positions, err := resourceadapter.NewGormMemberAllocationRepository(resourceDB, resourceadapter.TransactionConfig{})
	require.NoError(t, err)
	_, err = positions.Transfer(context.Background(), orgresource.MemberResourceTransfer{OrganizationID: "org-charge", MemberID: "membership:actor", ActorID: "admin", OperationID: "allocate", ResourceType: orgresource.ResourceDataRow, Action: orgresource.MemberResourceAllocate, Quantity: 2})
	require.NoError(t, err)
	ops, err := acquisitionstore.NewRepository(context.Background(), db)
	require.NoError(t, err)
	owner, err := NewAcquisitionChargeOwner(ops)
	require.NoError(t, err)
	repo, err := resourceadapter.NewGormConsumerChargeRepository(resourceDB, resourceadapter.TransactionConfig{})
	require.NoError(t, err)
	charges, err := orgresource.NewConsumerChargeService(repo, map[orgresource.ResourceConsumer]orgresource.ConsumerChargeOwner{orgresource.ConsumerProductAcquisition: owner})
	require.NoError(t, err)
	return charges, positions
}

func TestAcquisitionAdmissionRecoveryPreservesOriginalReservation(t *testing.T) {
	for _, browser := range []bool{false, true} {
		for _, lost := range []bool{false, true} {
			name := "http"
			if browser {
				name = "browser"
			}
			if lost {
				name += "/receipt-lost"
			} else {
				name += "/before-reserve"
			}
			t.Run(name, func(t *testing.T) {
				db := acquisitionDatabase(t)
				require.NoError(t, InstallAcquisitionSchema(db))
				charges, positions := acquisitionRecoveryResource(t, db)
				interrupted := &interruptedAcquisitionReserve{ConsumerChargePort: charges, fail: true, receiptLost: lost}
				provider, fetches, _ := acquisitionFixture(t)
				permissions, live := acquisitionPermissionDependencies(t)
				core, err := NewPublicAcquisition(context.Background(), db, live, permissions, provider, interrupted)
				require.NoError(t, err)
				acquire := core.Acquire
				if browser {
					service, err := NewBrowserAcquisitionService(core.operations, provider, core.publisher, core.reader, core.authorizer, 0, core.charges)
					require.NoError(t, err)
					acquire = service.Acquire
				}
				ctx := acquisitionIdentity("org-charge", "actor")
				key := uuid.NewString()
				_, err = acquire(ctx, key, "981645030344")
				require.ErrorIs(t, err, orgresource.ErrConsumerChargeUnknown)
				require.Zero(t, fetches.Load())
				op, err := core.operations.ByKey(ctx, sourcing.PublicationScope{OrganizationID: "org-charge", ActorID: "actor"}, key)
				require.NoError(t, err)
				require.Equal(t, sourcing.AcquisitionAcquiring, op.State)
				require.NoError(t, db.Exec("UPDATE public.product_acquisition_operations SET lease_until=clock_timestamp()-interval '1 second' WHERE operation_id=?", op.ID).Error)
				if browser {
					// Original recovery remains possible at the new-operation ceiling.
					for i := 1; i < sourcing.MaxActiveAcquisitionOperations; i++ {
						request, err := core.request(ctx, uuid.NewString(), "981645030344")
						require.NoError(t, err)
						_, claim, err := core.operations.Start(ctx, request)
						require.NoError(t, err)
						require.True(t, claim)
					}
				}
				interrupted.fail = false
				result, err := acquire(ctx, key, "981645030344")
				require.NoError(t, err)
				require.Equal(t, op.ID, result.Operation.ID)
				require.True(t, result.Replayed)
				require.NotNil(t, result.Publication)
				_, err = acquire(ctx, key, "981645030344")
				require.NoError(t, err)
				require.EqualValues(t, 1, fetches.Load())
				position, err := positions.ReadPosition(ctx, "org-charge", "membership:actor", orgresource.ResourceDataRow)
				require.NoError(t, err)
				require.EqualValues(t, 1, position.Free)
				require.EqualValues(t, 1, position.Consumed)
				require.Zero(t, position.Reserved)
			})
		}
	}
}

func TestBrowserCaptureByKeyRecoversWithoutResourceCharge(t *testing.T) {
	db := acquisitionDatabase(t)
	require.NoError(t, InstallAcquisitionSchema(db))
	permissions, live := acquisitionPermissionDependencies(t)
	service, err := NewBrowserAcquisition(context.Background(), db, live, permissions)
	require.NoError(t, err)
	_, request, _ := browserApplicationFixture(t)
	ctx := acquisitionIdentity(request.Scope.OrganizationID, request.Scope.ActorID)
	command := *request.Command
	request.Command = nil
	prepared, ok := service.core.operations.(sourcing.PreparedAcquisitionOperationStore)
	require.True(t, ok)
	op, claimed, err := prepared.StartPrepared(ctx, request, command)
	require.NoError(t, err)
	require.True(t, claimed)
	require.Equal(t, sourcing.AcquisitionPrepared, op.State)
	restarted, err := NewBrowserAcquisition(context.Background(), db, live, permissions)
	require.NoError(t, err)
	result, err := restarted.ByKey(ctx, request.Key)
	require.NoError(t, err)
	require.True(t, result.Replayed)
	require.NotNil(t, result.Publication)
	_, err = restarted.ByKey(ctx, request.Key)
	require.NoError(t, err)
	var chargeIntents int64
	require.NoError(t, db.Table("product_acquisition_charge_intents").Count(&chargeIntents).Error)
	require.Zero(t, chargeIntents, "browser recovery must not create a data-row intent")
}
