package httpapi

import (
	"fmt"
	"net/http"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"

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

func TestOpsHTTPModuleRegistersOnlyConfiguredHandlers(t *testing.T) {
	t.Parallel()

	reg := kernelmodule.NewRegistry()

	err := newOpsHTTPModule(httpModuleHandlers{
		sdsCatalog: &stubSDSCatalogRouteHandler{},
		taskRPC:    &stubTaskRPCHandler{},
		sheinLogin: &stubSheinLoginHandler{},
		sdsLogin:   &stubSDSLoginHandler{},
	}).Register(reg)
	require.NoError(t, err)

	require.Equal(t, []string{
		"GET /api/v1/sds/products",
		"GET /api/v1/sds/products/:product_id",
		"GET /api/v1/sds/categories",
		"GET /api/v1/sds/shipment-areas",
		"GET /api/v1/management/tasks/health",
		"GET /api/v1/management/tasks/:task_id/status",
		"POST /api/v1/management/tasks/:task_id/retry",
		"POST /api/v1/management/tasks/:task_id/cancel",
		"GET /api/v1/management/tasks/queue-stats",
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
		listingKit:     &stubListingKitHandler{},
		promptTemplate: &stubPromptTemplateHandler{},
		studioSession:  &stubStudioSessionHandler{},
	}).Register(reg)
	require.NoError(t, err)

	keys := routeKeys(reg.Routes())
	require.Contains(t, keys, "POST /api/v1/listing-kits/generate")
	require.Contains(t, keys, "GET /api/v1/listing-kits/prompts/catalog")
	require.Contains(t, keys, "GET /api/v1/listing-kits/studio/sessions/gallery")
}

func TestHTTPModuleRegisterRejectsNilRegistrar(t *testing.T) {
	t.Parallel()

	err := (httpModule{name: "broken"}).Register(kernelmodule.NewRegistry())
	require.EqualError(t, err, "http module broken has no registrar")
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
