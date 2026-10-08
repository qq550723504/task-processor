package currentapplication

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"
	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/mocks"
	"gorm.io/gorm"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	coreconfig "task-processor/internal/core/config"
	"task-processor/internal/storecenter"
	"testing"
)

func TestSupplyManifestRejectsIncompleteOwnerAssembly(t *testing.T) {
	for _, feature := range []string{
		`{}`, `{ "assetDatabase": {"host":"127.0.0.1","port":5432,"user":"supply_asset_runtime","password":"fixture","database":"assets","maxConnections":2}, "temporalAddress":"127.0.0.1:7233", "temporalNamespace":"default" }`,
	} {
		cfg := acquisitionRuntimeConfig()
		encoded, err := json.Marshal(cfg)
		require.NoError(t, err)
		var manifest map[string]json.RawMessage
		require.NoError(t, json.Unmarshal(encoded, &manifest))
		manifest["supplyChain"] = json.RawMessage(feature)
		encoded, err = json.Marshal(manifest)
		require.NoError(t, err)
		require.NoError(t, json.Unmarshal(encoded, cfg))
		require.Error(t, cfg.validate(), "supply cannot serve without collections, Store applications and live membership credentials")
	}
}

func TestSupplyRuntimePassesCanonicalPoolsAndClosesWorkflowOnConstructionFailure(t *testing.T) {
	c := supplyRuntimeConfig(t)
	c.SourceMedia = &coreconfig.ImageAgentArtifactStoreConfig{Enabled: true, Provider: "s3", PublicBase: "https://files.example.org/products", S3: coreconfig.ImageAgentArtifactStoreS3Config{Bucket: "products", Region: "us-east-1", AccessKeyID: "synthetic", SecretAccessKey: "synthetic", ArtifactMode: "aws"}}
	app := c.StoreCenter.OfficialApplications[0]
	require.NoError(t, os.WriteFile(app.AppSecretFile, []byte(strings.Repeat("synthetic", 4)), 0600))
	require.NoError(t, os.WriteFile(app.CredentialKeyFile, []byte(base64.StdEncoding.EncodeToString(make([]byte, 32))), 0600))
	privatizeSyntheticTestFile(t, app.AppSecretFile)
	privatizeSyntheticTestFile(t, app.CredentialKeyFile)
	source, product, owner, store, assets := &gorm.DB{}, &gorm.DB{}, &gorm.DB{}, &gorm.DB{}, &gorm.DB{}
	workflow := &mocks.Client{}
	closed := false
	var pools []*gorm.DB
	err := Run(context.Background(), c, logrus.New(), Dependencies{
		IdentityPreflight:      func(context.Context, IdentityConfig) error { return nil },
		OpenSourceAccount:      func(context.Context, DatabaseConfig) (*gorm.DB, error) { return source, nil },
		OpenProductAcquisition: func(context.Context, DatabaseConfig) (*gorm.DB, error) { return product, nil },
		OpenCommercialOwner:    func(context.Context, DatabaseConfig) (*gorm.DB, error) { return owner, nil },
		OpenStoreCenter:        func(context.Context, DatabaseConfig) (*gorm.DB, error) { return store, nil },
		OpenSupplyAssets: func(_ context.Context, db DatabaseConfig) (*gorm.DB, error) {
			require.Equal(t, c.SupplyChain.AssetDatabase, db)
			return assets, nil
		},
		DialSupplyWorkflow: func(context.Context, string, string) (client.Client, func() error, error) {
			return workflow, func() error { closed = true; return nil }, nil
		},
		CloseDatabase: func(db *gorm.DB) error { pools = append(pools, db); return nil },
		NewApplicationWithFeatures: func(_ context.Context, _ *gorm.DB, f ApplicationFeatures, _ *coreconfig.Config, _ *logrus.Logger) (*http.Server, error) {
			require.Same(t, assets, f.SupplyAssetDB)
			require.Same(t, workflow, f.SupplyWorkflow)
			require.NotNil(t, f.SupplyWorker)
			require.NotNil(t, f.SourceMediaStorage)
			require.Contains(t, f.SourceMediaStorage.PublicURL("fixture"), "https://files.example.org/products/")
			return nil, errors.New("fixture assembly failure")
		},
	})
	require.ErrorContains(t, err, "fixture assembly failure")
	require.True(t, closed)
	require.Contains(t, pools, assets)
}

func supplyRuntimeConfig(t *testing.T) *Config {
	c := acquisitionRuntimeConfig()
	c.ProductCollections = true
	c.Identity.TenantDirectoryToken = "fixture-token"
	root := t.TempDir()
	c.StoreCenter = &StoreCenterConfig{Enabled: true, Database: DatabaseConfig{Host: "127.0.0.1", Port: 5432, User: "store_center_runtime", Password: "fixture", Database: "stores", MaxConnections: 2}, OfficialApplications: []OfficialStoreConnectionConfig{{Type: storecenter.ApplicationSelfOperated, AppID: "fixture-app", Version: "v1", APIOrigin: "https://openapi.sheincorp.com", CallbackURL: "https://localhost/callback", AppSecretFile: filepath.Join(root, "secret"), CredentialKeyFile: filepath.Join(root, "key"), CredentialKeyID: "fixture-key"}}}
	c.SupplyChain = &SupplyChainConfig{AssetDatabase: DatabaseConfig{Host: "127.0.0.1", Port: 5432, User: "supply_asset_runtime", Password: "fixture", Database: "assets", MaxConnections: 2}, TemporalAddress: "127.0.0.1:7233", TemporalNamespace: "default"}
	return c
}
func TestSupplyManifestPinsExistingAssetOwnerAndDedicatedRole(t *testing.T) {
	c := supplyRuntimeConfig(t)
	require.NoError(t, c.validate())
	for name, mutate := range map[string]func(*Config){
		"missing membership":   func(c *Config) { c.Identity.TenantDirectoryToken = "" },
		"missing collections":  func(c *Config) { c.ProductCollections = false },
		"missing applications": func(c *Config) { c.StoreCenter.OfficialApplications = nil },
		"wrong role":           func(c *Config) { c.SupplyChain.AssetDatabase.User = "product_agent_runtime" },
		"shared Product":       func(c *Config) { c.SupplyChain.AssetDatabase.Database = "product" },
		"split Asset fact": func(c *Config) {
			c.ImageAgent = &ImageAgentConfig{Database: DatabaseConfig{Host: "127.0.0.1", Port: 5432, Database: "another_asset"}}
		},
		"unbounded workflow endpoint": func(c *Config) { c.SupplyChain.TemporalAddress = "external.example:7233" },
	} {
		t.Run(name, func(t *testing.T) { c := supplyRuntimeConfig(t); mutate(c); require.Error(t, c.validate()) })
	}
}
