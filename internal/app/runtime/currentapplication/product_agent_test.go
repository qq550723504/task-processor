package currentapplication

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"reflect"
	"testing"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
	coreconfig "task-processor/internal/core/config"

	"task-processor/internal/integration/agent/grsaitext"
)

func agentRuntimeConfig() *Config {
	cfg := runtimeTestConfig()
	owner := cfg.CommercialDatabase
	owner.User = "commercial_owner_runtime"
	cfg.CommercialOwnerDatabase = &owner
	product := cfg.SourceAccountDatabase
	product.User, product.Database = "source_acquisition_runtime", "product"
	cfg.ProductAcquisitionDatabase = &product
	run, review, asset := product, product, product
	run.User, run.Database = "agent_runtime", "agent"
	review.User = "product_review_runtime"
	asset.User, asset.Database = "asset_runtime", "asset"
	cfg.ProductAgent = &ProductAgentConfig{Enabled: true, Database: run, ReviewDatabase: review, AssetDatabase: asset, AllowedOrganizationIDs: []string{"org"}, Steps: 12, ModelCalls: 6, Tokens: 5000000, CostMicros: 5000000, RuntimeSeconds: 120, TextPolicy: grsaitext.AgentTextPolicy{PolicyVersion: "title-review-v1", Currency: "CNY"}}
	return cfg
}

func TestProductAgentConfigRejectsUnboundedOrSplitProductOwner(t *testing.T) {
	if err := agentRuntimeConfig().validate(); err != nil {
		t.Fatal(err)
	}
	for name, change := range map[string]func(*Config){
		"missing commercial":  func(c *Config) { c.CommercialOwnerDatabase = nil },
		"missing acquisition": func(c *Config) { c.ProductAcquisitionDatabase = nil },
		"split product":       func(c *Config) { c.ProductAgent.ReviewDatabase.Database = "different" },
		"unbounded runtime":   func(c *Config) { c.ProductAgent.RuntimeSeconds = 121 },
		"unbounded pool":      func(c *Config) { c.ProductAgent.Database.MaxConnections = 9 },
		"empty admission":     func(c *Config) { c.ProductAgent.AllowedOrganizationIDs = nil },
		"duplicate admission": func(c *Config) { c.ProductAgent.AllowedOrganizationIDs = []string{"org", "org"} },
	} {
		t.Run(name, func(t *testing.T) {
			cfg := agentRuntimeConfig()
			change(cfg)
			if cfg.validate() == nil {
				t.Fatal("invalid agent configuration admitted")
			}
		})
	}
}

func TestProductAgentAndImageGenerationShareManifestWithoutLosingAdmission(t *testing.T) {
	cfg := agentRuntimeConfig()
	cfg.ImageAgent = &ImageAgentConfig{
		Database:        DatabaseConfig{Host: "127.0.0.1", Port: 5432, User: "image_agent_runtime", Password: "fixture-password", Database: "image_agent", MaxConnections: 4},
		TemporalAddress: "127.0.0.1:7233", TemporalNamespace: "default", AllowedOrganizationIDs: []string{"org"},
		PublicBase: "https://images.example.test", Bucket: "image-agent-assets",
		Generation: coreconfig.ImageAgentGenerationConfig{PriceVersion: "price-2026-09", PointsPerImage: 12},
	}
	manifest, err := json.Marshal(cfg)
	require.NoError(t, err)
	var loaded Config
	require.NoError(t, json.Unmarshal(manifest, &loaded))
	require.NoError(t, loaded.validate())
	require.Equal(t, cfg.ProductAgent, loaded.ProductAgent)
	require.Equal(t, cfg.ImageAgent.Generation, loaded.CoreConfig().ImageAgent.Generation)
	loaded.ProductAgent.AllowedOrganizationIDs = nil
	require.Error(t, loaded.validate(), "image admission must not replace Product Agent admission")
	loaded.ProductAgent.AllowedOrganizationIDs = []string{"org"}
	loaded.ImageAgent.Generation.PriceVersion = ""
	require.Error(t, loaded.validate(), "Product Agent must not bypass explicit image pricing")
}

func TestProductAgentPoolsCloseOnPartialStartupAndStayClosedWhenDisabled(t *testing.T) {
	for _, failAt := range []int{0, 1, 2, 3, 4} {
		t.Run(string(rune('0'+failAt)), func(t *testing.T) {
			cfg := agentRuntimeConfig()
			cfg.ProductAgent.Enabled = failAt != 0
			pools := []*gorm.DB{{}, {}, {}, {}, {}, {}, {}}
			var openedAgent int
			var closed []*gorm.DB
			stop := errors.New("bounded fixture stop")
			deps := Dependencies{
				IdentityPreflight:      func(context.Context, IdentityConfig) error { return nil },
				OpenSourceAccount:      func(context.Context, DatabaseConfig) (*gorm.DB, error) { return pools[0], nil },
				OpenCommercial:         func(context.Context, DatabaseConfig) (*gorm.DB, error) { return pools[1], nil },
				OpenCommercialOwner:    func(context.Context, DatabaseConfig) (*gorm.DB, error) { return pools[2], nil },
				OpenProductAcquisition: func(context.Context, DatabaseConfig) (*gorm.DB, error) { return pools[3], nil },
				OpenProductAgent: func(context.Context, DatabaseConfig) (*gorm.DB, error) {
					openedAgent++
					if openedAgent == failAt {
						return nil, stop
					}
					return pools[3+openedAgent], nil
				},
				NewApplicationWithFeatures: func(_ context.Context, _, _ *gorm.DB, f ApplicationFeatures, _ *coreconfig.Config, _ *logrus.Logger) (*http.Server, error) {
					if failAt == 4 && (f.ProductAgentDB != pools[4] || f.ProductReviewDB != pools[5] || f.ProductAgentAssetDB != pools[6]) {
						t.Fatal("wrong owner pools")
					}
					return nil, stop
				},
				CloseDatabase: func(db *gorm.DB) error { closed = append(closed, db); return nil },
			}
			if Run(context.Background(), cfg, logrus.New(), deps) == nil {
				t.Fatal("expected startup stop")
			}
			count := 4
			if failAt > 0 {
				count += failAt - 1
			}
			var want []*gorm.DB
			for i := count - 1; i >= 0; i-- {
				want = append(want, pools[i])
			}
			if !reflect.DeepEqual(closed, want) {
				t.Fatalf("closed %d pools, want %d in reverse order", len(closed), len(want))
			}
			if failAt == 0 && openedAgent != 0 {
				t.Fatal("disabled agent opened owner pools")
			}
		})
	}
}
