package httpapi

import (
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	listingkithttpapi "task-processor/internal/listingkit/httpapi"
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
	routes := buildRouteDescriptorsWithShein(productHandler, imageHandler, amazonListingHandler, listingKitHandler, promptTemplateHandler, studioSessionHandler, sheinLoginHandler, sdsLoginHandler, taskRPCHandler, sdsCatalogHandlers...)
	return buildHTTPServerFromRoutes(port, routes), routes
}

func buildHTTPServerFromRoutes(port int, routes []routeDescriptor) *http.Server {
	router := gin.New()
	router.Use(gin.Recovery())
	mountRoutes(router, routes)
	return &http.Server{
		Addr:              fmt.Sprintf(":%d", port),
		Handler:           router,
		ReadHeaderTimeout: 5 * time.Second,
	}
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
	routes, err := buildRegisteredRoutes(nil, httpModuleHandlers{
		product:        productHandler,
		image:          imageHandler,
		amazonListing:  amazonListingHandler,
		listingKit:     listingKitHandler,
		promptTemplate: promptTemplateHandler,
		studioSession:  studioSessionHandler,
		sheinLogin:     sheinLoginHandler,
		sdsLogin:       sdsLoginHandler,
		taskRPC:        taskRPCHandler,
		sdsCatalog:     singleSDSCatalogHandler(sdsCatalogHandlers...),
	})
	if err != nil {
		panic(err)
	}
	return routes
}

func singleSDSCatalogHandler(sdsCatalogHandlers ...sdsCatalogRouteHandler) sdsCatalogRouteHandler {
	switch len(sdsCatalogHandlers) {
	case 0:
		return nil
	case 1:
		return sdsCatalogHandlers[0]
	default:
		panic(fmt.Sprintf("expected at most 1 SDS catalog handler, got %d", len(sdsCatalogHandlers)))
	}
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
