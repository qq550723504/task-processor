package currentapplication

import (
	"context"
	"errors"
	"github.com/sirupsen/logrus"
	"gorm.io/gorm"
	"net"
	"net/http"
	"reflect"
	"strings"
	coreconfig "task-processor/internal/core/config"
	"testing"
)

func customizationRuntimeConfig() *Config {
	cfg := runtimeTestConfig()
	db := cfg.SourceAccountDatabase
	db.User, db.Database, db.MaxConnections = "agent_customization_runtime", "agent_customization", 4
	cfg.AgentCustomizationDatabase = &db
	return cfg
}

func TestAgentCustomizationConfigIndependentRestrictedOwner(t *testing.T) {
	for _, kind := range []string{"valid", "source-alias", "notification-alias", "tool-market-alias", "project-alias", "owner-role", "unbounded"} {
		t.Run(kind, func(t *testing.T) {
			cfg := customizationRuntimeConfig()
			switch kind {
			case "source-alias":
				cfg.AgentCustomizationDatabase.Database = cfg.SourceAccountDatabase.Database
			case "notification-alias":
				db := *cfg.AgentCustomizationDatabase
				db.User = "notification_center_runtime"
				cfg.NotificationCenterDatabase = &db
			case "tool-market-alias":
				db := *cfg.AgentCustomizationDatabase
				db.User, db.Database = "tool_market_runtime", "tool_market"
				cfg.ToolMarket = &ToolMarketConfig{Database: db, CaptureAppURL: "https://localhost:31544/capture/1688"}
				cfg.AgentCustomizationDatabase.Database = db.Database
			case "project-alias":
				db := *cfg.AgentCustomizationDatabase
				db.User = "ai_projects_runtime"
				cfg.ProjectCenter = &ProjectCenterConfig{Database: db}
			case "owner-role":
				cfg.AgentCustomizationDatabase.User = "postgres"
			case "unbounded":
				cfg.AgentCustomizationDatabase.MaxConnections = 5
			}
			if err := cfg.validate(); (err == nil) != (kind == "valid") {
				t.Fatalf("%s: %v", kind, err)
			}
		})
	}
}

func TestAgentCustomizationAndToolMarketRuntimeKeepSeparateOwners(t *testing.T) {
	for _, scenario := range []struct {
		name   string
		shared bool
	}{{"distinct", false}, {"shared", true}} {
		t.Run(scenario.name, func(t *testing.T) {
			shared := scenario.shared
			cfg := customizationRuntimeConfig()
			db := *cfg.AgentCustomizationDatabase
			db.User, db.Database = "tool_market_runtime", "tool_market"
			cfg.ToolMarket = &ToolMarketConfig{Database: db, CaptureAppURL: "https://localhost:31544/capture/1688"}
			if cfg.Referrals.Enabled {
				cfg.ToolMarket.CaptureAppURL = cfg.Referrals.PublicAppOrigin + "/capture/1688"
			}
			source, custom, tool := &gorm.DB{}, &gorm.DB{}, &gorm.DB{}
			if shared {
				tool = custom
			}
			constructed := false
			stop := errors.New("bounded composition stop")
			deps := Dependencies{
				IdentityPreflight:      func(context.Context, IdentityConfig) error { return nil },
				OpenSourceAccount:      func(context.Context, DatabaseConfig) (*gorm.DB, error) { return source, nil },
				OpenAgentCustomization: func(context.Context, DatabaseConfig) (*gorm.DB, error) { return custom, nil },
				OpenToolMarket:         func(context.Context, DatabaseConfig) (*gorm.DB, error) { return tool, nil },
				CloseDatabase:          func(*gorm.DB) error { return nil },
				NewApplicationWithFeatures: func(_ context.Context, _ *gorm.DB, features ApplicationFeatures, _ *coreconfig.Config, _ *logrus.Logger) (*http.Server, error) {
					constructed = true
					if features.AgentCustomizationDB != custom || features.ToolMarketDB != tool || features.ToolMarket != cfg.ToolMarket {
						t.Fatal("independent feature owner lost during combination")
					}
					return nil, stop
				},
			}
			err := Run(context.Background(), cfg, logrus.New(), deps)
			if shared {
				if constructed || err == nil || !strings.Contains(err.Error(), "independent owner pool") {
					t.Fatalf("shared pool reached assembly: constructed=%v err=%v", constructed, err)
				}
			} else if !constructed || !errors.Is(err, stop) {
				t.Fatalf("distinct owners failed combination: constructed=%v err=%v", constructed, err)
			}
		})
	}
}

func TestAgentCustomizationAndProjectCenterRuntimeKeepSeparateOwners(t *testing.T) {
	for _, scenario := range []struct {
		name   string
		shared bool
	}{{"distinct", false}, {"shared", true}} {
		t.Run(scenario.name, func(t *testing.T) {
			shared := scenario.shared
			cfg := customizationRuntimeConfig()
			db := *cfg.AgentCustomizationDatabase
			db.User, db.Database = "ai_projects_runtime", "ai_projects"
			cfg.ProjectCenter = &ProjectCenterConfig{Database: db}
			source, custom, tool := &gorm.DB{}, &gorm.DB{}, &gorm.DB{}
			if shared {
				tool = custom
			}
			constructed := false
			stop := errors.New("bounded composition stop")
			deps := Dependencies{
				IdentityPreflight:      func(context.Context, IdentityConfig) error { return nil },
				OpenSourceAccount:      func(context.Context, DatabaseConfig) (*gorm.DB, error) { return source, nil },
				OpenAgentCustomization: func(context.Context, DatabaseConfig) (*gorm.DB, error) { return custom, nil },
				OpenProjectCenter:      func(context.Context, DatabaseConfig) (*gorm.DB, error) { return tool, nil },
				CloseDatabase:          func(*gorm.DB) error { return nil },
				NewApplicationWithFeatures: func(_ context.Context, _ *gorm.DB, features ApplicationFeatures, _ *coreconfig.Config, _ *logrus.Logger) (*http.Server, error) {
					constructed = true
					if features.AgentCustomizationDB != custom || features.ProjectCenterDB != tool {
						t.Fatal("independent feature owner lost during combination")
					}
					return nil, stop
				},
			}
			err := Run(context.Background(), cfg, logrus.New(), deps)
			if shared {
				if constructed || err == nil || !strings.Contains(err.Error(), "independent pool") {
					t.Fatalf("shared pool reached assembly: constructed=%v err=%v", constructed, err)
				}
			} else if !constructed || !errors.Is(err, stop) {
				t.Fatalf("distinct owners failed combination: constructed=%v err=%v", constructed, err)
			}
		})
	}
}

func TestAgentCustomizationRuntimeOptInFailureAndClose(t *testing.T) {
	for _, stage := range []string{"disabled", "missing", "open", "construct", "listen", "cancel"} {
		t.Run(stage, func(t *testing.T) {
			cfg := customizationRuntimeConfig()
			if stage == "disabled" {
				cfg.AgentCustomizationDatabase = nil
			}
			source, custom := &gorm.DB{}, &gorm.DB{}
			var closed []*gorm.DB
			opened := 0
			stop := errors.New("bounded fixture stop")
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			deps := Dependencies{
				IdentityPreflight: func(context.Context, IdentityConfig) error { return nil },
				OpenSourceAccount: func(context.Context, DatabaseConfig) (*gorm.DB, error) { return source, nil },
				OpenAgentCustomization: func(ctx context.Context, db DatabaseConfig) (*gorm.DB, error) {
					opened++
					if _, ok := ctx.Deadline(); !ok {
						t.Fatal("missing bounded startup context")
					}
					if db != *cfg.AgentCustomizationDatabase {
						t.Fatal("wrong owner configuration")
					}
					if stage == "open" {
						return nil, stop
					}
					if stage == "cancel" {
						cancel()
					}
					return custom, nil
				},
				NewApplicationWithFeatures: func(_ context.Context, _ *gorm.DB, features ApplicationFeatures, _ *coreconfig.Config, _ *logrus.Logger) (*http.Server, error) {
					if stage != "disabled" && features.AgentCustomizationDB != custom {
						t.Fatal("owner lost during composition")
					}
					if stage == "cancel" {
						t.Fatal("canceled startup constructed application")
					}
					if stage == "construct" {
						return nil, stop
					}
					return &http.Server{}, nil
				},
				CloseDatabase: func(db *gorm.DB) error { closed = append(closed, db); return nil },
				Listen:        func(string, string) (net.Listener, error) { return nil, stop },
			}
			if stage == "missing" {
				deps.OpenAgentCustomization = nil
			}
			if err := Run(ctx, cfg, logrus.New(), deps); err == nil {
				t.Fatal("expected bounded stop")
			}
			want := []*gorm.DB{source}
			if stage == "missing" {
				want = nil
			}
			if stage != "disabled" && stage != "missing" && stage != "open" {
				want = []*gorm.DB{custom, source}
			}
			if !reflect.DeepEqual(closed, want) {
				t.Fatalf("closed %v, want %v", closed, want)
			}
			if (stage == "disabled" || stage == "missing") && opened != 0 {
				t.Fatal("unavailable feature opened pool")
			}
		})
	}
}
