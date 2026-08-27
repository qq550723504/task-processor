package store

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"task-processor/internal/imageagent"
)

func TestAutoMigrateCreatesOwnerScopedV2TablesWithoutTouchingLegacySchema(t *testing.T) {
	dsn := fmt.Sprintf("file:image-agent-v2-migration-%d?mode=memory&cache=shared", concurrentSQLiteSequence.Add(1))
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	require.NoError(t, err)
	legacyDDL := []string{
		`CREATE TABLE image_agent_runs (tenant_id text NOT NULL, id text NOT NULL, user_id text, idempotency_key text NOT NULL, PRIMARY KEY (tenant_id,id), UNIQUE (tenant_id,idempotency_key))`,
		`CREATE TABLE image_agent_plans (tenant_id text NOT NULL, run_id text NOT NULL, revision integer NOT NULL, idempotency_key text NOT NULL, PRIMARY KEY (tenant_id,run_id,revision))`,
		`CREATE TABLE image_agent_events (tenant_id text NOT NULL, run_id text NOT NULL, cursor integer NOT NULL, PRIMARY KEY (tenant_id,run_id,cursor))`,
	}
	for _, statement := range legacyDDL {
		require.NoError(t, db.Exec(statement).Error)
	}
	require.NoError(t, db.Exec(`INSERT INTO image_agent_runs (tenant_id,id,user_id,idempotency_key) VALUES ('tenant-a','legacy-run','legacy-user','legacy-key')`).Error)

	require.NoError(t, AutoMigrate(db))
	require.NoError(t, AutoMigrate(db), "v2 schema migration must be repeatable")
	for _, table := range []string{
		"image_agent_v2_runs",
		"image_agent_v2_plans",
		"image_agent_v2_slots",
		"image_agent_v2_attempts",
		"image_agent_v2_events",
		"image_agent_v2_asset_catalog",
		"image_agent_v2_asset_catalog_manifests",
		"image_agent_v2_projection_snapshots",
		"image_agent_v2_projection_commits",
	} {
		require.True(t, db.Migrator().HasTable(table), table)
	}
	var legacyCount int64
	require.NoError(t, db.Raw(`SELECT count(*) FROM image_agent_runs WHERE id = 'legacy-run'`).Scan(&legacyCount).Error)
	require.EqualValues(t, 1, legacyCount, "v1 rows must remain untouched")

	repository := NewGormRepository(db)
	for _, owner := range []string{"user-a", "user-b"} {
		run := manualRun("same-run", "tenant-a")
		run.UserID = owner
		run.IdempotencyKey = "run-key-" + owner
		plan := planRevision(1)
		plan.CreatedBy = owner
		scope := imageagent.ScopeForRun(*run)
		_, err := repository.InitializeRun(context.Background(), imageagent.ProjectionInitialization{
			Scope: scope, Run: *run, Plan: plan,
			Catalog: imageagent.AssetCatalog{Assets: []imageagent.AuthorizedAsset{
				{ID: "source-1", Type: imageagent.AuthorizedAssetSource, URL: "https://source.example/source.png"},
				{ID: "style-1", Type: imageagent.AuthorizedAssetStyle},
			}},
			Snapshot: imageagent.RunProjection{Run: *run, Plan: plan}, CommitID: "start:" + owner,
			EventType: "run.initialized", EventPayload: json.RawMessage(`{}`),
		})
		require.NoError(t, err)
	}
	_, err = repository.GetProjection(context.Background(), imageagent.RunScope{TenantID: "tenant-a", OwnerUserID: "legacy-user", RunID: "legacy-run"})
	require.ErrorIs(t, err, imageagent.ErrRunNotFound, "v1 ownerless rows must be invisible to the v2 repository")
}
