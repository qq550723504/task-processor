package currentapplication

import (
	"context"
	"errors"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
	"net/http"
	core "task-processor/internal/core/config"
	"testing"
)

func TestProjectCenterLifecycleDoesNotNeedAgentOrModel(t *testing.T) {
	cfg := runtimeTestConfig()
	cfg.ProjectCenter = &ProjectCenterConfig{Database: DatabaseConfig{Host: "127.0.0.1", Port: 15432, User: "ai_projects_runtime", Password: "fixture-only", Database: "projects", MaxConnections: 4}}
	require.NoError(t, cfg.validate())
	source, projects := &gorm.DB{}, &gorm.DB{}
	stop := errors.New("bounded constructor stop")
	var closed []*gorm.DB
	deps := Dependencies{IdentityPreflight: func(context.Context, IdentityConfig) error { return nil }, OpenSourceAccount: func(context.Context, DatabaseConfig) (*gorm.DB, error) { return source, nil }, OpenProjectCenter: func(context.Context, DatabaseConfig) (*gorm.DB, error) { return projects, nil }, CloseDatabase: func(db *gorm.DB) error { closed = append(closed, db); return nil }, NewApplicationWithFeatures: func(_ context.Context, _ *gorm.DB, f ApplicationFeatures, _ *core.Config, _ *logrus.Logger) (*http.Server, error) {
		require.Equal(t, projects, f.ProjectCenterDB)
		require.Nil(t, f.AIWorkbench)
		require.Nil(t, f.ProductAgent)
		return nil, stop
	}}
	require.ErrorIs(t, Run(context.Background(), cfg, logrus.New(), deps), stop)
	require.Equal(t, []*gorm.DB{projects, source}, closed)
	cfg.ProjectCenter.Database.User = "source_account_runtime"
	require.Error(t, cfg.validate())
}
