package httpapi

import (
	"github.com/gin-gonic/gin"

	"task-processor/internal/amazonlisting"
	kernelmodule "task-processor/internal/kernel/module"
	"task-processor/internal/listingkit"
	listingkithttpapi "task-processor/internal/listingkit/httpapi"
	"task-processor/internal/productenrich"
	productimagehttpapi "task-processor/internal/productimage/httpapi"
	promptmgmtapi "task-processor/internal/promptmgmt/api"
	sdshttpapi "task-processor/internal/sds/httpapi"
	"task-processor/internal/sdslogin"
	"task-processor/internal/taskrpcapi"
)

type httpModuleHandlers struct {
	product          productenrich.ProductHandler
	image            productimagehttpapi.RouteHandler
	amazonListing    amazonlisting.Handler
	listingKit       listingkithttpapi.RouteHandler
	promptTemplate   promptmgmtapi.HTTPRouteHandler
	promptModule     kernelmodule.Module
	studioSession    listingkit.StudioSessionHandler
	sheinLoginModule kernelmodule.Module
	sheinLogin       sheinLoginRouteHandler
	sdsLoginModule   kernelmodule.Module
	sdsLogin         sdslogin.HTTPRouteHandler
	taskRPCModule    kernelmodule.Module
	taskRPC          taskrpcapi.Handler
	sdsCatalog       sdshttpapi.HTTPRouteHandler
	sdsModule        kernelmodule.Module
}

type sheinLoginRouteHandler interface {
	Health(c *gin.Context)
	ListAccounts(c *gin.Context)
	Login(c *gin.Context)
	Status(c *gin.Context)
	ListWarehouses(c *gin.Context)
	SubmitVerifyCode(c *gin.Context)
	CancelVerifyCodeWait(c *gin.Context)
	ClearCookie(c *gin.Context)
	GetLastFailure(c *gin.Context)
	ClearLastFailure(c *gin.Context)
}

func (c httpFeatureComposition) productHandler() productenrich.ProductHandler {
	if c.productModule == nil {
		return nil
	}
	return c.productModule.Handler
}

func (c httpFeatureComposition) imageHandler() productimagehttpapi.RouteHandler {
	if c.imageModule == nil {
		return nil
	}
	return c.imageModule.Handler
}

func (c httpFeatureComposition) amazonListingHandler() amazonlisting.Handler {
	if c.amazonListingModule == nil {
		return nil
	}
	return c.amazonListingModule.Handler
}

func (c httpFeatureComposition) listingKitHandler() listingkithttpapi.RouteHandler {
	if c.listingKitModule == nil {
		return nil
	}
	return c.listingKitModule.Handler
}

func (c httpFeatureComposition) studioSessionHandler() listingkit.StudioSessionHandler {
	if c.listingKitModule == nil {
		return nil
	}
	return c.listingKitModule.StudioSessionHandler
}
