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
	"task-processor/internal/httproute"
	"task-processor/internal/infra/worker"
	kernelmodule "task-processor/internal/kernel/module"
	listingkithttpapi "task-processor/internal/listingkit/httpapi"
	a1688 "task-processor/internal/product/sourcehandoff/a1688"
	productsourcea1688httpapi "task-processor/internal/product/sourcehandoff/a1688/httpapi"
	productenrichhttpapi "task-processor/internal/productenrich/httpapi"
	productimagehttpapi "task-processor/internal/productimage/httpapi"
	promptmgmtapi "task-processor/internal/promptmgmt/api"
	sdshttpapi "task-processor/internal/sds/httpapi"
	"task-processor/internal/sdslogin"
	"task-processor/internal/sheinlogin"
	"task-processor/internal/taskrpcapi"
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

	err := sdshttpapi.NewHTTPModule(&stubSDSCatalogRouteHandler{}).Register(reg)
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

	err := taskrpcapi.NewHTTPModule(&stubTaskRPCHandler{}).Register(reg)
	require.NoError(t, err)

	require.Equal(t, []string{
		"GET /api/v1/management/tasks/health",
		"GET /api/v1/management/tasks/:task_id/status",
		"GET /api/v1/management/tasks/queue-stats",
	}, routeKeys(reg.Routes()))
}

func TestSheinLoginHTTPModuleRegistersOnlyConfiguredHandlers(t *testing.T) {
	t.Parallel()

	reg := kernelmodule.NewRegistry()

	err := sheinlogin.NewHTTPModule(&stubSheinLoginHandler{}).Register(reg)
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

	err := sdslogin.NewHTTPModule(&stubSDSLoginHandler{}).Register(reg)
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

	err := productenrichhttpapi.NewHTTPModule(&stubProductHandler{}, &stubImageHandler{}).Register(reg)
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

	err := amazonlistinghttpapi.NewHTTPModule(&stubAmazonListingHandler{}).Register(reg)
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

func TestProductSourcingHTTPModuleRegistersRoutes(t *testing.T) {
	t.Parallel()

	reg := kernelmodule.NewRegistry()

	err := productsourcea1688httpapi.NewHTTPModule(productsourcea1688httpapi.NewHandler(&stubProductSourcingTaskCommandService{})).Register(reg)
	require.NoError(t, err)

	require.Equal(t, []string{
		"POST /api/v1/product-sourcing/1688/listingkit/tasks",
	}, routeKeys(reg.Routes()))
}

func TestListingKitHTTPModuleRegistersRoutes(t *testing.T) {
	t.Parallel()

	reg := kernelmodule.NewRegistry()

	err := listingkithttpapi.NewHTTPModule(&stubListingKitHandler{}).Register(reg)
	require.NoError(t, err)

	keys := routeKeys(reg.Routes())
	require.Contains(t, keys, "POST /api/v1/listing-kits/generate")
	require.NotContains(t, keys, "GET /api/v1/listing-kits/studio/sessions/gallery")
}

func TestListingKitStudioHTTPModuleRegistersRoutes(t *testing.T) {
	t.Parallel()

	reg := kernelmodule.NewRegistry()

	err := listingkithttpapi.NewStudioHTTPModule(&stubStudioSessionHandler{}).Register(reg)
	require.NoError(t, err)

	keys := routeKeys(reg.Routes())
	require.NotContains(t, keys, "POST /api/v1/listing-kits/generate")
	require.Contains(t, keys, "GET /api/v1/listing-kits/studio/sessions/gallery")
}

func TestPromptTemplateHTTPModuleRegistersRoutes(t *testing.T) {
	t.Parallel()

	reg := kernelmodule.NewRegistry()

	err := promptmgmtapi.NewHTTPModule(&stubPromptTemplateHandler{}).Register(reg)
	require.NoError(t, err)

	require.Equal(t, []string{
		"GET /api/v1/listing-kits/prompts/catalog",
		"GET /api/v1/listing-kits/prompts/schema/:key",
		"GET /api/v1/listing-kits/prompts",
		"PUT /api/v1/listing-kits/prompts",
		"PATCH /api/v1/listing-kits/prompts/:key/status",
	}, routeKeys(reg.Routes()))
}

func TestHTTPModuleRegisterRejectsNilRegistrar(t *testing.T) {
	t.Parallel()

	err := (httpModule{name: "broken"}).Register(kernelmodule.NewRegistry())
	require.EqualError(t, err, "http module broken has no registrar")
}

func TestBuildHTTPServerBundleFromModulesSkipsDisabledModules(t *testing.T) {
	enabledRoute := httproute.Descriptor{
		Method: http.MethodGet,
		Path:   "/enabled",
		Module: "enabled",
		Handler: func(c *gin.Context) {
			c.Status(http.StatusOK)
		},
	}
	disabledRoute := httproute.Descriptor{
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
			register: func(reg *kernelmodule.Registry) error { reg.AddRoutes(enabledRoute); return nil },
		},
		httpModule{
			name: "disabled",
			enabled: func(*config.Config) bool {
				return false
			},
			register: func(reg *kernelmodule.Registry) error { reg.AddRoutes(disabledRoute); return nil },
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
				reg.AddRoutes(httproute.Descriptor{
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
				reg.AddRoutes(httproute.Descriptor{
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

	status := bundle.localTaskHealthProvider()()
	summary := status["summary"].(map[string]any)
	require.Equal(t, 2, summary["poolCount"])

	pools := status["pools"].(map[string]any)
	require.Contains(t, pools, "custom_pool")
	require.Contains(t, pools, "secondary_pool")
	require.NotContains(t, pools, "disabled_pool")
}

func routeKeys(routes []httproute.Descriptor) []string {
	keys := make([]string, 0, len(routes))
	for _, route := range routes {
		keys = append(keys, fmt.Sprintf("%s %s", route.Method, route.Path))
	}
	return keys
}

type stubProductSourcingTaskCommandService struct{}

func (stubProductSourcingTaskCommandService) CreateTask(context.Context, a1688.CreateTaskCommand) (*a1688.CreateTaskResult, error) {
	return &a1688.CreateTaskResult{}, nil
}
