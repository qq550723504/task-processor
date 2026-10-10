package productsourcing

import (
	"context"
	"errors"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	catalogstore "task-processor/internal/integration/persistence/product/catalog"
	collectionstore "task-processor/internal/integration/persistence/product/collection"
	"task-processor/internal/product/catalog"
	"task-processor/internal/product/collection"
	"task-processor/internal/product/pod"
	"task-processor/internal/product/sourcing"
	"task-processor/internal/product/supplymarket"
	"testing"
	"time"
)

type receiverAuthority struct{ scope collection.Scope }

func (a receiverAuthority) Authorize(context.Context, string) (collection.Scope, error) {
	return a.scope, nil
}
func TestMarketReceiverUsesActualSourceCatalogCollectionUoWAndKeepsVariantImages(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	container, err := tcpostgres.Run(ctx, "postgres:16-alpine", tcpostgres.WithDatabase("market622"), tcpostgres.WithUsername("market_owner"), tcpostgres.WithPassword("isolated-market-test"), tcpostgres.BasicWaitStrategies())
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, container.Terminate(context.Background())) })
	dsn, err := container.ConnectionString(ctx, "sslmode=disable")
	require.NoError(t, err)
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	require.NoError(t, err)
	pool, err := db.DB()
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, pool.Close()) })
	require.NoError(t, InstallSchema(db))
	require.NoError(t, collectionstore.InstallSchema(db))
	release := supplymarket.Release{ID: uuid.NewString(), Channel: "selected", Revision: 1, Active: true, PublishedAt: time.Now(), Product: supplymarket.PublicProduct{Title: "已发布商品", Images: []string{"https://images.example.org/main.png"}, Variants: []supplymarket.PublicVariant{{SourceID: "blue", Title: "蓝色", Currency: "CNY", Price: 10, Stock: 20, Images: []string{"https://images.example.org/blue.png"}}}}}
	scope := collection.Scope{"org-b", "actor-b", "member-b"}
	op := uuid.NewString()
	err = db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		receiver, err := NewMarketReceiver(tx, receiverAuthority{scope})
		require.NoError(t, err)
		_, err = receiver.Receive(ctx, scope, op, release)
		require.NoError(t, err)
		return errors.New("later market command failure")
	})
	require.Error(t, err)
	var count int64
	for _, table := range []string{"product_snapshot_versions", "product_source_publications", "product_collection_batches", "product_collection_items"} {
		require.NoError(t, db.Table(table).Count(&count).Error)
		require.Zero(t, count, table+" must share rollback")
	}
	var received collection.Receipt
	err = db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		receiver, err := NewMarketReceiver(tx, receiverAuthority{scope})
		if err != nil {
			return err
		}
		received, err = receiver.Receive(ctx, scope, op, release)
		return err
	})
	require.NoError(t, err)
	selected, err := collectionstore.NewRepository(ctx, db, func(tx *gorm.DB) (collectionstore.OwnPublisher, error) {
		return NewOwnProductWriter(tx, receiverAuthority{scope})
	})
	require.NoError(t, err)
	item, err := selected.ReadItem(ctx, scope, received.ItemID)
	require.NoError(t, err)
	require.Equal(t, "market", item.Source.Kind)
	reader, err := catalogstore.NewBoundedSnapshotReader(db, 2<<20)
	require.NoError(t, err)
	saved, err := reader.GetSnapshot(ctx, catalog.SnapshotIdentity{TenantID: scope.OrganizationID, ProductKey: item.Source.ProductKey}, item.Source.Version)
	require.NoError(t, err)
	require.Equal(t, release.Product.Title, saved.Snapshot.Title)
	require.Equal(t, release.Product.Variants[0].Images[0], saved.Snapshot.Variants[0].Images[0].URL)
	require.Equal(t, release.ID, saved.Snapshot.Sources[0].Detail)
	require.Equal(t, op, saved.Snapshot.Sources[0].SourceRunID)
	require.Nil(t, saved.Snapshot.Review)
	_, err = selected.ReadItem(ctx, collection.Scope{"org-b", "another", "another-member"}, received.ItemID)
	require.ErrorIs(t, err, collection.ErrNotFound)
	t.Run("SDS template and finished imports use current owners", func(t *testing.T) {
		template := pod.Template{ID: "95146", Name: "SDS 模板", Images: []string{"https://cdn.sdspod.com/images/test/template.jpg"}, Variants: []pod.TemplateVariant{{ID: "95147", Name: "竖版", Type: "FREE", Images: []string{"https://cdn.sdspod.com/images/test/variant.jpg"}}}}
		for _, kind := range []string{"sds_template", "sds_finished"} {
			t.Run(kind, func(t *testing.T) {
				operation := uuid.NewString()
				var envelope sourcing.SourceEnvelope
				if kind == "sds_template" {
					envelope, err = pod.TemplateEnvelope(scope, operation, template)
				} else {
					p := pod.Plan{OperationID: uuid.NewString(), Scope: scope, Binding: pod.AccountBinding{MerchantID: "123"}, Name: "已核实 SDS 成品", Template: pod.TemplateManifest{ParentID: "95146", VariantID: "95147", RenderFiles: []pod.RenderFile{{ID: "60"}}}}
					f := pod.FinishedReference{OperationID: p.OperationID, MerchantID: "123", ID: "70", KeyID: "finished-test", EvidenceDigest: collection.Digest("qualified"), RenderURLs: []string{"https://cdn.sdspod.com/out/123/test/finished.jpg"}}
					envelope, err = pod.FinishedEnvelope(scope, operation, p, f)
				}
				require.NoError(t, err)
				counts := map[string]int64{}
				for _, table := range []string{"product_snapshot_versions", "product_source_publications", "product_collection_batches", "product_collection_items"} {
					require.NoError(t, db.Table(table).Count(&count).Error)
					counts[table] = count
				}
				err = db.Transaction(func(tx *gorm.DB) error {
					_, e := ReceivePOD(ctx, tx, receiverAuthority{scope}, scope, operation, kind, "SDS 商品", envelope)
					require.NoError(t, e)
					return errors.New("later POD import failure")
				})
				require.Error(t, err)
				for table, before := range counts {
					require.NoError(t, db.Table(table).Count(&count).Error)
					require.Equal(t, before, count, table)
				}
				var receipt collection.Receipt
				err = db.Transaction(func(tx *gorm.DB) error {
					var e error
					receipt, e = ReceivePOD(ctx, tx, receiverAuthority{scope}, scope, operation, kind, "SDS 商品", envelope)
					return e
				})
				require.NoError(t, err)
				item, e := selected.ReadItem(ctx, scope, receipt.ItemID)
				require.NoError(t, e)
				require.Equal(t, kind, item.Source.Kind)
				saved, e := reader.GetSnapshot(ctx, catalog.SnapshotIdentity{TenantID: scope.OrganizationID, ProductKey: item.Source.ProductKey}, item.Source.Version)
				require.NoError(t, e)
				require.Equal(t, envelope.ProductCandidate.Title, saved.Snapshot.Title)
				require.Equal(t, envelope.RawReference.ReferenceID, saved.Snapshot.Sources[0].Detail)
				require.Nil(t, saved.Snapshot.Review)
				_, e = selected.ReadItem(ctx, collection.Scope{scope.OrganizationID, "other", "other-member"}, receipt.ItemID)
				require.ErrorIs(t, e, collection.ErrNotFound)
			})
		}
	})
}
