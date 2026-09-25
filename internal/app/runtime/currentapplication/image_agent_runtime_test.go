package currentapplication

import (
	"context"
	"errors"
	"net/http"
	"testing"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
	coreconfig "task-processor/internal/core/config"
	"task-processor/internal/imageagent"
)

func TestCurrentImageAgentRequiresExplicitOwnedRuntimeAndNeverFallsBack(t *testing.T) {
	cfg := acquisitionRuntimeConfig()
	cfg.ImageAgent = &ImageAgentConfig{
		Database:        DatabaseConfig{Host: "127.0.0.1", Port: 5432, User: "image_agent_runtime", Password: "fixture-password", Database: "image_agent", MaxConnections: 4},
		TemporalAddress: "127.0.0.1:7233", TemporalNamespace: "default", AllowedOrganizationIDs: []string{"org-a"},
		PublicBase: "https://images.example.test", Bucket: "image-agent-assets",
	}
	require.NoError(t, cfg.validate())
	for _, mutate := range []func(*Config){
		func(c *Config) { c.ProductAcquisitionDatabase = nil },
		func(c *Config) { c.ImageAgent.Database.User = "source_acquisition_runtime" },
		func(c *Config) { c.ImageAgent.Database.Database = c.ProductAcquisitionDatabase.Database },
		func(c *Config) { c.ImageAgent.AllowedOrganizationIDs = nil },
		func(c *Config) { c.ImageAgent.PublicBase = "http://127.0.0.1/private" },
	} {
		copy := *cfg
		image := *cfg.ImageAgent
		copy.ImageAgent = &image
		mutate(&copy)
		require.Error(t, copy.validate())
	}
	core := cfg.CoreConfig()
	require.True(t, core.ImageAgent.Admission.Enabled)
	require.Equal(t, []string{"org-a"}, core.ImageAgent.Admission.AllowedTenantIDs)
	require.Equal(t, "https://images.example.test", core.ImageAgent.ArtifactStore.PublicBase)

	source, commercial, product, imageDB := &gorm.DB{}, &gorm.DB{}, &gorm.DB{}, &gorm.DB{}
	var closed []*gorm.DB
	var workflowClosed bool
	stop := errors.New("stop before listener")
	err := Run(context.Background(), cfg, logrus.New(), Dependencies{
		IdentityPreflight:      func(context.Context, IdentityConfig) error { return nil },
		OpenSourceAccount:      func(context.Context, DatabaseConfig) (*gorm.DB, error) { return source, nil },
		OpenCommercial:         func(context.Context, DatabaseConfig) (*gorm.DB, error) { return commercial, nil },
		OpenProductAcquisition: func(context.Context, DatabaseConfig) (*gorm.DB, error) { return product, nil },
		OpenImageAgent: func(_ context.Context, got DatabaseConfig) (*gorm.DB, error) {
			require.Equal(t, cfg.ImageAgent.Database, got)
			return imageDB, nil
		},
		DialImageAgentWorkflow: func(_ context.Context, address, namespace string) (imageagent.WorkflowClient, func() error, error) {
			require.Equal(t, "127.0.0.1:7233", address)
			require.Equal(t, "default", namespace)
			return fakeImageWorkflowClient{}, func() error { workflowClosed = true; return nil }, nil
		},
		NewApplicationWithFeatures: func(_ context.Context, a, b *gorm.DB, features ApplicationFeatures, _ *coreconfig.Config, _ *logrus.Logger) (*http.Server, error) {
			require.Same(t, source, a)
			require.Same(t, commercial, b)
			require.Same(t, product, features.ProductAcquisitionDB)
			require.Same(t, imageDB, features.ImageAgentDB)
			require.NotNil(t, features.ImageAgentWorkflow)
			return nil, stop
		},
		CloseDatabase: func(db *gorm.DB) error { closed = append(closed, db); return nil },
	})
	require.ErrorIs(t, err, stop)
	require.Equal(t, []*gorm.DB{imageDB, product, commercial, source}, closed)
	require.True(t, workflowClosed)
}

type fakeImageWorkflowClient struct{ imageagent.WorkflowClient }
