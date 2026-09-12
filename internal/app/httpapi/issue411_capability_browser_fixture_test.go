//go:build issue357

package httpapi

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	"task-processor/internal/app/runtime/currentapplication"
)

// This opt-in fixture changes only test assembly configuration. It uses the
// owned run's official local identity provider, production authorization,
// current context/routes and existing PostgreSQL pools. Production RUN-1's
// manifest does not acquire a new platform-admin configuration surface.
func TestIssue411CapabilityBrowserFixture(t *testing.T) {
	if os.Getenv("ISSUE411_CAPABILITY_FIXTURE") != "1" {
		t.Skip("requires an explicitly owned local issue357 run")
	}
	_, dir := issue357ReadConfig(t)
	raw, err := os.ReadFile(filepath.Join(dir, "manifest.json"))
	require.NoError(t, err)
	var manifest struct {
		RunID     string `json:"runId"`
		SourceSHA string `json:"sourceSha"`
		Users     map[string]struct {
			ID string `json:"id"`
		} `json:"users"`
	}
	require.NoError(t, json.Unmarshal(raw, &manifest))
	require.Equal(t, filepath.Base(dir), manifest.RunID)
	require.Equal(t, os.Getenv("ISSUE411_EXPECTED_SHA"), manifest.SourceSHA)
	require.Len(t, manifest.SourceSHA, 40)
	require.NotEmpty(t, manifest.Users["viewer"].ID)
	cfg, err := currentapplication.LoadConfig(filepath.Join(dir, "current-application.json"))
	require.NoError(t, err)
	open := func(dbCfg currentapplication.DatabaseConfig) *gorm.DB {
		dsn := fmt.Sprintf("host=127.0.0.1 port=%d user=%s password=%s dbname=%s sslmode=disable", dbCfg.Port, dbCfg.User, dbCfg.Password, dbCfg.Database)
		db, openErr := gorm.Open(postgres.Open(dsn), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
		require.True(t, openErr == nil, "owned database unavailable")
		pool, poolErr := db.DB()
		require.NoError(t, poolErr)
		pool.SetMaxOpenConns(2)
		t.Cleanup(func() { require.NoError(t, pool.Close()) })
		return db
	}
	source, commercial := open(cfg.SourceAccountDatabase), open(cfg.CommercialDatabase)
	core := cfg.CoreConfig()
	core.ListingKit.PlatformAdminUsers = []string{manifest.Users["viewer"].ID}
	core.ListingKit.PlatformAdminRoles = []string{"issue411_support_admin"}
	log := logrus.New()
	log.SetOutput(io.Discard)
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	server, err := NewCurrentApplication(ctx, source, commercial, core, log)
	cancel()
	require.NoError(t, err)
	listener := httptest.NewServer(server.Handler)
	t.Cleanup(listener.Close)
	issue357Write(t, filepath.Join(dir, "issue411-capability-fixture.json"), map[string]any{
		"runId": manifest.RunID, "sourceSha": manifest.SourceSHA, "goOrigin": listener.URL,
		"configuration":  "test-only configured platform user and role; production current application",
		"configuredUser": manifest.Users["viewer"].ID, "configuredRole": "issue411_support_admin",
	})
	stop := filepath.Join(dir, "stop-issue411-capability")
	ticker := time.NewTicker(200 * time.Millisecond)
	defer ticker.Stop()
	timeout := time.NewTimer(30 * time.Minute)
	defer timeout.Stop()
	for {
		select {
		case <-ticker.C:
			if _, stopErr := os.Stat(stop); stopErr == nil {
				return
			}
		case <-timeout.C:
			t.Fatal("owned capability fixture timed out")
		}
	}
}
