package currentapplication

import (
	"context"
	"errors"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"
	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/mocks"
	"gorm.io/gorm"
	"net"
	"net/http"
	coreconfig "task-processor/internal/core/config"
	"task-processor/internal/integration/sds"
	"task-processor/internal/product/supplymarket"
	"testing"
)

type marketTestStorage struct{}

func (marketTestStorage) PutImmutable(context.Context, supplymarket.PrivateFile, []byte) error {
	return nil
}
func (marketTestStorage) Inspect(context.Context, supplymarket.PrivateFile) (supplymarket.PrivateObject, error) {
	return supplymarket.PrivateObject{}, nil
}
func (marketTestStorage) ReadBounded(context.Context, supplymarket.PrivateFile) ([]byte, error) {
	return nil, nil
}

type podTestCredentials struct{}

func (podTestCredentials) Current(context.Context) (sds.Credentials, error) {
	return sds.Credentials{}, nil
}

type podStartFailureWorker struct{ starts, stops int }

func (w *podStartFailureWorker) Start() error {
	w.starts++
	return errors.New("fixture worker start failure")
}
func (w *podStartFailureWorker) Stop() { w.stops++ }
func TestPODRuntimeAssemblyFailureClosesCanonicalPools(t *testing.T) {
	for _, phase := range []string{"assembly", "worker start", "customization alias", "report alias"} {
		t.Run(phase, func(t *testing.T) {
			c := supplyRuntimeConfig(t)
			c.POD = &PODConfig{AssetDatabase: c.SupplyChain.AssetDatabase, TemporalAddress: c.SupplyChain.TemporalAddress, TemporalNamespace: "default", CredentialFile: writeManifest(t, `{}`), OSSHosts: []string{"fixture.oss-cn-hangzhou.aliyuncs.com"}}
			c.SupplyMarket = &SupplyMarketConfig{Storage: KnowledgeStorageConfig{Region: "local", Bucket: "private", AccessKeyID: "fixture", SecretAccessKey: "fixture", Mode: "aws"}}
			c.StoreCenter = nil
			c.SupplyChain = nil
			if phase == "customization alias" {
				c.AgentCustomizationDatabase = &DatabaseConfig{Host: "127.0.0.1", Port: 5432, User: "agent_customization_runtime", Password: "fixture", Database: "customization", MaxConnections: 2}
			}
			if phase == "report alias" {
				c.ReportCenter = reportRuntimeConfig().ReportCenter
			}
			source, product, owner, assets := &gorm.DB{}, &gorm.DB{}, &gorm.DB{}, &gorm.DB{}
			workflow := &mocks.Client{}
			worker := &podStartFailureWorker{}
			closed := 0
			var pools []*gorm.DB
			err := Run(context.Background(), c, logrus.New(), Dependencies{
				IdentityPreflight:      func(context.Context, IdentityConfig) error { return nil },
				OpenSourceAccount:      func(context.Context, DatabaseConfig) (*gorm.DB, error) { return source, nil },
				OpenProductAcquisition: func(context.Context, DatabaseConfig) (*gorm.DB, error) { return product, nil },
				OpenCommercialOwner:    func(context.Context, DatabaseConfig) (*gorm.DB, error) { return owner, nil },
				OpenSupplyAssets:       func(context.Context, DatabaseConfig) (*gorm.DB, error) { return assets, nil },
				OpenAgentCustomization: func(context.Context, DatabaseConfig) (*gorm.DB, error) { return assets, nil },
				OpenReportCenter:       func(context.Context, DatabaseConfig) (*gorm.DB, error) { return assets, nil },
				NewMarketStorage: func(context.Context, *SupplyMarketConfig, *logrus.Logger) (supplymarket.PrivateFileStorage, error) {
					return marketTestStorage{}, nil
				},
				PreparePOD: func(context.Context, *PODConfig) (sds.CredentialSource, error) { return podTestCredentials{}, nil },
				DialSupplyWorkflow: func(context.Context, string, string) (client.Client, func() error, error) {
					return workflow, func() error { closed++; return nil }, nil
				},
				CloseDatabase: func(db *gorm.DB) error { pools = append(pools, db); return nil },
				Listen:        func(string, string) (net.Listener, error) { return net.Listen("tcp", "127.0.0.1:0") },
				NewApplicationWithFeatures: func(_ context.Context, _ *gorm.DB, f ApplicationFeatures, _ *coreconfig.Config, _ *logrus.Logger) (*http.Server, error) {
					require.Same(t, assets, f.PODAssetDB)
					require.Same(t, workflow, f.PODWorkflow)
					require.NotNil(t, f.MarketStorage)
					require.NotNil(t, f.PODCredentials)
					require.NotNil(t, f.PODWorker)
					if phase == "assembly" || phase == "customization alias" || phase == "report alias" {
						return nil, errors.New("fixture assembly failure")
					}
					*f.PODWorker = worker
					return &http.Server{Handler: http.NotFoundHandler()}, nil
				},
			})
			if phase == "customization alias" || phase == "report alias" {
				require.ErrorContains(t, err, "independent Asset pool")
				require.Zero(t, closed)
			} else if phase == "assembly" {
				require.ErrorContains(t, err, "fixture assembly failure")
			} else {
				require.ErrorContains(t, err, "start POD worker failed")
				require.Equal(t, 1, worker.starts)
				require.Equal(t, 1, worker.stops)
			}
			if phase != "customization alias" && phase != "report alias" {
				require.Equal(t, 1, closed)
			}
			require.Contains(t, pools, assets)
		})
	}
}
