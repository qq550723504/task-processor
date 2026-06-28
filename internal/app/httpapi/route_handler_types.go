package httpapi

import (
	"github.com/gin-gonic/gin"

	"task-processor/internal/amazonlisting"
	"task-processor/internal/listingkit"
	listingkithttpapi "task-processor/internal/listingkit/httpapi"
	"task-processor/internal/productenrich"
	productimagehttpapi "task-processor/internal/productimage/httpapi"
	promptmgmtapi "task-processor/internal/promptmgmt/api"
	sdshttpapi "task-processor/internal/sds/httpapi"
	"task-processor/internal/sdslogin"
	"task-processor/internal/taskrpcapi"
)

type productRouteHandler = productenrich.ProductHandler
type imageRouteHandler = productimagehttpapi.RouteHandler
type amazonListingRouteHandler = amazonlisting.Handler
type listingKitRouteHandler = listingkithttpapi.RouteHandler
type studioSessionRouteHandler = listingkit.StudioSessionHandler
type taskRPCRouteHandler = taskrpcapi.Handler

type promptTemplateRouteHandler = promptmgmtapi.HTTPRouteHandler

type sdsCatalogRouteHandler = sdshttpapi.HTTPRouteHandler

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

type sdsLoginRouteHandler = sdslogin.HTTPRouteHandler
