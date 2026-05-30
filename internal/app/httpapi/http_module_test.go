package httpapi

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"

	amazonlistinghttpapi "task-processor/internal/amazonlisting/httpapi"
	"task-processor/internal/core/config"
	"task-processor/internal/infra/worker"
	kernelmodule "task-processor/internal/kernel/module"
	listingkithttpapi "task-processor/internal/listingkit/httpapi"
	productenrichhttpapi "task-processor/internal/productenrich/httpapi"
	productimagehttpapi "task-processor/internal/productimage/httpapi"
)

func TestCoreHTTPModuleRegistersHealthRoute(t *testing.T) {
	t.Parallel()

	reg := kernelmodule.NewRegistry()

	err := newCoreHTTPModule().Register(reg)
	require.NoError(t, err)

	routes := reg.Routes()
	require.Len(t, routes, 1)
	require.Equal(t, http.MethodGet, routes[0].Method)
	require.Equal(t, "/health", routes[0].Path)
	require.Equal(t, "system", routes[0].Module)
}

func TestSDSCatalogHTTPModuleRegistersOnlyConfiguredHandlers(t *testing.T) {
	t.Parallel()

	reg := kernelmodule.NewRegistry()

	err := newSDSCatalogHTTPModule(httpModuleHandlers{
		sdsCatalog: &stubSDSCatalogRouteHandler{},
	}).Register(reg)
	require.NoError(t, err)

	require.Equal(t, []string{
		"GET /api/v1/sds/products",
		"GET /api/v1/sds/products/:product_id",
		"GET /api/v1/sds/categories",
		"GET /api/v1/sds/shipment-areas",
	}, routeKeys(reg.Routes()))
}

func TestTaskRPCHTTPModuleRegistersOnlyConfiguredHandlers(t *testing.T) {
	t.Parallel()

	reg := kernelmodule.NewRegistry()

	err := newTaskRPCHTTPModule(httpModuleHandlers{
		taskRPC: &stubTaskRPCHandler{},
	}).Register(reg)
	require.NoError(t, err)

	require.Equal(t, []string{
		"GET /api/v1/management/tasks/health",
		"GET /api/v1/management/tasks/:task_id/status",
		"POST /api/v1/management/tasks/:task_id/retry",
		"POST /api/v1/management/tasks/:task_id/cancel",
		"GET /api/v1/management/tasks/queue-stats",
	}, routeKeys(reg.Routes()))
}

func TestSheinLoginHTTPModuleRegistersOnlyConfiguredHandlers(t *testing.T) {
	t.Parallel()

	reg := kernelmodule.NewRegistry()

	err := newSheinLoginHTTPModule(httpModuleHandlers{
		sheinLogin: &stubSheinLoginHandler{},
	}).Register(reg)
	require.NoError(t, err)

	require.Equal(t, []string{
		"GET /api/v1/shein-login/health",
		"GET /api/v1/shein-login/accounts",
		"POST /api/v1/shein-login/accounts/:store_id/login",
		"GET /api/v1/shein-login/accounts/:store_id/status",
		"GET /api/v1/shein-login/accounts/:store_id/warehouses",
		"POST /api/v1/shein-login/accounts/:store_id/verify-code",
		"DELETE /api/v1/shein-login/accounts/:store_id/verify-code-wait",
		"DELETE /api/v1/shein-login/accounts/:store_id/cookie",
		"GET /api/v1/shein-login/accounts/:store_id/last-failure",
		"DELETE /api/v1/shein-login/accounts/:store_id/last-failure",
	}, routeKeys(reg.Routes()))
}

func TestSDSLoginHTTPModuleRegistersOnlyConfiguredHandlers(t *testing.T) {
	t.Parallel()

	reg := kernelmodule.NewRegistry()

	err := newSDSLoginHTTPModule(httpModuleHandlers{
		sdsLogin: &stubSDSLoginHandler{},
	}).Register(reg)
	require.NoError(t, err)

	require.Equal(t, []string{
		"GET /api/v1/sds-login/health",
		"GET /api/v1/sds-login/status",
		"POST /api/v1/sds-login/login",
		"POST /api/v1/sds-login/manual-login",
		"GET /api/v1/sds-login/auth-state",
		"DELETE /api/v1/sds-login/state",
	}, routeKeys(reg.Routes()))
}

func TestProductHTTPModuleRegistersRoutes(t *testing.T) {
	t.Parallel()

	reg := kernelmodule.NewRegistry()

	err := newProductHTTPModule(httpModuleHandlers{
		product: &stubProductHandler{},
		image:   &stubImageHandler{},
	}, nil, nil).Register(reg)
	require.NoError(t, err)

	require.Equal(t, []string{
		"POST /api/v1/products/generate",
		"GET /api/v1/products/tasks/:task_id",
		"POST /api/v1/images/process",
		"GET /api/v1/images/tasks/:task_id",
		"POST /api/v1/images/tasks/:task_id/review",
	}, routeKeys(reg.Routes()))
}

func TestAmazonListingHTTPModuleRegistersRoutes(t *testing.T) {
	t.Parallel()

	reg := kernelmodule.NewRegistry()

	err := newAmazonListingHTTPModule(httpModuleHandlers{
		amazonListing: &stubAmazonListingHandler{},
	}, nil).Register(reg)
	require.NoError(t, err)

	require.Equal(t, []string{
		"POST /api/v1/amazon/listings/generate",
		"GET /api/v1/amazon/listings/tasks",
		"GET /api/v1/amazon/listings/tasks/:task_id",
		"GET /api/v1/amazon/listings/tasks/:task_id/workbench",
		"POST /api/v1/amazon/listings/tasks/:task_id/review",
		"POST /api/v1/amazon/listings/tasks/:task_id/submit",
	}, routeKeys(reg.Routes()))
}

func TestListingKitHTTPModuleRegistersRoutes(t *testing.T) {
	t.Parallel()

	reg := kernelmodule.NewRegistry()

	err := newListingKitHTTPModule(httpModuleHandlers{
		listingKit: &stubListingKitHandler{},
	}, nil).Register(reg)
	require.NoError(t, err)

	keys := routeKeys(reg.Routes())
	require.Contains(t, keys, "POST /api/v1/listing-kits/generate")
	require.NotContains(t, keys, "GET /api/v1/listing-kits/studio/sessions/gallery")
}

func TestListingKitStudioHTTPModuleRegistersRoutes(t *testing.T) {
	t.Parallel()

	reg := kernelmodule.NewRegistry()

	err := newListingKitStudioHTTPModule(httpModuleHandlers{
		studioSession: &stubStudioSessionHandler{},
	}, nil).Register(reg)
	require.NoError(t, err)

	keys := routeKeys(reg.Routes())
	require.NotContains(t, keys, "POST /api/v1/listing-kits/generate")
	require.Contains(t, keys, "GET /api/v1/listing-kits/studio/sessions/gallery")
}

func TestPromptTemplateHTTPModuleRegistersRoutes(t *testing.T) {
	t.Parallel()

	reg := kernelmodule.NewRegistry()

	err := newPromptTemplateHTTPModule(httpModuleHandlers{
		promptTemplate: &stubPromptTemplateHandler{},
	}).Register(reg)
	require.NoError(t, err)

	require.Equal(t, []string{
		"GET /api/v1/listing-kits/prompts/catalog",
		"GET /api/v1/listing-kits/prompts/schema/:key",
		"GET /api/v1/listing-kits/prompts",
		"PUT /api/v1/listing-kits/prompts",
		"PATCH /api/v1/listing-kits/prompts/:key/status",
	}, routeKeys(reg.Routes()))
}

func TestPromptTemplateHTTPModuleUsesPrebuiltModuleWhenProvided(t *testing.T) {
	t.Parallel()

	reg := kernelmodule.NewRegistry()

	prebuilt := httpModule{
		name: "prompt-prebuilt",
		register: func(reg *kernelmodule.Registry) error {
			reg.AddRoutes(routeDescriptor{
				Method: http.MethodGet,
				Path:   "/prompt-prebuilt",
				Module: "prompt-prebuilt",
				Handler: func(c *gin.Context) {
					c.Status(http.StatusNoContent)
				},
			})
			return nil
		},
	}

	err := newPromptTemplateHTTPModule(httpModuleHandlers{
		promptTemplate: &stubPromptTemplateHandler{},
		promptModule:   prebuilt,
	}).Register(reg)
	require.NoError(t, err)
	require.Equal(t, []string{"GET /prompt-prebuilt"}, routeKeys(reg.Routes()))
}

func TestSDSCatalogHTTPModuleUsesPrebuiltModuleWhenProvided(t *testing.T) {
	t.Parallel()

	reg := kernelmodule.NewRegistry()

	prebuilt := httpModule{
		name: "sds-prebuilt",
		register: func(reg *kernelmodule.Registry) error {
			reg.AddRoutes(routeDescriptor{
				Method: http.MethodGet,
				Path:   "/sds-prebuilt",
				Module: "sds-prebuilt",
				Handler: func(c *gin.Context) {
					c.Status(http.StatusNoContent)
				},
			})
			return nil
		},
	}

	err := newSDSCatalogHTTPModule(httpModuleHandlers{
		sdsCatalog: &stubSDSCatalogRouteHandler{},
		sdsModule:  prebuilt,
	}).Register(reg)
	require.NoError(t, err)
	require.Equal(t, []string{"GET /sds-prebuilt"}, routeKeys(reg.Routes()))
}

func TestTaskRPCHTTPModuleUsesPrebuiltModuleWhenProvided(t *testing.T) {
	t.Parallel()

	reg := kernelmodule.NewRegistry()

	prebuilt := httpModule{
		name: "taskrpc-prebuilt",
		register: func(reg *kernelmodule.Registry) error {
			reg.AddRoutes(routeDescriptor{
				Method: http.MethodGet,
				Path:   "/taskrpc-prebuilt",
				Module: "taskrpc-prebuilt",
				Handler: func(c *gin.Context) {
					c.Status(http.StatusNoContent)
				},
			})
			return nil
		},
	}

	err := newTaskRPCHTTPModule(httpModuleHandlers{
		taskRPC:       &stubTaskRPCHandler{},
		taskRPCModule: prebuilt,
	}).Register(reg)
	require.NoError(t, err)
	require.Equal(t, []string{"GET /taskrpc-prebuilt"}, routeKeys(reg.Routes()))
}

func TestSheinLoginHTTPModuleUsesPrebuiltModuleWhenProvided(t *testing.T) {
	t.Parallel()

	reg := kernelmodule.NewRegistry()

	prebuilt := httpModule{
		name: "shein-prebuilt",
		register: func(reg *kernelmodule.Registry) error {
			reg.AddRoutes(routeDescriptor{
				Method: http.MethodGet,
				Path:   "/shein-prebuilt",
				Module: "shein-prebuilt",
				Handler: func(c *gin.Context) {
					c.Status(http.StatusNoContent)
				},
			})
			return nil
		},
	}

	err := newSheinLoginHTTPModule(httpModuleHandlers{
		sheinLogin:       &stubSheinLoginHandler{},
		sheinLoginModule: prebuilt,
	}).Register(reg)
	require.NoError(t, err)
	require.Equal(t, []string{"GET /shein-prebuilt"}, routeKeys(reg.Routes()))
}

func TestSDSLoginHTTPModuleUsesPrebuiltModuleWhenProvided(t *testing.T) {
	t.Parallel()

	reg := kernelmodule.NewRegistry()

	prebuilt := httpModule{
		name: "sdslogin-prebuilt",
		register: func(reg *kernelmodule.Registry) error {
			reg.AddRoutes(routeDescriptor{
				Method: http.MethodGet,
				Path:   "/sdslogin-prebuilt",
				Module: "sdslogin-prebuilt",
				Handler: func(c *gin.Context) {
					c.Status(http.StatusNoContent)
				},
			})
			return nil
		},
	}

	err := newSDSLoginHTTPModule(httpModuleHandlers{
		sdsLogin:       &stubSDSLoginHandler{},
		sdsLoginModule: prebuilt,
	}).Register(reg)
	require.NoError(t, err)
	require.Equal(t, []string{"GET /sdslogin-prebuilt"}, routeKeys(reg.Routes()))
}

func TestHTTPModuleRegisterRejectsNilRegistrar(t *testing.T) {
	t.Parallel()

	err := (httpModule{name: "broken"}).Register(kernelmodule.NewRegistry())
	require.EqualError(t, err, "http module broken has no registrar")
}

func TestBuildHTTPServerBundleFromModulesSkipsDisabledModules(t *testing.T) {
	enabledRoute := routeDescriptor{
		Method: http.MethodGet,
		Path:   "/enabled",
		Module: "enabled",
		Handler: func(c *gin.Context) {
			c.Status(http.StatusOK)
		},
	}
	disabledRoute := routeDescriptor{
		Method: http.MethodGet,
		Path:   "/disabled",
		Module: "disabled",
		Handler: func(c *gin.Context) {
			c.Status(http.StatusTeapot)
		},
	}

	server, routes, err := buildHTTPServerBundleFromModules(18080, &config.Config{}, []kernelmodule.Module{
		httpModule{
			name: "enabled",
			register: func(reg *kernelmodule.Registry) error {
				reg.AddRoutes(enabledRoute)
				return nil
			},
		},
		httpModule{
			name: "disabled",
			enabled: func(*config.Config) bool {
				return false
			},
			register: func(reg *kernelmodule.Registry) error {
				reg.AddRoutes(disabledRoute)
				return nil
			},
		},
	})
	require.NoError(t, err)
	require.Equal(t, []string{"GET /enabled"}, routeKeys(routes))

	router, ok := server.Handler.(*gin.Engine)
	require.True(t, ok)

	req := httptest.NewRequest(http.MethodGet, "/enabled", nil)
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)
	require.Equal(t, http.StatusOK, resp.Code)

	req = httptest.NewRequest(http.MethodGet, "/disabled", nil)
	resp = httptest.NewRecorder()
	router.ServeHTTP(resp, req)
	require.Equal(t, http.StatusNotFound, resp.Code)
}

func TestBuildHTTPServerBundleFromModulesSkipsNilModules(t *testing.T) {
	t.Parallel()

	server, routes, err := buildHTTPServerBundleFromModules(18080, &config.Config{}, []kernelmodule.Module{
		nil,
		httpModule{
			name: "enabled",
			register: func(reg *kernelmodule.Registry) error {
				reg.AddRoutes(routeDescriptor{
					Method: http.MethodGet,
					Path:   "/enabled",
					Module: "enabled",
					Handler: func(c *gin.Context) {
						c.Status(http.StatusOK)
					},
				})
				return nil
			},
		},
	})
	require.NoError(t, err)
	require.Equal(t, []string{"GET /enabled"}, routeKeys(routes))

	router, ok := server.Handler.(*gin.Engine)
	require.True(t, ok)

	req := httptest.NewRequest(http.MethodGet, "/enabled", nil)
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)
	require.Equal(t, http.StatusOK, resp.Code)
}

func TestBuildRuntimeBundleFromModulesCollectsRoutesAndWorkerPools(t *testing.T) {
	t.Parallel()

	bundle, err := buildRuntimeBundleFromModules(&config.Config{}, []kernelmodule.Module{
		httpModule{
			name: "product",
			register: func(reg *kernelmodule.Registry) error {
				reg.AddRoutes(routeDescriptor{
					Method: http.MethodGet,
					Path:   "/health",
					Module: "product",
					Handler: func(c *gin.Context) {
						c.Status(http.StatusOK)
					},
				})
				return reg.AddWorkerPool("product_enrich", stubWorkerPool{})
			},
		},
	})
	require.NoError(t, err)
	require.Equal(t, []string{"GET /health"}, routeKeys(bundle.routes))
	require.Len(t, bundle.workerPools, 1)
	require.Equal(t, "product_enrich", bundle.workerPools[0].Name)
}

func TestRuntimeBundleBuildsLocalTaskHealthProviderFromRegisteredPools(t *testing.T) {
	t.Parallel()

	metrics := worker.NewMetrics()
	metrics.RecordSubmit()
	metrics.RecordProcessSuccess(1)

	bundle, err := buildRuntimeBundleFromModules(&config.Config{}, []kernelmodule.Module{
		httpModule{
			name: "custom-runtime",
			register: func(reg *kernelmodule.Registry) error {
				return reg.AddWorkerPool("custom_pool", stubWorkerPool{
					stats: worker.QueueStats{
						QueueSize:      1,
						BufferSize:     4,
						AvailableSlots: 3,
					},
					metrics: metrics,
				})
			},
		},
		httpModule{
			name: "secondary-runtime",
			register: func(reg *kernelmodule.Registry) error {
				return reg.AddWorkerPool("secondary_pool", stubWorkerPool{
					stats: worker.QueueStats{
						QueueSize:      2,
						BufferSize:     5,
						AvailableSlots: 3,
					},
				})
			},
		},
		httpModule{
			name: "disabled-runtime",
			enabled: func(*config.Config) bool {
				return false
			},
			register: func(reg *kernelmodule.Registry) error {
				return reg.AddWorkerPool("disabled_pool", stubWorkerPool{
					stats: worker.QueueStats{
						QueueSize:      99,
						BufferSize:     99,
						AvailableSlots: 0,
					},
				})
			},
		},
	})
	require.NoError(t, err)

	status := bundle.localTaskHealthProvider()
	require.NotNil(t, status)

	snapshot := status()
	require.Equal(t, 2, snapshot["summary"].(map[string]any)["poolCount"])
	require.Equal(t, 3, snapshot["summary"].(map[string]any)["totalQueueSize"])

	pools := snapshot["pools"].(map[string]any)
	require.Contains(t, pools, "custom_pool")
	require.Contains(t, pools, "secondary_pool")
	require.NotContains(t, pools, "disabled_pool")
}

func TestHTTPFeatureCompositionBuildRuntimeBundleUsesRegisteredModules(t *testing.T) {
	t.Parallel()

	composition := httpFeatureComposition{
		productModule: &productenrichhttpapi.Module{
			Handler: &stubProductHandler{},
			Pool: stubWorkerPool{
				stats: worker.QueueStats{
					QueueSize:      1,
					BufferSize:     4,
					AvailableSlots: 3,
				},
			},
		},
		imageModule: &productimagehttpapi.Module{
			Handler: &stubImageHandler{},
			Pool: stubWorkerPool{
				stats: worker.QueueStats{
					QueueSize:      2,
					BufferSize:     5,
					AvailableSlots: 3,
				},
			},
		},
		amazonListingModule: &amazonlistinghttpapi.Module{},
		listingKitModule: &listingkithttpapi.Module{
			Handler: &stubListingKitHandler{},
			Pool: stubWorkerPool{
				stats: worker.QueueStats{
					QueueSize:      3,
					BufferSize:     6,
					AvailableSlots: 3,
				},
			},
		},
	}

	bundle, err := composition.buildRuntimeBundle(&config.Config{})
	require.NoError(t, err)
	require.Len(t, bundle.workerPools, 3)
	require.Contains(t, routeKeys(bundle.routes), "POST /api/v1/products/generate")
	require.Contains(t, routeKeys(bundle.routes), "POST /api/v1/images/process")
	require.Contains(t, routeKeys(bundle.routes), "POST /api/v1/listing-kits/generate")
}

func TestHTTPFeatureCompositionBuildServerBundleUsesRouteModules(t *testing.T) {
	t.Parallel()

	composition := httpFeatureComposition{
		productModule: &productenrichhttpapi.Module{
			Handler: &stubProductHandler{},
		},
		imageModule: &productimagehttpapi.Module{
			Handler: &stubImageHandler{},
		},
	}

	server, routes, err := composition.buildServerBundle(18080, &config.Config{})
	require.NoError(t, err)
	require.Contains(t, routeKeys(routes), "GET /health")
	require.Contains(t, routeKeys(routes), "POST /api/v1/products/generate")
	require.Contains(t, routeKeys(routes), "POST /api/v1/images/process")

	router, ok := server.Handler.(*gin.Engine)
	require.True(t, ok)

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)
	require.Equal(t, http.StatusOK, resp.Code)
}

type stubWorkerPool struct {
	stats   worker.QueueStats
	metrics *worker.Metrics
}

func (p stubWorkerPool) Start(_ context.Context)          {}
func (p stubWorkerPool) Stop(_ context.Context)           {}
func (p stubWorkerPool) Submit(worker.WorkerJob) error    { return nil }
func (p stubWorkerPool) AvailableSlots() int              { return p.stats.AvailableSlots }
func (p stubWorkerPool) GetQueueStats() worker.QueueStats { return p.stats }
func (p stubWorkerPool) SetJobHandler(worker.JobHandler)  {}
func (p stubWorkerPool) GetMetrics() *worker.Metrics      { return p.metrics }

type stubSDSCatalogRouteHandler struct{}

func (stubSDSCatalogRouteHandler) ListSDSProducts(*gin.Context)      {}
func (stubSDSCatalogRouteHandler) GetSDSProduct(*gin.Context)        {}
func (stubSDSCatalogRouteHandler) ListSDSCategories(*gin.Context)    {}
func (stubSDSCatalogRouteHandler) ListSDSShipmentAreas(*gin.Context) {}

type stubStudioSessionHandler struct{}

func (stubStudioSessionHandler) ListStudioSessionGallery(*gin.Context)    {}
func (stubStudioSessionHandler) ListStudioBatches(*gin.Context)           {}
func (stubStudioSessionHandler) GetStudioBatch(*gin.Context)              {}
func (stubStudioSessionHandler) UpsertStudioBatch(*gin.Context)           {}
func (stubStudioSessionHandler) DeleteStudioBatch(*gin.Context)           {}
func (stubStudioSessionHandler) EnsureStudioSession(*gin.Context)         {}
func (stubStudioSessionHandler) GetStudioSession(*gin.Context)            {}
func (stubStudioSessionHandler) UpdateStudioSession(*gin.Context)         {}
func (stubStudioSessionHandler) ReplaceStudioSessionDesigns(*gin.Context) {}
func (stubStudioSessionHandler) AppendStudioSessionDesigns(*gin.Context)  {}

func routeKeys(routes []routeDescriptor) []string {
	keys := make([]string, 0, len(routes))
	for _, route := range routes {
		keys = append(keys, fmt.Sprintf("%s %s", route.Method, route.Path))
	}
	return keys
}
