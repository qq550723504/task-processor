package currentapplication

import (
	"context"
	"errors"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"
	"go.temporal.io/sdk/client"
	"gorm.io/gorm"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	core "task-processor/internal/core/config"
	"testing"
)

func dataServicesRuntimeConfig(t *testing.T) *Config {
	c := runtimeTestConfig()
	c.Identity.TenantDirectoryToken = "fixture-only"
	product := DatabaseConfig{Host: "127.0.0.1", Port: 15432, Database: "product_acquisition", User: "source_acquisition_runtime", Password: "fixture-only", MaxConnections: 4}
	resource := product
	resource.Database = "resource"
	resource.User = "commercial_owner_runtime"
	c.ProductAcquisitionDatabase = &product
	c.CommercialOwnerDatabase = &resource
	c.ProductCollections = true
	d := product
	d.User = "data_services_runtime"
	dir := t.TempDir()
	node := "node"
	if runtime.GOOS == "windows" {
		node = "node.exe"
	}
	require.NoError(t, os.Mkdir(filepath.Join(dir, "package"), 0700))
	for _, path := range []string{filepath.Join(dir, node), filepath.Join(dir, "package", "cli.js"), filepath.Join(dir, "chrome")} {
		require.NoError(t, os.WriteFile(path, []byte("unit readiness metadata fixture"), 0600))
	}
	c.DataServices = &DataServicesConfig{Database: d, TemporalAddress: "127.0.0.1:7233", TemporalNamespace: "default", BrowserExecutable: filepath.Join(dir, "chrome"), DriverDirectory: dir, EnabledSites: []string{"us"}}
	return c
}
func TestDataServicesConfigRequiresCanonicalProductAndExplicitRuntime(t *testing.T) {
	c := dataServicesRuntimeConfig(t)
	require.NoError(t, c.validate())
	for name, change := range map[string]func(*Config){
		"no collections":     func(c *Config) { c.ProductCollections = false },
		"no IAM":             func(c *Config) { c.Identity.TenantDirectoryToken = "" },
		"other DB":           func(c *Config) { c.DataServices.Database.Database = "other" },
		"other server":       func(c *Config) { c.DataServices.Database.Port++ },
		"owner":              func(c *Config) { c.DataServices.Database.User = "acquisition_owner" },
		"large pool":         func(c *Config) { c.DataServices.Database.MaxConnections = 5 },
		"relative driver":    func(c *Config) { c.DataServices.DriverDirectory = "relative" },
		"unknown site":       func(c *Config) { c.DataServices.EnabledSites = []string{"xx"} },
		"duplicate site":     func(c *Config) { c.DataServices.EnabledSites = []string{"us", "us"} },
		"remote temporal":    func(c *Config) { c.DataServices.TemporalAddress = "external.example:7233" },
		"invalid proxy CIDR": func(c *Config) { c.DataServices.TrustedProxyCIDRs = []string{"invalid"} },
	} {
		t.Run(name, func(t *testing.T) { cfg := dataServicesRuntimeConfig(t); change(cfg); require.Error(t, cfg.validate()) })
	}
}

type dataClientFixture struct{ client.Client }
type dataWorkerFixture struct {
	starts, stops int
	fail          bool
}

func (w *dataWorkerFixture) Start() error {
	w.starts++
	if w.fail {
		return errors.New("fixture worker failure")
	}
	return nil
}
func (w *dataWorkerFixture) Stop() { w.stops++ }

func TestDataServicesLifecycleFailureClosesOnlyOwnedPoolsAndClient(t *testing.T) {
	for _, stage := range []string{"open", "dial", "construct", "listen", "start", "cancel"} {
		t.Run(stage, func(t *testing.T) {
			cfg := dataServicesRuntimeConfig(t)
			source, product, resource, data := &gorm.DB{}, &gorm.DB{}, &gorm.DB{}, &gorm.DB{}
			var closed []*gorm.DB
			clientCloses := 0
			worker := &dataWorkerFixture{fail: stage == "start"}
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			stop := errors.New("fixture stop")
			deps := Dependencies{
				IdentityPreflight:      func(context.Context, IdentityConfig) error { return nil },
				OpenSourceAccount:      func(context.Context, DatabaseConfig) (*gorm.DB, error) { return source, nil },
				OpenCommercialOwner:    func(context.Context, DatabaseConfig) (*gorm.DB, error) { return resource, nil },
				OpenProductAcquisition: func(context.Context, DatabaseConfig) (*gorm.DB, error) { return product, nil },
				OpenDataServices: func(context.Context, DatabaseConfig) (*gorm.DB, error) {
					if stage == "open" {
						return data, stop
					}
					return data, nil
				},
				DialDataServicesWorkflow: func(context.Context, string, string) (client.Client, func() error, error) {
					if stage == "dial" {
						return &dataClientFixture{}, func() error { clientCloses++; return nil }, stop
					}
					return &dataClientFixture{}, func() error { clientCloses++; return nil }, nil
				},
				CloseDatabase: func(db *gorm.DB) error { closed = append(closed, db); return nil },
				NewApplicationWithFeatures: func(_ context.Context, _ *gorm.DB, f ApplicationFeatures, _ *core.Config, _ *logrus.Logger) (*http.Server, error) {
					require.Same(t, data, f.DataServicesDB)
					require.NotNil(t, f.DataServicesProvider)
					require.NotNil(t, f.DataServicesWorkflow)
					if stage == "construct" {
						return nil, stop
					}
					*f.DataServicesWorker = worker
					return &http.Server{Handler: http.HandlerFunc(func(http.ResponseWriter, *http.Request) {})}, nil
				},
				Listen: func(network, address string) (net.Listener, error) {
					if stage == "listen" {
						return nil, stop
					}
					listener, err := net.Listen(network, "127.0.0.1:0")
					if stage == "cancel" {
						cancel()
					}
					return listener, err
				},
			}
			err := Run(ctx, cfg, logrus.New(), deps)
			if stage == "cancel" {
				require.NoError(t, err)
			} else {
				require.Error(t, err)
			}
			require.Equal(t, []*gorm.DB{data, product, resource, source}, closed)
			if stage == "open" {
				require.Zero(t, clientCloses)
			} else {
				require.Equal(t, 1, clientCloses)
			}
			if stage == "start" || stage == "cancel" {
				require.Equal(t, 1, worker.starts)
				require.Equal(t, 1, worker.stops)
			} else {
				require.Zero(t, worker.starts)
			}
		})
	}
}
