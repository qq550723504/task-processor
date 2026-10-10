package currentapplication

import (
	"context"
	"encoding/base64"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"
	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/mocks"
	"gorm.io/gorm"
	coreconfig "task-processor/internal/core/config"
	"task-processor/internal/imageagent"
)

func TestFullImageGenericManifestNeedsCurrentOwnersWithoutOfficialPlatform(t *testing.T) {
	cfg := fullImageGenericRuntimeConfig(t)
	require.Nil(t, cfg.SupplyChain)
	require.Nil(t, cfg.StoreCenter)
	require.NoError(t, cfg.validate(), "generic image sets must not require an official platform application")
	for name, mutate := range map[string]func(*Config){
		"no Asset":           func(c *Config) { c.ImageAgent.AssetDatabase = DatabaseConfig{} },
		"HTTP role":          func(c *Config) { c.ImageAgent.AssetDatabase.User = "image_agent_runtime" },
		"worker role":        func(c *Config) { c.ImageAgent.AssetDatabase.User = "image_agent_worker_runtime" },
		"different owner":    func(c *Config) { c.ImageAgent.AssetDatabase.Database = "other_assets" },
		"unbounded pool":     func(c *Config) { c.ImageAgent.AssetDatabase.MaxConnections = 9 },
		"no live membership": func(c *Config) { c.Identity.TenantDirectoryToken = "" },
	} {
		t.Run(name, func(t *testing.T) {
			changed := fullImageGenericRuntimeConfig(t)
			mutate(changed)
			require.Error(t, changed.validate())
		})
	}
	platform := supplyRuntimeConfig(t)
	platform.ImageAgent, platform.ProductAgent = cfg.ImageAgent, cfg.ProductAgent
	platform.SupplyChain.AssetDatabase = cfg.ImageAgent.AssetDatabase
	require.NoError(t, platform.validate())
	platform.SupplyChain.AssetDatabase.Password = "another-pool-password"
	require.Error(t, platform.validate(), "two configurations cannot silently open competing Asset pools")
}

func fullImageGenericRuntimeConfig(t *testing.T) *Config {
	t.Helper()
	cfg := acquisitionRuntimeConfig()
	cfg.Identity.TenantDirectoryToken = "isolated-membership-reader"
	cfg.ProductAgent = &ProductAgentConfig{Database: DatabaseConfig{Host: "127.0.0.1", Port: 5432, User: "product_agent_runtime", Password: "fixture", Database: "product_agent", MaxConnections: 4}}
	cfg.ImageAgent = &ImageAgentConfig{
		WorkerConfigFile: filepath.Join(t.TempDir(), "private-worker.yaml"),
		Generation:       coreconfig.ImageAgentGenerationConfig{PriceVersion: "isolated-price", PointsPerImage: 12},
		Database:         DatabaseConfig{Host: "127.0.0.1", Port: 5432, User: "image_agent_runtime", Password: "fixture", Database: "image_agent", MaxConnections: 4},
		TemporalAddress:  "127.0.0.1:7233", TemporalNamespace: "default", AllowedOrganizationIDs: []string{"org-a"},
		PublicBase: "https://images.example.test", Bucket: "image-agent-assets",
		AssetDatabase: DatabaseConfig{Host: "127.0.0.1", Port: 5432, User: "supply_asset_runtime", Password: "fixture", Database: "image_agent", MaxConnections: 4},
	}
	return cfg
}

func TestFullImageGenericRuntimeOpensOnlyItsNarrowAssetPool(t *testing.T) {
	for _, mode := range []string{"independent", "reject HTTP pool", "shared with Supply"} {
		t.Run(mode, func(t *testing.T) {
			cfg := fullImageGenericRuntimeConfig(t)
			platform := mode == "shared with Supply"
			if platform {
				platformConfig := supplyRuntimeConfig(t)
				cfg.StoreCenter, cfg.SupplyChain = platformConfig.StoreCenter, platformConfig.SupplyChain
				cfg.ProductCollections = true
				cfg.SupplyChain.AssetDatabase = cfg.ImageAgent.AssetDatabase
				app := cfg.StoreCenter.OfficialApplications[0]
				require.NoError(t, os.WriteFile(app.AppSecretFile, []byte(strings.Repeat("synthetic", 4)), 0600))
				require.NoError(t, os.WriteFile(app.CredentialKeyFile, []byte(base64.StdEncoding.EncodeToString(make([]byte, 32))), 0600))
				privatizeSyntheticTestFile(t, app.AppSecretFile)
				privatizeSyntheticTestFile(t, app.CredentialKeyFile)
			}
			source, product, configuration, points, imageDB, workerDB, assets, store := &gorm.DB{}, &gorm.DB{}, &gorm.DB{}, &gorm.DB{}, &gorm.DB{}, &gorm.DB{}, &gorm.DB{}, &gorm.DB{}
			if mode == "reject HTTP pool" {
				assets = imageDB
			}
			var closed []*gorm.DB
			var workflowClosed, supplyClosed bool
			supplyDials := 0
			assetOpens := 0
			workflow := &mocks.Client{}
			stop := errors.New("fixture full image construction boundary")
			err := Run(context.Background(), cfg, logrus.New(), Dependencies{
				IdentityPreflight:      func(context.Context, IdentityConfig) error { return nil },
				OpenSourceAccount:      func(context.Context, DatabaseConfig) (*gorm.DB, error) { return source, nil },
				OpenProductAcquisition: func(context.Context, DatabaseConfig) (*gorm.DB, error) { return product, nil },
				OpenProductAgent:       func(context.Context, DatabaseConfig) (*gorm.DB, error) { return configuration, nil },
				OpenCommercialOwner:    func(context.Context, DatabaseConfig) (*gorm.DB, error) { return points, nil },
				OpenStoreCenter:        func(context.Context, DatabaseConfig) (*gorm.DB, error) { return store, nil },
				OpenImageAgent:         func(context.Context, DatabaseConfig) (*gorm.DB, error) { return imageDB, nil },
				OpenImageSetWorker: func(context.Context, string, DatabaseConfig) (*coreconfig.Config, *gorm.DB, error) {
					return &coreconfig.Config{}, workerDB, nil
				},
				DialImageSetWorkflow: func(context.Context, string, string) (client.Client, func() error, error) {
					return workflow, func() error { workflowClosed = true; return nil }, nil
				},
				OpenSupplyAssets: func(_ context.Context, got DatabaseConfig) (*gorm.DB, error) {
					assetOpens++
					require.Equal(t, cfg.ImageAgent.AssetDatabase, got)
					return assets, nil
				},
				DialSupplyWorkflow: func(context.Context, string, string) (client.Client, func() error, error) {
					supplyDials++
					return workflow, func() error { supplyClosed = true; return nil }, nil
				},
				NewApplicationWithFeatures: func(_ context.Context, _ *gorm.DB, f ApplicationFeatures, _ *coreconfig.Config, _ *logrus.Logger) (*http.Server, error) {
					require.Same(t, assets, f.ImageSetAssetDB)
					if platform {
						require.Same(t, assets, f.SupplyAssetDB)
						require.Same(t, workflow, f.SupplyWorkflow)
						require.NotNil(t, f.OfficialStoreApplications)
					} else {
						require.Nil(t, f.SupplyAssetDB)
						require.Nil(t, f.SupplyWorkflow)
						require.Nil(t, f.OfficialStoreApplications)
					}
					require.Same(t, workerDB, f.ImageSetWorkerDB)
					return nil, stop
				},
				CloseDatabase: func(db *gorm.DB) error { closed = append(closed, db); return nil },
			})
			require.Equal(t, 1, assetOpens, "runtime error: %v", err)
			require.True(t, workflowClosed)
			require.Equal(t, platform, supplyClosed)
			if platform {
				require.Equal(t, 1, supplyDials)
			} else {
				require.Zero(t, supplyDials)
			}
			if mode == "reject HTTP pool" {
				require.ErrorContains(t, err, "narrow independently opened pool")
			} else {
				require.ErrorIs(t, err, stop)
				assetCloses := 0
				for _, pool := range closed {
					if pool == assets {
						assetCloses++
					}
				}
				require.Equal(t, 1, assetCloses, "shared Asset owner closes exactly once")
			}
			owned := []*gorm.DB{source, product, configuration, points, imageDB, workerDB}
			if platform {
				owned = append(owned, store)
			}
			for _, pool := range owned {
				count := 0
				for _, got := range closed {
					if got == pool {
						count++
					}
				}
				require.Equal(t, 1, count, "every owned pool closes once")
			}
		})
	}
}

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
	cfg.CommercialOwnerDatabase = nil
	require.Error(t, cfg.validate(), "price without the explicit resource owner must not open generation")
	cfg.CommercialOwnerDatabase = &DatabaseConfig{Host: "127.0.0.1", Port: 5434, User: "commercial_owner_runtime", Password: "fixture-password", Database: "commercial_owner", MaxConnections: 4}
	require.NoError(t, cfg.validate())
	require.Equal(t, cfg.ImageAgent.Generation, cfg.CoreConfig().ImageAgent.Generation)
	for _, price := range []coreconfig.ImageAgentGenerationConfig{{PriceVersion: "version"}, {PointsPerImage: 12}, {PriceVersion: " version ", PointsPerImage: 12}, {PriceVersion: "bad\nversion", PointsPerImage: 12}} {
		cfg.ImageAgent.Generation = price
		require.Error(t, cfg.validate())
	}
	// Acquisition always uses the canonical resource owner, even without AI.
	cfg.ImageAgent.Generation = coreconfig.ImageAgentGenerationConfig{}

	source, product, imageDB, owner := &gorm.DB{}, &gorm.DB{}, &gorm.DB{}, &gorm.DB{}
	var closed []*gorm.DB
	var workflowClosed bool
	stop := errors.New("stop before listener")
	err := Run(context.Background(), cfg, logrus.New(), Dependencies{
		IdentityPreflight: func(context.Context, IdentityConfig) error { return nil },
		OpenSourceAccount: func(context.Context, DatabaseConfig) (*gorm.DB, error) { return source, nil },

		OpenCommercialOwner:    func(context.Context, DatabaseConfig) (*gorm.DB, error) { return owner, nil },
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
		NewApplicationWithFeatures: func(_ context.Context, a *gorm.DB, features ApplicationFeatures, _ *coreconfig.Config, _ *logrus.Logger) (*http.Server, error) {
			require.Same(t, source, a)
			require.Same(t, product, features.ProductAcquisitionDB)
			require.Same(t, imageDB, features.ImageAgentDB)
			require.NotNil(t, features.ImageAgentWorkflow)
			require.Same(t, owner, features.CommercialOwnerDB)
			return nil, stop
		},
		CloseDatabase: func(db *gorm.DB) error { closed = append(closed, db); return nil },
	})
	require.ErrorIs(t, err, stop)
	require.Equal(t, []*gorm.DB{imageDB, product, owner, source}, closed)
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
