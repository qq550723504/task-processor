package httpapi

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"

	"task-processor/internal/core/config"
	kernelmodule "task-processor/internal/kernel/module"
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
	}).Register(reg)
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
	}).Register(reg)
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
	}).Register(reg)
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
	}).Register(reg)
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

func routeKeys(routes []routeDescriptor) []string {
	keys := make([]string, 0, len(routes))
	for _, route := range routes {
		keys = append(keys, fmt.Sprintf("%s %s", route.Method, route.Path))
	}
	return keys
}
