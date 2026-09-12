package currentapplication

import (
	"context"
	"errors"
	"net"
	"net/http"
	"testing"
	"time"

	"github.com/sirupsen/logrus"
	"gorm.io/gorm"
	coreconfig "task-processor/internal/core/config"
)

func TestMembershipPoolFailureAndShutdownOwnership(t *testing.T) {
	for _, scenario := range []string{"open-error", "open-nil", "source-alias", "commercial-alias", "construct", "cancel-after-open", "listen", "nil-server", "serve"} {
		t.Run(scenario, func(t *testing.T) {
			cfg := runtimeTestConfig()
			cfg.Membership = &MembershipConfig{ProviderOrigin: cfg.Identity.IssuerURL, ReadToken: "read", WriteToken: "write", Database: DatabaseConfig{Host: "127.0.0.1", Port: 5432, User: "organization_membership_runtime", Password: "password", Database: "member", MaxConnections: 2}}
			source, commercial, member := &gorm.DB{}, &gorm.DB{}, &gorm.DB{}
			closed := []*gorm.DB{}
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			deps := runtimeDependencies{IdentityPreflight: func(context.Context, IdentityConfig) error { return nil }, OpenSourceAccount: func(context.Context, DatabaseConfig) (*gorm.DB, error) { return source, nil }, OpenCommercial: func(context.Context, DatabaseConfig) (*gorm.DB, error) { return commercial, nil }, CloseDatabase: func(db *gorm.DB) error { closed = append(closed, db); return nil }}
			deps.OpenMembership = func(openContext context.Context, _ DatabaseConfig) (*gorm.DB, error) {
				deadline, ok := openContext.Deadline()
				if !ok || time.Until(deadline) > 15*time.Second {
					t.Error("missing startup deadline")
				}
				if scenario == "open-error" {
					return nil, errors.New("open failed")
				}
				if scenario == "open-nil" {
					return nil, nil
				}
				if scenario == "source-alias" {
					return source, nil
				}
				if scenario == "commercial-alias" {
					return commercial, nil
				}
				if scenario == "cancel-after-open" {
					cancel()
				}
				return member, nil
			}
			deps.NewApplicationWithMembership = func(_ context.Context, a, b, c *gorm.DB, _ *coreconfig.Config, m *MembershipConfig, _ *logrus.Logger) (*http.Server, error) {
				if a != source || b != commercial || c != member || m != cfg.Membership {
					t.Error("wrong dependencies")
				}
				if scenario == "construct" {
					return nil, errors.New("construct failed")
				}
				if scenario == "nil-server" {
					return nil, nil
				}
				return &http.Server{}, nil
			}
			deps.Listen = func(string, string) (net.Listener, error) {
				if scenario == "listen" {
					return nil, errors.New("listen failed")
				}
				l, err := net.Listen("tcp", "127.0.0.1:0")
				if err == nil {
					_ = l.Close()
				}
				return l, err
			}
			if err := run(ctx, cfg, logrus.New(), deps); err == nil {
				t.Fatal("failure reported success")
			}
			expected := []*gorm.DB{member, commercial, source}
			if scenario == "open-error" || scenario == "open-nil" || scenario == "source-alias" || scenario == "commercial-alias" {
				expected = expected[1:]
			}
			if len(closed) != len(expected) {
				t.Fatalf("closed %d want %d", len(closed), len(expected))
			}
			for i, db := range expected {
				if closed[i] != db {
					t.Fatal("wrong close order/ownership")
				}
			}
		})
	}
}
