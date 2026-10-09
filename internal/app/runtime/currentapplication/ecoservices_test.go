package currentapplication

import (
	"context"
	"encoding/base64"
	"errors"
	"github.com/sirupsen/logrus"
	"gorm.io/gorm"
	"net"
	"net/http"
	"reflect"
	"strings"
	b "task-processor/internal/commercial/billing"
	core "task-processor/internal/core/config"
	"task-processor/internal/knowledge"
	"testing"
)

func ecoservicesTestConfig() *Config {
	c := runtimeTestConfig()
	commercial, money, eco := c.SourceAccountDatabase, c.SourceAccountDatabase, c.SourceAccountDatabase
	commercial.User = "commercial_owner_runtime"
	money.User = "money_owner_runtime"
	eco.Database = "ecoservices"
	eco.User = "ecoservices_runtime"
	c.CommercialOwnerDatabase = &commercial
	c.MoneyOwnerDatabase = &money
	c.Ecoservices = &EcoservicesConfig{Enabled: true, Database: eco, PayloadKey: base64.StdEncoding.EncodeToString(make([]byte, 32)), Storage: KnowledgeStorageConfig{Region: "local", Bucket: "eco", AccessKeyID: "fixture", SecretAccessKey: "fixture", Mode: "aws"}, Payments: EcoservicesPaymentsConfig{Profile: b.ServiceMerchantProfile{Version: "original", Environment: "PRODUCTION", PlatformMerchantID: "platform", AppID: "app", FreezeDays: 180}, PrivateKey: "fixture", SerialNumber: "fixture", APIv3Key: "0123456789abcdef0123456789abcdef", PublicKeyID: "PUB_KEY_ID_fixture", PublicKey: "fixture", NotifyURL: "https://platform.example/api/v1/payments/ecoservices/wechat/notify"}}
	return c
}

func TestEcoservicesAndNotificationRuntimePoolsStayIndependent(t *testing.T) {
	for _, alias := range []bool{false, true} {
		t.Run(map[bool]string{false: "both features", true: "shared injected pool"}[alias], func(t *testing.T) {
			cfg := ecoservicesTestConfig()
			noticeConfig := cfg.SourceAccountDatabase
			noticeConfig.Database = "notification_center"
			noticeConfig.User = "notification_center_runtime"
			cfg.NotificationCenterDatabase = &noticeConfig
			source, commercial, money, eco, notice := &gorm.DB{}, &gorm.DB{}, &gorm.DB{}, &gorm.DB{}, &gorm.DB{}
			if alias {
				eco = notice
			}
			constructed, assembled := 0, 0
			var closed []*gorm.DB
			stop := errors.New("bounded listener stop")
			deps := runtimeDependencies{
				IdentityPreflight:      func(context.Context, IdentityConfig) error { return nil },
				OpenSourceAccount:      func(context.Context, DatabaseConfig) (*gorm.DB, error) { return source, nil },
				OpenCommercialOwner:    func(context.Context, DatabaseConfig) (*gorm.DB, error) { return commercial, nil },
				OpenMoneyOwner:         func(context.Context, DatabaseConfig) (*gorm.DB, error) { return money, nil },
				OpenNotificationCenter: func(context.Context, DatabaseConfig) (*gorm.DB, error) { return notice, nil },
				OpenEcoservices:        func(context.Context, DatabaseConfig) (*gorm.DB, error) { return eco, nil },
				NewEcoservices: func(context.Context, *gorm.DB, *EcoservicesConfig, *logrus.Logger) (*EcoservicesRuntime, error) {
					constructed++
					return &EcoservicesRuntime{DB: eco}, nil
				},
				NewApplicationWithFeatures: func(_ context.Context, _ *gorm.DB, features ApplicationFeatures, _ *core.Config, _ *logrus.Logger) (*http.Server, error) {
					assembled++
					if features.Ecoservices == nil || features.Ecoservices.DB != eco || features.NotificationCenterDB != notice {
						t.Fatal("composition lost a separately enabled feature")
					}
					return &http.Server{}, nil
				},
				Listen:        func(string, string) (net.Listener, error) { return nil, stop },
				CloseDatabase: func(db *gorm.DB) error { closed = append(closed, db); return nil },
			}
			err := run(context.Background(), cfg, logrus.New(), deps)
			want := []*gorm.DB{notice, money, commercial, source}
			if alias {
				if err == nil || !strings.Contains(err.Error(), "ecoservices requires an independent owner pool") || constructed != 0 || assembled != 0 {
					t.Fatalf("aliased pool reached consumer: err=%v constructed=%d assembled=%d", err, constructed, assembled)
				}
			} else {
				want = append([]*gorm.DB{eco}, want...)
				if !errors.Is(err, stop) || constructed != 1 || assembled != 1 {
					t.Fatalf("valid composition failed before listener: %v", err)
				}
			}
			if !reflect.DeepEqual(closed, want) {
				t.Fatal("owner pools were leaked or closed twice")
			}
		})
	}
}

func TestEcoservicesOpeningRequiresDedicatedOwnersAndExplicitQualification(t *testing.T) {
	if err := ecoservicesTestConfig().validate(); err != nil {
		t.Fatal(err)
	}
	for name, change := range map[string]func(*Config){"wide role": func(c *Config) { c.Ecoservices.Database.User = "ecoservices_owner" }, "shared database": func(c *Config) { c.Ecoservices.Database.Database = "task_processor" }, "wide pool": func(c *Config) { c.Ecoservices.Database.MaxConnections = 5 }, "new payments": func(c *Config) { c.Ecoservices.Payments.NewPayments = true }, "new onboarding": func(c *Config) { c.Ecoservices.Payments.NewMerchantApplications = true }, "wrong ingress": func(c *Config) { c.Ecoservices.Payments.NotifyURL = "https://platform.example/other" }, "missing financial owner": func(c *Config) { c.MoneyOwnerDatabase = nil }, "missing protection": func(c *Config) { c.Ecoservices.PayloadKey = "" }} {
		t.Run(name, func(t *testing.T) {
			c := ecoservicesTestConfig()
			change(c)
			if c.validate() == nil {
				t.Fatal("unsafe opening admitted")
			}
		})
	}
	c := ecoservicesTestConfig()
	c.Identity.TenantDirectoryToken = "fixture"
	c.Ecoservices.Payments.NewPayments = true
	c.Ecoservices.Payments.NewMerchantApplications = true
	c.Ecoservices.Payments.ProductQualified = true
	c.Ecoservices.Payments.PlatformPaysFees = true
	if err := c.validate(); err != nil {
		t.Fatal(err)
	}
}

func TestEcoservicesRuntimeReleasesEachOwnerOnceBeforeConstructingAliasedFeature(t *testing.T) {
	for _, stage := range []string{"disabled", "open", "construct", "listen", "alias", "knowledge alias"} {
		t.Run(stage, func(t *testing.T) {
			c := ecoservicesTestConfig()
			c.Ecoservices.Enabled = stage != "disabled"
			source, commercial, money, eco := &gorm.DB{}, &gorm.DB{}, &gorm.DB{}, &gorm.DB{}
			if stage == "alias" {
				eco = source
			}
			if stage == "knowledge alias" {
				c.Knowledge = knowledgeRuntimeTestConfig().Knowledge
			}
			var closed []*gorm.DB
			constructed, knowledgeConstructed := 0, 0
			stop := errors.New("bounded stop")
			deps := runtimeDependencies{
				IdentityPreflight:   func(context.Context, IdentityConfig) error { return nil },
				OpenSourceAccount:   func(context.Context, DatabaseConfig) (*gorm.DB, error) { return source, nil },
				OpenCommercialOwner: func(context.Context, DatabaseConfig) (*gorm.DB, error) { return commercial, nil },
				OpenMoneyOwner:      func(context.Context, DatabaseConfig) (*gorm.DB, error) { return money, nil },
				OpenEcoservices: func(context.Context, DatabaseConfig) (*gorm.DB, error) {
					if stage == "open" {
						return nil, stop
					}
					return eco, nil
				},
				NewEcoservices: func(context.Context, *gorm.DB, *EcoservicesConfig, *logrus.Logger) (*EcoservicesRuntime, error) {
					constructed++
					if stage == "construct" {
						return nil, stop
					}
					return &EcoservicesRuntime{DB: eco}, nil
				},
				OpenKnowledge: func(context.Context, DatabaseConfig) (*gorm.DB, error) { return eco, nil },
				NewKnowledge: func(context.Context, *gorm.DB, *KnowledgeConfig, *logrus.Logger) (*knowledge.Service, *knowledge.Processor, error) {
					knowledgeConstructed++
					return nil, nil, stop
				},
				NewApplicationWithFeatures: func(context.Context, *gorm.DB, ApplicationFeatures, *core.Config, *logrus.Logger) (*http.Server, error) {
					return &http.Server{}, nil
				},
				Listen:        func(string, string) (net.Listener, error) { return nil, stop },
				CloseDatabase: func(db *gorm.DB) error { closed = append(closed, db); return nil },
			}
			if err := run(context.Background(), c, logrus.New(), deps); err == nil {
				t.Fatal("expected bounded stop")
			}
			want := []*gorm.DB{money, commercial, source}
			if stage != "disabled" && stage != "open" && stage != "alias" {
				want = append([]*gorm.DB{eco}, want...)
			}
			if !reflect.DeepEqual(closed, want) {
				t.Fatalf("pool released twice or leaked: got=%v want=%v", closed, want)
			}
			if stage == "alias" && constructed != 0 || stage == "knowledge alias" && knowledgeConstructed != 0 {
				t.Fatal("aliased owner reached consumer construction")
			}
		})
	}
}
