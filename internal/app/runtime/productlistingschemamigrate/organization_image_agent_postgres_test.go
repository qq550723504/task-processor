package productlistingschemamigrate

import (
	"context"
	"os"
	"regexp"
	"slices"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

var trialDBNamePattern = regexp.MustCompile(`(?:^|\s)dbname=([^\s]+)`)

func TestOrganizationImageAgentInitCreatesOnlyCurrentOwnerTablesPostgres(t *testing.T) {
	dsn := os.Getenv("ISSUE487_TEST_DSN")
	if dsn == "" || os.Getenv("ISSUE487_EXCLUSIVE_POSTGRES") != "ISOLATED_TRIAL_ONLY" {
		t.Skip("requires an explicitly exclusive issue 487 PostgreSQL cluster")
	}
	match := trialDBNamePattern.FindStringSubmatch(dsn)
	if len(match) != 2 || match[1] == "postgres" || match[1] == "template0" || match[1] == "template1" {
		t.Fatal("test DSN must name an explicit non-system database")
	}
	admin, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	adminPool, err := admin.DB()
	require.NoError(t, err)
	t.Cleanup(func() { _ = adminPool.Close() })
	name := "issue487_orginit_" + strings.ReplaceAll(uuid.NewString(), "-", "")[:16]
	require.NoError(t, admin.Exec(`CREATE DATABASE "`+name+`"`).Error)
	trialDSN := strings.Replace(dsn, "dbname="+match[1], "dbname="+name, 1)
	var trialPool *gorm.DB
	t.Cleanup(func() {
		if trialPool != nil {
			pool, poolErr := trialPool.DB()
			if poolErr == nil {
				_ = pool.Close()
			}
		}
		if dropErr := admin.Exec(`DROP DATABASE "` + name + `"`).Error; dropErr != nil {
			t.Errorf("drop exact temporary database %s: %v", name, dropErr)
		}
	})
	trialPool, err = gorm.Open(postgres.Open(trialDSN), &gorm.Config{})
	require.NoError(t, err)
	ctx := context.Background()
	require.NoError(t, migrateEmptyOrganizationImageAgent(ctx, trialPool))
	var names []string
	require.NoError(t, trialPool.Raw(`SELECT tablename FROM pg_tables WHERE schemaname='public' ORDER BY tablename`).Scan(&names).Error)
	want := []string{
		"ai_client_credentials", "ai_invocations",
		"image_agent_v2_asset_catalog", "image_agent_v2_asset_catalog_manifests", "image_agent_v2_attempts", "image_agent_v2_events", "image_agent_v2_plans", "image_agent_v2_projection_commits", "image_agent_v2_projection_snapshots", "image_agent_v2_runs", "image_agent_v2_slot_external_effects", "image_agent_v2_slots", "image_agent_v3_slot_external_effects",
		"product_approval_receipts", "product_approved_assets", "product_approved_inventory_heads", "product_approved_inventory_version_heads",
	}
	slices.Sort(want)
	require.Equal(t, want, names)
	var indexes []string
	require.NoError(t, trialPool.Raw(`SELECT indexname FROM pg_indexes WHERE schemaname='public' AND indexname IN ('image_agent_org_run_identity','image_agent_org_idempotency') ORDER BY indexname`).Scan(&indexes).Error)
	require.Equal(t, []string{"image_agent_org_idempotency", "image_agent_org_run_identity"}, indexes)
	require.Error(t, migrateEmptyOrganizationImageAgent(ctx, trialPool), "nonempty owner database must not be reinitialized")
	pool, err := trialPool.DB()
	require.NoError(t, err)
	require.NoError(t, pool.Close())
	trialPool, err = gorm.Open(postgres.Open(trialDSN), &gorm.Config{})
	require.NoError(t, err)
	var after []string
	require.NoError(t, trialPool.Raw(`SELECT tablename FROM pg_tables WHERE schemaname='public' ORDER BY tablename`).Scan(&after).Error)
	require.Equal(t, names, after, "restart must read the same owner tables without migration")
}
