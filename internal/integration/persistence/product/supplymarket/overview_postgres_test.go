package supplymarketpersistence

import (
	"context"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	"task-processor/internal/product/collection"
	"testing"
	"time"
)

func TestApplicationCountsUseWholeDatabaseAndExactMemberKind(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	container, err := tcpostgres.Run(ctx, "postgres:16-alpine", tcpostgres.WithDatabase("overview"), tcpostgres.WithUsername("test_owner"), tcpostgres.WithPassword("isolated-test"), tcpostgres.BasicWaitStrategies())
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
	insert := func(org, actor, member, kind, stage string, n int) {
		for i := 0; i < n; i++ {
			require.NoError(t, db.Exec(`INSERT INTO supply_market_records(id,organization_id,actor_id,member_id,kind,stage,revision,record_json,created_at) VALUES(?,?,?,?,?,?,1,'{}',now())`, uuid.NewString(), org, actor, member, kind, stage).Error)
		}
	}
	for stage, n := range map[string]int{"SUBMITTED": 25, "EVALUATING": 2, "SUPPLEMENT_REQUIRED": 4, "APPROVED": 7, "REJECTED": 3} {
		insert("org-a", "actor-a", "member-a", "selected", stage, n)
	}
	insert("org-b", "actor-a", "member-a", "selected", "SUBMITTED", 10)
	insert("org-a", "actor-b", "member-a", "selected", "SUBMITTED", 10)
	insert("org-a", "actor-a", "member-b", "selected", "SUBMITTED", 10)
	insert("org-a", "actor-a", "member-a", "connection", "SUBMITTED", 10)
	insert("org-a", "actor-a", "member-a", "official", "APPROVED", 10)
	r := &Repository{db: db}
	counts, err := r.CountApplications(ctx, collection.Scope{"org-a", "actor-a", "member-a"})
	require.NoError(t, err)
	require.EqualValues(t, 27, counts.Reviewing)
	require.EqualValues(t, 4, counts.SupplementRequired)
	require.EqualValues(t, 7, counts.Approved)
	empty, err := r.CountApplications(ctx, collection.Scope{"empty-org", "empty-actor", "empty-member"})
	require.NoError(t, err)
	require.Zero(t, empty)
	_, err = r.CountApplications(ctx, collection.Scope{})
	require.Error(t, err)
}
