package imageagentworker

import (
	"os"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	"task-processor/internal/imageagent"
	"task-processor/internal/listingsubscription"
)

func TestOrganizationMainAdmissionRealCommercialQuotaDeniesBeforeAnyProvider(t *testing.T) {
	dsn := os.Getenv("ISSUE487_TEST_DSN")
	if dsn == "" {
		t.Skip("requires isolated PostgreSQL ISSUE487_TEST_DSN")
	}
	root, err := gorm.Open(postgres.Open(dsn), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	require.NoError(t, err)
	schema := "issue487_admission_" + strings.ReplaceAll(uuid.NewString(), "-", "")
	require.NoError(t, root.Exec("CREATE SCHEMA "+schema).Error)
	db, err := gorm.Open(postgres.Open(dsn+" search_path="+schema), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	require.NoError(t, err)
	t.Cleanup(func() {
		pool, _ := db.DB()
		_ = pool.Close()
		_ = root.Exec("DROP SCHEMA " + schema + " CASCADE").Error
		rootPool, _ := root.DB()
		_ = rootPool.Close()
	})
	require.NoError(t, listingsubscription.AutoMigrateRepository(db))
	require.NoError(t, db.Exec(`CREATE TABLE account_member_token_locks (organization_id text PRIMARY KEY, updated_at timestamptz NOT NULL)`).Error)
	require.NoError(t, db.Exec(`CREATE TABLE account_member_token_allocations (organization_id text NOT NULL, member_id text NOT NULL, metric text NOT NULL, allocated bigint NOT NULL, version bigint NOT NULL, active boolean NOT NULL, window_start timestamptz NOT NULL, window_end timestamptz NOT NULL, updated_at timestamptz NOT NULL, PRIMARY KEY (organization_id,member_id,metric))`).Error)
	start, end := time.Now().UTC().Add(-time.Hour), time.Now().UTC().Add(time.Hour)
	require.NoError(t, db.Exec(`INSERT INTO saas_tenant_entitlements (tenant_id,module_code,status,starts_at,expires_at,limits) VALUES (?,?,?,?,?,?)`, "org-1", listingsubscription.ModuleListingKit, listingsubscription.StatusActive, start, end, `{"ai_tokens":1000}`).Error)
	require.NoError(t, db.Exec(`INSERT INTO account_member_token_allocations (organization_id,member_id,metric,allocated,version,active,window_start,window_end,updated_at) VALUES (?,?,?,?,?,?,?,?,?)`, "org-1", "member-1", "token", 1, 1, true, start, end, start).Error)
	delegate := &recordingMainExecutor{}
	reservation := listingsubscription.AIInvocationUsageAdapter{Repository: listingsubscription.NewGormRepository(db)}
	executor := organizationMainSlotExecutor{delegate: delegate, quoter: fixedMainReviewQuoter{quote: mainReviewQuote()}, reservation: reservation}
	_, err = executor.GenerateQuotedSlot(mainAdmissionContext(), mainAdmissionInput(), mainGenerationQuote())
	require.Error(t, err)
	require.Equal(t, imageagent.ProviderNotDispatched, imageagent.ProviderDispatchStateOf(err))
	require.Zero(t, delegate.generateCalls, "commercial member admission precedes Extract, Render and Review")
	var reservations int64
	require.NoError(t, db.Table("saas_usage_events").Where("source_type = ?", "ai_invocation_reservation").Count(&reservations).Error)
	require.Zero(t, reservations, "denied admission cannot create a held reservation")
}
