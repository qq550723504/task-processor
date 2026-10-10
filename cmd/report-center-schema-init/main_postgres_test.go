//go:build integration

package main

import (
	"context"
	"github.com/stretchr/testify/require"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	"net"
	"net/url"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"runtime"
	"testing"
	"time"
)

func TestNativeReportInitializerRollsBackWithoutRoleAndRefusesNonemptyDatabase(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	container, err := tcpostgres.Run(ctx, "postgres:16-alpine", tcpostgres.WithDatabase("reports"), tcpostgres.WithUsername("reports"), tcpostgres.WithPassword("fixture-only"), tcpostgres.BasicWaitStrategies())
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, container.Terminate(context.Background())) })
	dsn, err := container.ConnectionString(ctx, "sslmode=disable")
	require.NoError(t, err)
	u, err := url.Parse(dsn)
	require.NoError(t, err)
	u.Host = net.JoinHostPort("127.0.0.1", u.Port())
	dsn = u.String()
	owner, err := gorm.Open(postgres.Open(dsn), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	require.NoError(t, err)
	pool, _ := owner.DB()
	t.Cleanup(func() { _ = pool.Close() })
	path := filepath.Join(t.TempDir(), "owner.private.dsn")
	require.NoError(t, os.WriteFile(path, []byte(dsn), 0600))
	if runtime.GOOS == "windows" {
		current, err := user.Current()
		require.NoError(t, err)
		require.NoError(t, exec.Command("icacls", path, "/inheritance:r", "/grant:r", current.Username+":(F)", "SYSTEM:(F)").Run())
	}
	require.Error(t, initialize(ctx, path, "reports", false))
	require.Error(t, initialize(ctx, path, "product", true))
	require.Error(t, initialize(ctx, path, "reports", true))
	var exists bool
	require.NoError(t, owner.Raw("SELECT EXISTS(SELECT 1 FROM pg_namespace WHERE nspname='report_center')").Scan(&exists).Error)
	require.False(t, exists, "failed grants must roll back installation")
	require.NoError(t, owner.Exec(`REVOKE CREATE ON DATABASE reports FROM PUBLIC; REVOKE CREATE ON SCHEMA public FROM PUBLIC; CREATE ROLE report_center_runtime LOGIN PASSWORD 'fixture-runtime' NOSUPERUSER NOCREATEDB NOCREATEROLE NOBYPASSRLS`).Error)
	require.NoError(t, initialize(ctx, path, "reports", true))
	require.Error(t, initialize(ctx, path, "reports", true), "repeat is not a migration")
	var tables int
	require.NoError(t, owner.Raw("SELECT count(*) FROM pg_tables WHERE schemaname='report_center'").Scan(&tables).Error)
	require.Equal(t, 3, tables)
	require.NoError(t, owner.Exec("DROP SCHEMA report_center CASCADE; CREATE TABLE unrelated_owner(id integer)").Error)
	require.Error(t, initialize(ctx, path, "reports", true), "unrelated business tables prevent installation")
	require.NoError(t, owner.Raw("SELECT EXISTS(SELECT 1 FROM pg_namespace WHERE nspname='report_center')").Scan(&exists).Error)
	require.False(t, exists)
}
