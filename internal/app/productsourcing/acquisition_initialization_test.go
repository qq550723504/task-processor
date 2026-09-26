package productsourcing

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/require"
)

func TestAcquisitionExplicitInitializerRequiresEmptyConfirmedDatabase(t *testing.T) {
	db := acquisitionDatabase(t)
	parsed, err := pgx.ParseConfig(os.Getenv("ISSUE398_TEST_DSN"))
	require.NoError(t, err)
	var name string
	require.NoError(t, db.Raw("SELECT current_database()").Scan(&name).Error)
	require.NoError(t, db.Exec(`DO $$ BEGIN IF NOT EXISTS(SELECT 1 FROM pg_roles WHERE rolname='source_acquisition_runtime') THEN CREATE ROLE source_acquisition_runtime LOGIN NOSUPERUSER NOCREATEDB NOCREATEROLE NOREPLICATION NOBYPASSRLS; END IF; END $$`).Error)
	manifest := filepath.Join(t.TempDir(), "init.json")
	raw, err := json.Marshal(map[string]any{"schemaVersion": 1, "database": map[string]any{"host": "127.0.0.1", "port": parsed.Port, "user": parsed.User, "password": parsed.Password, "database": name}})
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(manifest, raw, 0600))
	require.Error(t, InitializeAcquisitionDatabase(context.Background(), manifest, "another-database"))
	var count int64
	require.NoError(t, db.Raw("SELECT count(*) FROM information_schema.tables WHERE table_schema='public'").Scan(&count).Error)
	require.Zero(t, count)
	require.NoError(t, InitializeAcquisitionDatabase(context.Background(), manifest, name))
	require.NoError(t, db.Raw("SELECT count(*) FROM information_schema.tables WHERE table_schema='public'").Scan(&count).Error)
	require.EqualValues(t, 5, count)
	require.Error(t, InitializeAcquisitionDatabase(context.Background(), manifest, name), "never migrate or repair a nonempty database")
}

func TestAcquisitionInitializerRejectsUnsafeManifestWithoutConnecting(t *testing.T) {
	for _, body := range []string{`{}`, `{"schemaVersion":1,"schemaVersion":1}`, `{"schemaVersion":1,"database":{"host":"remote.example"}}`} {
		path := filepath.Join(t.TempDir(), "init.json")
		require.NoError(t, os.WriteFile(path, []byte(body), 0600))
		require.Error(t, InitializeAcquisitionDatabase(context.Background(), path, "product"))
	}
	require.Error(t, InitializeAcquisitionDatabase(context.Background(), "relative.json", "product"))
}
