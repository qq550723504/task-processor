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

func TestProjectCenterRejectsEveryLogicalOwnerDatabaseEvenWithDifferentRoles(t *testing.T) {
	for name, configure := range map[string]func(*Config, DatabaseConfig){
		"source":        func(c *Config, d DatabaseConfig) { c.SourceAccountDatabase = d },
		"commercial":    func(c *Config, d DatabaseConfig) { c.CommercialOwnerDatabase = &d },
		"money":         func(c *Config, d DatabaseConfig) { c.MoneyOwnerDatabase = &d },
		"notification":  func(c *Config, d DatabaseConfig) { c.NotificationCenterDatabase = &d },
		"acquisition":   func(c *Config, d DatabaseConfig) { c.ProductAcquisitionDatabase = &d },
		"store":         func(c *Config, d DatabaseConfig) { c.StoreCenter = &StoreCenterConfig{Database: d} },
		"knowledge":     func(c *Config, d DatabaseConfig) { c.Knowledge = &KnowledgeConfig{Database: d} },
		"membership":    func(c *Config, d DatabaseConfig) { c.Membership = &MembershipConfig{Database: d} },
		"ecoservices":   func(c *Config, d DatabaseConfig) { c.Ecoservices = &EcoservicesConfig{Database: d} },
		"tool":          func(c *Config, d DatabaseConfig) { c.ToolMarket = &ToolMarketConfig{Database: d} },
		"chat":          func(c *Config, d DatabaseConfig) { c.AIWorkbench = &AIWorkbenchConfig{Database: d} },
		"product":       func(c *Config, d DatabaseConfig) { c.ProductAgent = &ProductAgentConfig{Database: d} },
		"review":        func(c *Config, d DatabaseConfig) { c.ProductAgent = &ProductAgentConfig{ReviewDatabase: d} },
		"asset":         func(c *Config, d DatabaseConfig) { c.ProductAgent = &ProductAgentConfig{AssetDatabase: d} },
		"image":         func(c *Config, d DatabaseConfig) { c.ImageAgent = &ImageAgentConfig{Database: d} },
		"supply":        func(c *Config, d DatabaseConfig) { c.SupplyChain = &SupplyChainConfig{AssetDatabase: d} },
		"referrals":     func(c *Config, d DatabaseConfig) { c.Referrals.Enabled = true; c.Referrals.Database = d },
		"trial":         func(c *Config, d DatabaseConfig) { c.LocalTrial = &LocalTrialConfig{Enabled: true, Database: d} },
		"audit-image":   func(c *Config, d DatabaseConfig) { c.AccountAuditUsage = &AccountAuditUsageConfig{Image: d} },
		"audit-product": func(c *Config, d DatabaseConfig) { c.AccountAuditUsage = &AccountAuditUsageConfig{Product: d} },
	} {
		t.Run(name, func(t *testing.T) {
			cfg := runtimeTestConfig()
			d := DatabaseConfig{Host: "127.0.0.1", Port: 15432, User: "ai_projects_runtime", Password: "fixture-only", Database: "projects", MaxConnections: 4}
			cfg.ProjectCenter = &ProjectCenterConfig{Database: d}
			other := d
			other.User = "other_runtime"
			other.Password = "different-fixture"
			configure(cfg, other)
			require.ErrorContains(t, cfg.validateProjectCenter(), "dedicated database")
			cfg.ProjectCenter.Database.Database = "separate_projects"
			require.NoError(t, cfg.validateProjectCenter())
		})
	}
}
