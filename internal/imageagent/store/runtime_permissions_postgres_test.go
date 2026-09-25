package store

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	"task-processor/internal/imageagent"
)

func TestOrganizationImageRuntimeRoleHasOnlyCurrentAPIGrants(t *testing.T) {
	dsn := os.Getenv("ISSUE487_TEST_DSN")
	if dsn == "" {
		t.Skip("result=SKIP: requires isolated ISSUE487_TEST_DSN")
	}
	require.Contains(t, dsn, "host=127.0.0.1")
	require.Contains(t, dsn, "user=issue487_owner")
	ctx := context.Background()
	root, err := gorm.Open(postgres.Open(dsn), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	require.NoError(t, err)
	rootPool, err := root.DB()
	require.NoError(t, err)
	name := "issue487_image_" + strings.ReplaceAll(uuid.NewString(), "-", "")
	require.NoError(t, root.Exec("CREATE DATABASE "+name).Error)
	owner, err := gorm.Open(postgres.Open(dsn+" dbname="+name), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	require.NoError(t, err)
	ownerPool, err := owner.DB()
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, ownerPool.Close())
		require.NoError(t, root.Exec("DROP DATABASE "+name+" WITH (FORCE)").Error)
		require.NoError(t, rootPool.Close())
	})
	require.NoError(t, AutoMigrateOrganizationScope(owner))
	require.NoError(t, owner.Exec(`CREATE TABLE unrelated_secret(value text)`).Error)
	require.NoError(t, owner.Exec(`DO $$ BEGIN IF NOT EXISTS(SELECT 1 FROM pg_roles WHERE rolname='image_agent_runtime') THEN CREATE ROLE image_agent_runtime LOGIN NOSUPERUSER NOCREATEDB NOCREATEROLE NOREPLICATION NOBYPASSRLS; END IF; END $$`).Error)
	sum := sha256.Sum256([]byte(dsn))
	password := hex.EncodeToString(sum[:])
	require.NoError(t, owner.Exec("ALTER ROLE image_agent_runtime PASSWORD '"+password+"'").Error)
	require.NoError(t, GrantOrganizationRuntimePermissions(ctx, owner))
	runtimeDB, err := gorm.Open(postgres.Open(dsn+" dbname="+name+" user=image_agent_runtime password="+password), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	require.NoError(t, err)
	pool, err := runtimeDB.DB()
	require.NoError(t, err)
	pool.SetMaxOpenConns(8)
	t.Cleanup(func() { require.NoError(t, pool.Close()) })
	require.NoError(t, VerifyOrganizationRuntimePermissions(ctx, runtimeDB))
	require.Error(t, VerifyOrganizationRuntimePermissions(ctx, owner))
	run := manualRun("runtime-start", "org-runtime")
	run.ScopeProtocol = imageagent.OrganizationScopeProtocol
	run.MemberID = "member-runtime"
	run.BusinessTaskID = "operation-runtime"
	run.TargetPlatform = "product"
	run.ImagePolicyContext = imageagent.ImagePolicyContext{Country: "zz", Family: "default", SceneCategory: "general"}
	run.ActivePlanRevision = 1
	run.Version = 1
	plan := planRevision(1)
	scope := imageagent.ScopeForRun(*run)
	repository := NewOrganizationRepository(runtimeDB)
	_, err = repository.InitializeRun(ctx, imageagent.ProjectionInitialization{
		Scope: scope, Run: *run, Plan: plan,
		Catalog: imageagent.AssetCatalog{Assets: []imageagent.AuthorizedAsset{
			{ID: "source-1", Type: imageagent.AuthorizedAssetSource, URL: "https://source.example/source.png"},
			{ID: "style-1", Type: imageagent.AuthorizedAssetStyle, URL: "https://style.example/style.png"},
		}},
		Snapshot: imageagent.RunProjection{Run: *run, Plan: plan}, CommitID: "start:runtime-start",
		EventType: "run.initialized", EventPayload: []byte(`{}`),
	})
	require.NoError(t, err, "current API Start must initialize only its admitted tables")
	_, err = repository.GetProjection(ctx, scope)
	require.NoError(t, err, "current API GET/Approve must read its projection")
	for _, tc := range []struct{ name, grant, revoke string }{
		{"delete_run", "GRANT DELETE ON image_agent_v2_runs TO image_agent_runtime", "REVOKE DELETE ON image_agent_v2_runs FROM image_agent_runtime"},
		{"write_attempt", "GRANT INSERT ON image_agent_v2_attempts TO image_agent_runtime", "REVOKE INSERT ON image_agent_v2_attempts FROM image_agent_runtime"},
		{"other_table", "GRANT SELECT ON unrelated_secret TO image_agent_runtime", "REVOKE SELECT ON unrelated_secret FROM image_agent_runtime"},
		{"missing_commit_insert", "REVOKE INSERT ON image_agent_v2_projection_commits FROM image_agent_runtime", "GRANT INSERT ON image_agent_v2_projection_commits TO image_agent_runtime"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			require.NoError(t, owner.Exec(tc.grant).Error)
			t.Cleanup(func() { require.NoError(t, owner.Exec(tc.revoke).Error) })
			require.Error(t, VerifyOrganizationRuntimePermissions(ctx, runtimeDB))
			require.NoError(t, owner.Exec(tc.revoke).Error)
			require.NoError(t, VerifyOrganizationRuntimePermissions(ctx, runtimeDB))
		})
	}
	require.Error(t, runtimeDB.Exec("DELETE FROM image_agent_v2_runs").Error)
	require.Error(t, runtimeDB.Exec("SELECT value FROM unrelated_secret").Error)
	require.Error(t, runtimeDB.Exec("SELECT id FROM image_agent_v2_asset_catalog").Error)
	require.Error(t, runtimeDB.Exec("UPDATE image_agent_v2_runs SET status='failed'").Error)
}
