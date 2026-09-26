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
	require.Equal(t, coreconfig.ImageAgentGenerationConfig{}, core.ImageAgent.Generation, "no default price opens generation")
	cfg.ImageAgent.Generation = coreconfig.ImageAgentGenerationConfig{PriceVersion: "price-2026-09", PointsPerImage: 12}
	require.Error(t, cfg.validate(), "price without the explicit resource owner must not open generation")
	cfg.CommercialOwnerDatabase = &DatabaseConfig{Host: "127.0.0.1", Port: 5434, User: "commercial_owner_runtime", Password: "fixture-password", Database: "commercial_owner", MaxConnections: 4}
	require.NoError(t, cfg.validate())
	require.Equal(t, cfg.ImageAgent.Generation, cfg.CoreConfig().ImageAgent.Generation)
	for _, price := range []coreconfig.ImageAgentGenerationConfig{{PriceVersion: "version"}, {PointsPerImage: 12}, {PriceVersion: " version ", PointsPerImage: 12}, {PriceVersion: "bad\nversion", PointsPerImage: 12}} {
		cfg.ImageAgent.Generation = price
		require.Error(t, cfg.validate())
	}
	// Restore the original pool-less fixture below: this test's Run lifecycle
	// does not construct the additional resource owner pool.
	cfg.ImageAgent.Generation = coreconfig.ImageAgentGenerationConfig{}
	cfg.CommercialOwnerDatabase = nil

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

func TestCurrentImageAgentTrialGeneratedBaseIsExplicitAndFixed(t *testing.T) {
	cfg := acquisitionRuntimeConfig()
	cfg.ImageAgent = &ImageAgentConfig{
		Database:        DatabaseConfig{Host: "127.0.0.1", Port: 5432, User: "image_agent_runtime", Password: "fixture-password", Database: "image_agent", MaxConnections: 4},
		TemporalAddress: "127.0.0.1:7233", TemporalNamespace: "default", AllowedOrganizationIDs: []string{"org-a"},
		PublicBase: "https://localhost:19444/image-agent-assets/issue487-images", Bucket: "issue487-images", IsolatedTrialGeneratedURLs: true,
	}
	require.NoError(t, cfg.validate())
	require.True(t, cfg.CoreConfig().ImageAgent.ArtifactStore.IsolatedTrialGeneratedURLs)
	cfg.ImageAgent.PublicBase = "https://localhost:19444/image-agent-assets/other"
	require.Error(t, cfg.validate())
	cfg.ImageAgent.PublicBase = "https://localhost:19444/image-agent-assets/issue487-images"
	cfg.ImageAgent.IsolatedTrialGeneratedURLs = false
	require.Error(t, cfg.validate(), "loopback URL must fail without the isolated-trial flag")
}

type fakeImageWorkflowClient struct{ imageagent.WorkflowClient }
