package httpapi

import (
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
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
