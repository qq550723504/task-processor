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

func reportRuntimeConfig() *Config {
	c := runtimeTestConfig()
	c.ReportCenter = &ReportCenterConfig{Database: DatabaseConfig{Host: "127.0.0.1", Port: 15432, User: "report_center_runtime", Password: "fixture-only", Database: "reports", MaxConnections: 4}}
	return c
}
func TestReportCenterLifecycleIndependentSourcesAndCleanup(t *testing.T) {
	for _, openError := range []bool{false, true} {
		cfg := reportRuntimeConfig()
		require.NoError(t, cfg.validate())
		source, reports := &gorm.DB{}, &gorm.DB{}
		stop := errors.New("fixture stop")
		var closed []*gorm.DB
		deps := Dependencies{IdentityPreflight: func(context.Context, IdentityConfig) error { return nil }, OpenSourceAccount: func(context.Context, DatabaseConfig) (*gorm.DB, error) { return source, nil }, OpenReportCenter: func(context.Context, DatabaseConfig) (*gorm.DB, error) {
			if openError {
				return reports, stop
			}
			return reports, nil
		}, CloseDatabase: func(db *gorm.DB) error { closed = append(closed, db); return nil }, NewApplicationWithFeatures: func(_ context.Context, _ *gorm.DB, f ApplicationFeatures, _ *core.Config, _ *logrus.Logger) (*http.Server, error) {
			require.Same(t, reports, f.ReportCenterDB)
			require.Nil(t, f.ProductAgent)
			require.Nil(t, f.SupplyAssetDB)
			return nil, stop
		}}
		require.Error(t, Run(context.Background(), cfg, logrus.New(), deps))
		require.Equal(t, []*gorm.DB{reports, source}, closed)
	}
}
func TestReportCenterRejectsOwnerAliasAndElevatedConfig(t *testing.T) {
	c := reportRuntimeConfig()
	c.ReportCenter.Database.User = "postgres"
	require.Error(t, c.validateReportCenter())
	c = reportRuntimeConfig()
	c.ReportCenter.Database.MaxConnections = 5
	require.Error(t, c.validateReportCenter())
	for name, configure := range map[string]func(*Config, DatabaseConfig){
		"source":        func(c *Config, d DatabaseConfig) { c.SourceAccountDatabase = d },
		"project":       func(c *Config, d DatabaseConfig) { c.ProjectCenter = &ProjectCenterConfig{Database: d} },
		"customization": func(c *Config, d DatabaseConfig) { c.AgentCustomizationDatabase = &d },
		"tool":          func(c *Config, d DatabaseConfig) { c.ToolMarket = &ToolMarketConfig{Database: d} },
		"review":        func(c *Config, d DatabaseConfig) { c.ProductAgent = &ProductAgentConfig{ReviewDatabase: d} },
		"asset":         func(c *Config, d DatabaseConfig) { c.SupplyChain = &SupplyChainConfig{AssetDatabase: d} },
		"trial":         func(c *Config, d DatabaseConfig) { c.LocalTrial = &LocalTrialConfig{Enabled: true, Database: d} },
	} {
		t.Run(name, func(t *testing.T) {
			c := reportRuntimeConfig()
			other := c.ReportCenter.Database
			other.User = "other_runtime"
			configure(c, other)
			require.ErrorContains(t, c.validateReportCenter(), "dedicated database")
		})
	}
}
