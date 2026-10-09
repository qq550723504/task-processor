//go:build integration

package storecenter

import (
	"context"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	pg "github.com/testcontainers/testcontainers-go/modules/postgres"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	currentstore "task-processor/internal/storecenter"
	"testing"
	"time"
)

type observationInstallMember struct{}

func (observationInstallMember) AuthorizeStoreMember(context.Context, string) (currentstore.StoreMemberAccess, error) {
	return currentstore.StoreMemberAccess{OrganizationID: "org-a", ActorID: "actor-a", MemberID: "member-a", Administrator: true, CanWrite: true}, nil
}

func TestObservationExplicitInstallRejectsBusinessFactsAndExistingSchema(t *testing.T) {
	for _, nonempty := range []bool{false, true} {
		t.Run(map[bool]string{false: "fresh", true: "business-facts"}[nonempty], func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
			defer cancel()
			c, err := pg.Run(ctx, "postgres:16-alpine", pg.WithDatabase("fresh_store"), pg.WithUsername("installer"), pg.WithPassword(uuid.NewString()), pg.BasicWaitStrategies())
			require.NoError(t, err)
			t.Cleanup(func() { _ = c.Terminate(context.Background()) })
			dsn, err := c.ConnectionString(ctx, "sslmode=disable")
			require.NoError(t, err)
			db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
			require.NoError(t, err)
			pool, err := db.DB()
			require.NoError(t, err)
			t.Cleanup(func() { _ = pool.Close() })
			require.NoError(t, Migrate(ctx, db))
			require.NoError(t, db.Exec(`CREATE ROLE store_center_runtime`).Error)
			if nonempty {
				repo, err := currentstore.NewMemberScopedStoreRepository(db, observationInstallMember{})
				require.NoError(t, err)
				store, err := currentstore.NewStore(currentstore.CreateStoreInput{ID: uuid.NewString(), OrganizationID: "org-a", ActorSubject: "actor-a", Name: "Synthetic Store", Platform: "shein", Region: "US", ExternalStoreID: "synthetic-store", CreateIdempotencyKey: uuid.NewString(), OccurredAt: time.Now().UTC()})
				require.NoError(t, err)
				_, _, err = repo.CreateOrReplay(ctx, "org-a", store)
				require.NoError(t, err)
				require.Error(t, InstallObservations(ctx, db))
				var absent bool
				require.NoError(t, db.Raw(`SELECT to_regnamespace('shein_observations') IS NULL`).Scan(&absent).Error)
				require.True(t, absent, "existing business facts must not cause new schema or grants")
				return
			}
			require.NoError(t, InstallObservations(ctx, db))
			require.Error(t, InstallObservations(ctx, db), "existing schema is not migrated or overwritten")
			var exact bool
			require.NoError(t, db.Raw(`SELECT has_schema_privilege('store_center_runtime','shein_observations','USAGE') AND has_table_privilege('store_center_runtime','shein_observations.commands','INSERT') AND NOT has_table_privilege('store_center_runtime','shein_observations.commands','UPDATE')`).Scan(&exact).Error)
			require.True(t, exact)
		})
	}
}
