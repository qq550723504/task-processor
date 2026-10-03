package currentapplication

import (
	"context"
	"errors"
	"net"
	"net/http"
	"strings"
	"testing"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	"github.com/sirupsen/logrus"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"

	coreconfig "task-processor/internal/core/config"
)

func auditUsageTestConfig() *Config {
	cfg := runtimeTestConfig()
	cfg.AccountAuditUsage = &AccountAuditUsageConfig{
		Image:   DatabaseConfig{Host: "127.0.0.1", Port: 15432, User: "account_audit_image_reader", Password: "secret", Database: "image_agent", MaxConnections: 2},
		Product: DatabaseConfig{Host: "127.0.0.1", Port: 15432, User: "account_audit_product_reader", Password: "secret", Database: "product_agent", MaxConnections: 2},
	}
	return cfg
}

func TestAccountAuditUsageRejectsUnreadableLedgerBeforeServing(t *testing.T) {
	cfg := auditUsageTestConfig()
	source := &gorm.DB{}
	imageSQL, imageMock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer imageSQL.Close()
	productSQL, _, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer productSQL.Close()
	image, err := gorm.Open(postgres.New(postgres.Config{Conn: imageSQL, PreferSimpleProtocol: true}), &gorm.Config{DisableAutomaticPing: true})
	if err != nil {
		t.Fatal(err)
	}
	product, err := gorm.Open(postgres.New(postgres.Config{Conn: productSQL, PreferSimpleProtocol: true}), &gorm.Config{DisableAutomaticPing: true})
	if err != nil {
		t.Fatal(err)
	}
	// A role may SELECT invocation_id but lack tenant_id or other columns read
	// by ListObservedUsage. The startup probe must run that full owner query.
	imageMock.ExpectQuery(`SELECT "invocation_id","member_id","prompt_tokens","completion_tokens","total_tokens","finished_at" FROM "ai_invocations" WHERE tenant_id`).WillReturnError(errors.New("permission denied for column tenant_id"))
	closed := []*gorm.DB{}
	err = run(context.Background(), cfg, logrus.New(), Dependencies{
		IdentityPreflight: func(context.Context, IdentityConfig) error { return nil },
		OpenSourceAccount: func(context.Context, DatabaseConfig) (*gorm.DB, error) { return source, nil },
		OpenAccountAuditUsage: func(_ context.Context, target DatabaseConfig) (*gorm.DB, error) {
			if target.Database == "image_agent" {
				return image, nil
			}
			return product, nil
		},
		CloseDatabase: func(db *gorm.DB) error { closed = append(closed, db); return nil },
		NewApplicationWithFeatures: func(context.Context, *gorm.DB, ApplicationFeatures, *coreconfig.Config, *logrus.Logger) (*http.Server, error) {
			t.Fatal("served with unreadable ledger")
			return nil, nil
		},
	})
	if err == nil || !strings.Contains(err.Error(), "permission denied for column tenant_id") || len(closed) != 3 || closed[0] != product || closed[1] != image || closed[2] != source {
		t.Fatalf("failure=%v closed=%v", err, closed)
	}
	if err := imageMock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestAccountAuditUsageRequiresBothIndependentReadOnlyOwners(t *testing.T) {
	for name, mutate := range map[string]func(*Config){
		"missing product":         func(c *Config) { c.AccountAuditUsage.Product = DatabaseConfig{} },
		"wrong image role":        func(c *Config) { c.AccountAuditUsage.Image.User = "image_agent_owner" },
		"wrong product database":  func(c *Config) { c.AccountAuditUsage.Product.Database = "image_agent" },
		"other host":              func(c *Config) { c.AccountAuditUsage.Image.Port++ },
		"unbounded pool":          func(c *Config) { c.AccountAuditUsage.Product.MaxConnections = 9 },
		"image execution overlap": func(c *Config) { c.ImageAgent = &ImageAgentConfig{} },
	} {
		t.Run(name, func(t *testing.T) {
			cfg := auditUsageTestConfig()
			mutate(cfg)
			if err := cfg.validate(); err == nil {
				t.Fatal("invalid audit source accepted")
			}
		})
	}
	if err := auditUsageTestConfig().validate(); err != nil {
		t.Fatalf("valid audit sources: %v", err)
	}
	cfg := auditUsageTestConfig()
	cfg.ImageAgent = &ImageAgentConfig{}
	if err := cfg.AccountAuditUsage.validate(cfg); err == nil {
		t.Fatal("image execution and audit-only source overlap")
	}
	cfg = auditUsageTestConfig()
	cfg.ProductAgent = &ProductAgentConfig{Enabled: true}
	if err := cfg.AccountAuditUsage.validate(cfg); err == nil {
		t.Fatal("product execution and audit-only source overlap")
	}
}

func TestAccountAuditUsagePoolsFailClosedAndCloseBeforeServing(t *testing.T) {
	cfg := auditUsageTestConfig()
	source, image := &gorm.DB{}, &gorm.DB{}
	closed := []*gorm.DB{}
	want := errors.New("product audit source unavailable")
	opened := []string{}
	err := run(context.Background(), cfg, logrus.New(), Dependencies{
		IdentityPreflight: func(context.Context, IdentityConfig) error { return nil },
		OpenSourceAccount: func(context.Context, DatabaseConfig) (*gorm.DB, error) { return source, nil },
		OpenAccountAuditUsage: func(_ context.Context, target DatabaseConfig) (*gorm.DB, error) {
			opened = append(opened, target.Database)
			if target.Database == "image_agent" {
				return image, nil
			}
			return nil, want
		},
		CloseDatabase: func(db *gorm.DB) error { closed = append(closed, db); return nil },
		NewApplicationWithFeatures: func(context.Context, *gorm.DB, ApplicationFeatures, *coreconfig.Config, *logrus.Logger) (*http.Server, error) {
			t.Fatal("served with incomplete audit sources")
			return nil, nil
		},
		Listen: func(string, string) (net.Listener, error) {
			t.Fatal("listened with incomplete audit sources")
			return nil, nil
		},
	})
	if !errors.Is(err, want) {
		t.Fatalf("run error = %v", err)
	}
	if len(opened) != 2 || opened[0] != "image_agent" || opened[1] != "product_agent" {
		t.Fatalf("opened %v", opened)
	}
	if len(closed) != 2 || closed[0] != image || closed[1] != source {
		t.Fatalf("closed %v", closed)
	}
}
