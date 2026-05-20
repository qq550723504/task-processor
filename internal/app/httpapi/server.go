package httpapi

import (
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	amazonlistinghttpapi "task-processor/internal/amazonlisting/httpapi"
	listingkithttpapi "task-processor/internal/listingkit/httpapi"
	productenrichhttpapi "task-processor/internal/productenrich/httpapi"
)

func buildHTTPServer(port int, productHandler productRouteHandler, imageHandler imageRouteHandler, amazonListingHandler amazonListingRouteHandler, listingKitHandler listingKitRouteHandler, taskRPCHandler taskRPCRouteHandler, sdsCatalogHandlers ...sdsCatalogRouteHandler) *http.Server {
	server, _ := buildHTTPServerBundleWithStudio(port, productHandler, imageHandler, amazonListingHandler, listingKitHandler, nil, nil, nil, nil, taskRPCHandler, sdsCatalogHandlers...)
	return server
}

func buildHTTPServerWithStudio(port int, productHandler productRouteHandler, imageHandler imageRouteHandler, amazonListingHandler amazonListingRouteHandler, listingKitHandler listingKitRouteHandler, studioSessionHandler studioSessionRouteHandler, taskRPCHandler taskRPCRouteHandler, sdsCatalogHandlers ...sdsCatalogRouteHandler) *http.Server {
	server, _ := buildHTTPServerBundleWithStudio(port, productHandler, imageHandler, amazonListingHandler, listingKitHandler, nil, studioSessionHandler, nil, nil, taskRPCHandler, sdsCatalogHandlers...)
	return server
}

func buildHTTPServerBundleWithStudio(port int, productHandler productRouteHandler, imageHandler imageRouteHandler, amazonListingHandler amazonListingRouteHandler, listingKitHandler listingKitRouteHandler, promptTemplateHandler promptTemplateRouteHandler, studioSessionHandler studioSessionRouteHandler, sheinLoginHandler sheinLoginRouteHandler, sdsLoginHandler sdsLoginRouteHandler, taskRPCHandler taskRPCRouteHandler, sdsCatalogHandlers ...sdsCatalogRouteHandler) (*http.Server, []routeDescriptor) {
	router := gin.New()
	router.Use(gin.Recovery())
	routes := buildRouteDescriptorsWithShein(productHandler, imageHandler, amazonListingHandler, listingKitHandler, promptTemplateHandler, studioSessionHandler, sheinLoginHandler, sdsLoginHandler, taskRPCHandler, sdsCatalogHandlers...)
	mountRoutes(router, routes)
	return &http.Server{
		Addr:              fmt.Sprintf(":%d", port),
		Handler:           router,
		ReadHeaderTimeout: 5 * time.Second,
	}, routes
}

func RegisterRoutes(r *gin.Engine, productHandler productRouteHandler, imageHandler imageRouteHandler, amazonListingHandler amazonListingRouteHandler, listingKitHandler listingKitRouteHandler, taskRPCHandler taskRPCRouteHandler, sdsCatalogHandlers ...sdsCatalogRouteHandler) {
	mountRoutes(r, buildRouteDescriptorsWithShein(productHandler, imageHandler, amazonListingHandler, listingKitHandler, nil, nil, nil, nil, taskRPCHandler, sdsCatalogHandlers...))
}

func RegisterRoutesWithPrompt(r *gin.Engine, productHandler productRouteHandler, imageHandler imageRouteHandler, amazonListingHandler amazonListingRouteHandler, listingKitHandler listingKitRouteHandler, promptTemplateHandler promptTemplateRouteHandler, taskRPCHandler taskRPCRouteHandler, sdsCatalogHandlers ...sdsCatalogRouteHandler) {
	mountRoutes(r, buildRouteDescriptorsWithShein(productHandler, imageHandler, amazonListingHandler, listingKitHandler, promptTemplateHandler, nil, nil, nil, taskRPCHandler, sdsCatalogHandlers...))
}

func buildRouteDescriptors(productHandler productRouteHandler, imageHandler imageRouteHandler, amazonListingHandler amazonListingRouteHandler, listingKitHandler listingKitRouteHandler, studioSessionHandler studioSessionRouteHandler, taskRPCHandler taskRPCRouteHandler, sdsCatalogHandlers ...sdsCatalogRouteHandler) []routeDescriptor {
	return buildRouteDescriptorsWithShein(productHandler, imageHandler, amazonListingHandler, listingKitHandler, nil, studioSessionHandler, nil, nil, taskRPCHandler, sdsCatalogHandlers...)
}

func buildRouteDescriptorsWithShein(productHandler productRouteHandler, imageHandler imageRouteHandler, amazonListingHandler amazonListingRouteHandler, listingKitHandler listingKitRouteHandler, promptTemplateHandler promptTemplateRouteHandler, studioSessionHandler studioSessionRouteHandler, sheinLoginHandler sheinLoginRouteHandler, sdsLoginHandler sdsLoginRouteHandler, taskRPCHandler taskRPCRouteHandler, sdsCatalogHandlers ...sdsCatalogRouteHandler) []routeDescriptor {
	routes := buildCoreRouteDescriptors()
	routes = productenrichhttpapi.AppendProductRouteDescriptors(routes, productHandler, imageHandler)
	routes = amazonlistinghttpapi.AppendRouteDescriptors(routes, amazonListingHandler)
	routes = listingkithttpapi.AppendRouteDescriptors(routes, listingKitHandler)
	routes = listingkithttpapi.AppendPromptTemplateRouteDescriptors(routes, promptTemplateHandler)
	routes = listingkithttpapi.AppendStudioSessionRouteDescriptors(routes, studioSessionHandler)
	routes = appendSDSCatalogRouteDescriptors(routes, sdsCatalogHandlers...)
	routes = appendTaskRPCRouteDescriptors(routes, taskRPCHandler)
	routes = appendSheinLoginRouteDescriptors(routes, sheinLoginHandler)
	routes = appendSDSLoginRouteDescriptors(routes, sdsLoginHandler)
	return routes
}

func mountRoutes(r *gin.Engine, routes []routeDescriptor) {
	zitadelAuth := listingkithttpapi.NewZitadelAuthMiddlewareFromEnv()
	for _, route := range routes {
		if zitadelAuth != nil && listingkithttpapi.RouteRequiresZitadelAuth(route) {
			if roleAuth := listingkithttpapi.NewRouteRoleMiddleware(route); roleAuth != nil {
				r.Handle(route.Method, route.Path, zitadelAuth, roleAuth, route.Handler)
				continue
			}
			r.Handle(route.Method, route.Path, zitadelAuth, route.Handler)
			continue
		}
		r.Handle(route.Method, route.Path, route.Handler)
	}
}
