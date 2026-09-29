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
	_ "modernc.org/sqlite"
	resourceadapter "task-processor/internal/integration/orgresource"
	acquisitionstore "task-processor/internal/integration/persistence/product/acquisition"
	"task-processor/internal/ledger/orgresource"
)

func TestAcquisitionActualSeparateResourceOwnerChargesOnlySavedResults(t *testing.T) {
	productDB := acquisitionDatabase(t)
	require.NoError(t, InstallAcquisitionSchema(productDB))
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
	operations, err := acquisitionstore.NewRepository(context.Background(), productDB)
	require.NoError(t, err)
	owner, err := NewAcquisitionChargeOwner(operations)
	require.NoError(t, err)
	repository, err := resourceadapter.NewGormConsumerChargeRepository(resourceDB, resourceadapter.TransactionConfig{})
	require.NoError(t, err)
	charges, err := orgresource.NewConsumerChargeService(repository, map[orgresource.ResourceConsumer]orgresource.ConsumerChargeOwner{orgresource.ConsumerProductAcquisition: owner})
	require.NoError(t, err)
	provider, fetches, _ := acquisitionFixture(t)
	permissions, live := acquisitionPermissionDependencies(t)
	service, err := NewPublicAcquisition(context.Background(), productDB, live, permissions, provider, charges)
	require.NoError(t, err)
	ctx := acquisitionIdentity("org-charge", "actor")
	firstKey := uuid.NewString()
	first, err := service.Acquire(ctx, firstKey, "981645030344")
	require.NoError(t, err)
	replayed, err := service.Acquire(ctx, firstKey, "981645030344")
	require.NoError(t, err)
	require.True(t, replayed.Replayed)
	require.Equal(t, first.Publication.Receipt, replayed.Publication.Receipt)
	require.EqualValues(t, 1, fetches.Load())
	position, err := positions.ReadPosition(context.Background(), "org-charge", "membership:actor", orgresource.ResourceDataRow)
	require.NoError(t, err)
	require.EqualValues(t, 1, position.Free)
	require.EqualValues(t, 1, position.Consumed)
	require.Zero(t, position.Reserved)
	second, err := service.Acquire(ctx, uuid.NewString(), "981645030344")
	require.NoError(t, err)
	require.EqualValues(t, 2, second.Publication.Receipt.CatalogVersion)
	insufficientKey := uuid.NewString()
	_, err = service.Acquire(ctx, insufficientKey, "981645030344")
	require.ErrorIs(t, err, orgresource.ErrInsufficientBalance)
	_, err = service.Acquire(ctx, insufficientKey, "981645030344")
	require.ErrorIs(t, err, orgresource.ErrInsufficientBalance, "replay must preserve the recorded quota rejection")
	require.EqualValues(t, 2, fetches.Load(), "insufficient allocation must fail before provider dispatch")
	position, err = positions.ReadPosition(context.Background(), "org-charge", "membership:actor", orgresource.ResourceDataRow)
	require.NoError(t, err)
	require.Zero(t, position.Free)
	require.Zero(t, position.Reserved)
	require.EqualValues(t, 2, position.Consumed)
	var receipts int64
	require.NoError(t, productDB.Table("product_source_publication_receipts").Count(&receipts).Error)
	require.EqualValues(t, 2, receipts)
}
