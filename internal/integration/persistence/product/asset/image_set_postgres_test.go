package assetpersistence

import (
	"context"
	"errors"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	"os"
	"strings"
	productasset "task-processor/internal/product/asset"
	"testing"
	"time"
)

func TestPostgresImageSetConcurrentSaveHasOneHeadWinner(t *testing.T) {
	dsn := os.Getenv("ISSUE487_TEST_DSN")
	if dsn == "" {
		t.Skip("requires isolated PostgreSQL ISSUE487_TEST_DSN")
	}
	for _, initial := range []bool{true, false} {
		t.Run(map[bool]string{true: "empty_head", false: "existing_head"}[initial], func(t *testing.T) {
			root, err := gorm.Open(postgres.Open(dsn), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
			require.NoError(t, err)
			schema := "images612_" + strings.ReplaceAll(uuid.NewString(), "-", "")
			require.NoError(t, root.Exec("CREATE SCHEMA "+schema).Error)
			var pools []*gorm.DB
			t.Cleanup(func() {
				for _, db := range pools {
					pool, _ := db.DB()
					_ = pool.Close()
				}
				_ = root.Exec("DROP SCHEMA " + schema + " CASCADE").Error
				pool, _ := root.DB()
				_ = pool.Close()
			})
			for i := 0; i < 2; i++ {
				db, err := gorm.Open(postgres.Open(approvalSchemaDSN(dsn, schema)), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
				require.NoError(t, err)
				pools = append(pools, db)
			}
			require.NoError(t, AutoMigrate(pools[0]))
			writers := make([]productasset.Repository, 2)
			for i, db := range pools {
				writers[i], err = NewRepository(db)
				require.NoError(t, err)
			}
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			head := productasset.ImageInventoryHead{}
			if !initial {
				first := setCommit("first", head)
				_, err = writers[0].CommitApproval(ctx, first)
				require.NoError(t, err)
				hash, err := approvalPayloadHash(first)
				require.NoError(t, err)
				head = productasset.ImageInventoryHead{ActionID: first.ActionID, PayloadHash: hash}
			}
			start := make(chan struct{})
			done := make(chan error, 2)
			for i, writer := range writers {
				action := []string{"save-a", "save-b"}[i]
				go func() { <-start; _, err := writer.CommitApproval(ctx, setCommit(action, head)); done <- err }()
			}
			close(start)
			wins, conflicts := 0, 0
			for i := 0; i < 2; i++ {
				err := <-done
				if err == nil {
					wins++
				} else if errors.Is(err, productasset.ErrApprovalConflict) {
					conflicts++
				} else {
					t.Fatal(err)
				}
			}
			require.Equal(t, 1, wins)
			require.Equal(t, 1, conflicts)
			inventory, err := writers[0].(productasset.ImageSetInventoryReader).ReadImageSetInventory(ctx, productasset.InventoryScope{TenantID: "org-source", ProductKey: "product-source", TargetPlatform: "product", SourceSnapshotVersion: 1})
			require.NoError(t, err)
			require.Len(t, inventory.Assets, 2)
			require.Contains(t, []string{"save-a", "save-b"}, inventory.Head.ActionID)
			var receipts int64
			require.NoError(t, pools[0].Model(&ApprovalReceiptRecord{}).Where("action_id IN ?", []string{"save-a", "save-b"}).Count(&receipts).Error)
			require.EqualValues(t, 1, receipts)
		})
	}
}
