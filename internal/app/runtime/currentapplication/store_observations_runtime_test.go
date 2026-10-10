package currentapplication

import (
	"context"
	"encoding/base64"
	"errors"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"
	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/mocks"
	"gorm.io/gorm"
	"net"
	"net/http"
	"os"
	"strings"
	coreconfig "task-processor/internal/core/config"
	"testing"
	"time"
)

type observationProcessWorker struct {
	start   func() error
	stopped int
}

func (w *observationProcessWorker) Start() error { return w.start() }
func (w *observationProcessWorker) Stop()        { w.stopped++ }

func TestStoreObservationAndSupplyReuseOnlyExactTemporalTarget(t *testing.T) {
	for _, same := range []bool{true, false} {
		t.Run(map[bool]string{true: "shared", false: "separate"}[same], func(t *testing.T) {
			c := supplyRuntimeConfig(t)
			address := "127.0.0.1:7233"
			if !same {
				address = "127.0.0.1:7234"
			}
			c.StoreCenter.Observations = &StoreObservationsConfig{TemporalAddress: address, TemporalNamespace: "default"}
			app := c.StoreCenter.OfficialApplications[0]
			require.NoError(t, os.WriteFile(app.AppSecretFile, []byte(strings.Repeat("synthetic", 4)), 0600))
			require.NoError(t, os.WriteFile(app.CredentialKeyFile, []byte(base64.StdEncoding.EncodeToString(make([]byte, 32))), 0600))
			privatizeSyntheticTestFile(t, app.AppSecretFile)
			privatizeSyntheticTestFile(t, app.CredentialKeyFile)
			supply, observations := &mocks.Client{}, &mocks.Client{}
			supplyDials, observationDials, closes := 0, 0, 0
			stop := errors.New("synthetic-construction-stop")
			open := func(context.Context, DatabaseConfig) (*gorm.DB, error) { return &gorm.DB{}, nil }
			err := Run(context.Background(), c, logrus.New(), Dependencies{
				IdentityPreflight: func(context.Context, IdentityConfig) error { return nil }, OpenSourceAccount: open, OpenCommercialOwner: open, OpenProductAcquisition: open, OpenStoreCenter: open, OpenSupplyAssets: open, CloseDatabase: func(*gorm.DB) error { return nil },
				DialSupplyWorkflow: func(context.Context, string, string) (client.Client, func() error, error) {
					supplyDials++
					return supply, func() error { closes++; return nil }, nil
				},
				DialStoreObservationsWorkflow: func(_ context.Context, actual, namespace string) (client.Client, func() error, error) {
					observationDials++
					require.Equal(t, address, actual)
					return observations, func() error { closes++; return nil }, nil
				},
				NewApplicationWithFeatures: func(_ context.Context, _ *gorm.DB, f ApplicationFeatures, _ *coreconfig.Config, _ *logrus.Logger) (*http.Server, error) {
					require.Same(t, supply, f.SupplyWorkflow)
					if same {
						require.Same(t, supply, f.StoreObservationsWorkflow)
					} else {
						require.Same(t, observations, f.StoreObservationsWorkflow)
					}
					return nil, stop
				},
			})
			require.ErrorIs(t, err, stop)
			require.Equal(t, 1, supplyDials)
			if same {
				require.Zero(t, observationDials)
				require.Equal(t, 1, closes)
			} else {
				require.Equal(t, 1, observationDials)
				require.Equal(t, 2, closes)
			}
		})
	}
}

func TestStoreObservationProcessServesOnlyAfterWorkerAndPreservesPools(t *testing.T) {
	for _, stage := range []string{"construct-error", "start-error", "serve"} {
		t.Run(stage, func(t *testing.T) {
			c := supplyRuntimeConfig(t)
			c.ProductAcquisitionDatabase = nil
			c.ProductCollections = false
			c.SupplyChain = nil
			c.StoreCenter.Observations = &StoreObservationsConfig{TemporalAddress: "127.0.0.1:7233", TemporalNamespace: "default"}
			app := c.StoreCenter.OfficialApplications[0]
			require.NoError(t, os.WriteFile(app.AppSecretFile, []byte(strings.Repeat("synthetic", 4)), 0600))
			require.NoError(t, os.WriteFile(app.CredentialKeyFile, []byte(base64.StdEncoding.EncodeToString(make([]byte, 32))), 0600))
			privatizeSyntheticTestFile(t, app.AppSecretFile)
			privatizeSyntheticTestFile(t, app.CredentialKeyFile)
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			workflowClient := &mocks.Client{}
			workflowClosed := 0
			closed := []*gorm.DB{}
			source, commercial, stores := &gorm.DB{}, &gorm.DB{}, &gorm.DB{}
			worker := &observationProcessWorker{}
			listening := make(chan string, 1)
			result := make(chan error, 1)
			go func() {
				result <- Run(ctx, c, logrus.New(), Dependencies{
					IdentityPreflight:   func(context.Context, IdentityConfig) error { return nil },
					OpenSourceAccount:   func(context.Context, DatabaseConfig) (*gorm.DB, error) { return source, nil },
					OpenCommercialOwner: func(context.Context, DatabaseConfig) (*gorm.DB, error) { return commercial, nil },
					OpenStoreCenter:     func(context.Context, DatabaseConfig) (*gorm.DB, error) { return stores, nil },
					DialStoreObservationsWorkflow: func(_ context.Context, address, namespace string) (client.Client, func() error, error) {
						require.Equal(t, "127.0.0.1:7233", address)
						require.Equal(t, "default", namespace)
						return workflowClient, func() error { workflowClosed++; return nil }, nil
					},
					CloseDatabase: func(db *gorm.DB) error { closed = append(closed, db); return nil },
					NewApplicationWithFeatures: func(_ context.Context, _ *gorm.DB, f ApplicationFeatures, _ *coreconfig.Config, _ *logrus.Logger) (*http.Server, error) {
						require.Same(t, stores, f.StoreCenterDB)
						require.Same(t, workflowClient, f.StoreObservationsWorkflow)
						require.Nil(t, f.SupplyAssetDB)
						require.Nil(t, f.SupplyWorkflow)
						require.False(t, f.StoreObservationsLifecycle.Ready())
						if stage == "construct-error" {
							return nil, errors.New("synthetic-construct-error")
						}
						worker.start = func() error {
							require.False(t, f.StoreObservationsLifecycle.Ready())
							if stage == "start-error" {
								return errors.New("synthetic-start-error")
							}
							return nil
						}
						f.StoreObservationsLifecycle.Worker = worker
						return &http.Server{Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
							if !f.StoreObservationsLifecycle.Ready() {
								http.Error(w, "unready", 503)
								return
							}
							w.WriteHeader(200)
						})}, nil
					},
					Listen: func(string, string) (net.Listener, error) {
						listener, err := net.Listen("tcp", "127.0.0.1:0")
						if err == nil {
							listening <- listener.Addr().String()
						}
						return listener, err
					},
				})
			}()
			if stage == "serve" {
				select {
				case address := <-listening:
					response, err := (&http.Client{Timeout: 3 * time.Second}).Get("http://" + address)
					require.NoError(t, err)
					require.Equal(t, 200, response.StatusCode)
					_ = response.Body.Close()
					cancel()
				case err := <-result:
					t.Fatalf("runtime failed before listening: %v", err)
				case <-ctx.Done():
					t.Fatal("runtime did not listen")
				}
			}
			err := <-result
			if stage == "serve" {
				require.NoError(t, err)
				require.Equal(t, 1, worker.stopped)
			} else {
				require.Error(t, err)
			}
			require.Equal(t, 1, workflowClosed)
			require.Equal(t, []*gorm.DB{stores, commercial, source}, closed)
		})
	}
}
