package httpapi

import (
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

func buildHTTPServer(port int, productHandler productRouteHandler, imageHandler imageRouteHandler, amazonListingHandler amazonListingRouteHandler, listingKitHandler listingKitRouteHandler, taskRPCHandler taskRPCRouteHandler, sdsCatalogHandlers ...sdsCatalogRouteHandler) *http.Server {
	server, _ := buildHTTPServerBundleWithStudio(port, productHandler, imageHandler, amazonListingHandler, listingKitHandler, nil, nil, nil, taskRPCHandler, sdsCatalogHandlers...)
	return server
}

func buildHTTPServerWithStudio(port int, productHandler productRouteHandler, imageHandler imageRouteHandler, amazonListingHandler amazonListingRouteHandler, listingKitHandler listingKitRouteHandler, studioSessionHandler studioSessionRouteHandler, taskRPCHandler taskRPCRouteHandler, sdsCatalogHandlers ...sdsCatalogRouteHandler) *http.Server {
	server, _ := buildHTTPServerBundleWithStudio(port, productHandler, imageHandler, amazonListingHandler, listingKitHandler, studioSessionHandler, nil, nil, taskRPCHandler, sdsCatalogHandlers...)
	return server
}

func buildHTTPServerBundleWithStudio(port int, productHandler productRouteHandler, imageHandler imageRouteHandler, amazonListingHandler amazonListingRouteHandler, listingKitHandler listingKitRouteHandler, studioSessionHandler studioSessionRouteHandler, sheinLoginHandler sheinLoginRouteHandler, sdsLoginHandler sdsLoginRouteHandler, taskRPCHandler taskRPCRouteHandler, sdsCatalogHandlers ...sdsCatalogRouteHandler) (*http.Server, []routeDescriptor) {
	router := gin.New()
	router.Use(gin.Recovery())
	routes := buildRouteDescriptorsWithShein(productHandler, imageHandler, amazonListingHandler, listingKitHandler, studioSessionHandler, sheinLoginHandler, sdsLoginHandler, taskRPCHandler, sdsCatalogHandlers...)
	mountRoutes(router, routes)
	return &http.Server{
		Addr:              fmt.Sprintf(":%d", port),
		Handler:           router,
		ReadHeaderTimeout: 5 * time.Second,
	}, routes
}

func RegisterRoutes(r *gin.Engine, productHandler productRouteHandler, imageHandler imageRouteHandler, amazonListingHandler amazonListingRouteHandler, listingKitHandler listingKitRouteHandler, taskRPCHandler taskRPCRouteHandler, sdsCatalogHandlers ...sdsCatalogRouteHandler) {
	mountRoutes(r, buildRouteDescriptorsWithShein(productHandler, imageHandler, amazonListingHandler, listingKitHandler, nil, nil, nil, taskRPCHandler, sdsCatalogHandlers...))
}

func buildRouteDescriptors(productHandler productRouteHandler, imageHandler imageRouteHandler, amazonListingHandler amazonListingRouteHandler, listingKitHandler listingKitRouteHandler, studioSessionHandler studioSessionRouteHandler, taskRPCHandler taskRPCRouteHandler, sdsCatalogHandlers ...sdsCatalogRouteHandler) []routeDescriptor {
	return buildRouteDescriptorsWithShein(productHandler, imageHandler, amazonListingHandler, listingKitHandler, studioSessionHandler, nil, nil, taskRPCHandler, sdsCatalogHandlers...)
}

func buildRouteDescriptorsWithShein(productHandler productRouteHandler, imageHandler imageRouteHandler, amazonListingHandler amazonListingRouteHandler, listingKitHandler listingKitRouteHandler, studioSessionHandler studioSessionRouteHandler, sheinLoginHandler sheinLoginRouteHandler, sdsLoginHandler sdsLoginRouteHandler, taskRPCHandler taskRPCRouteHandler, sdsCatalogHandlers ...sdsCatalogRouteHandler) []routeDescriptor {
	routes := []routeDescriptor{
		{Method: http.MethodGet, Path: "/health", Module: "system", Handler: func(c *gin.Context) {
			c.JSON(http.StatusOK, gin.H{"status": "ok"})
		}},
	}

	if productHandler != nil {
		routes = append(routes,
			routeDescriptor{Method: http.MethodPost, Path: "/api/v1/products/generate", Module: "products", Handler: productHandler.GenerateProduct},
			routeDescriptor{Method: http.MethodGet, Path: "/api/v1/products/tasks/:task_id", Module: "products", Handler: productHandler.GetTaskResult},
		)
	}

	if imageHandler != nil {
		routes = append(routes,
			routeDescriptor{Method: http.MethodPost, Path: "/api/v1/images/process", Module: "images", Handler: imageHandler.ProcessImages},
			routeDescriptor{Method: http.MethodGet, Path: "/api/v1/images/tasks/:task_id", Module: "images", Handler: imageHandler.GetTaskResult},
			routeDescriptor{Method: http.MethodPost, Path: "/api/v1/images/tasks/:task_id/review", Module: "images", Handler: imageHandler.ReviewTask},
		)
	}

	if amazonListingHandler != nil {
		routes = append(routes,
			routeDescriptor{Method: http.MethodPost, Path: "/api/v1/amazon/listings/generate", Module: "amazon-listing", Handler: amazonListingHandler.GenerateListing},
			routeDescriptor{Method: http.MethodGet, Path: "/api/v1/amazon/listings/tasks", Module: "amazon-listing", Handler: amazonListingHandler.ListTaskQueue},
			routeDescriptor{Method: http.MethodGet, Path: "/api/v1/amazon/listings/tasks/:task_id", Module: "amazon-listing", Handler: amazonListingHandler.GetTaskResult},
			routeDescriptor{Method: http.MethodGet, Path: "/api/v1/amazon/listings/tasks/:task_id/workbench", Module: "amazon-listing", Handler: amazonListingHandler.GetTaskWorkbench},
			routeDescriptor{Method: http.MethodPost, Path: "/api/v1/amazon/listings/tasks/:task_id/review", Module: "amazon-listing", Handler: amazonListingHandler.ReviewTask},
			routeDescriptor{Method: http.MethodPost, Path: "/api/v1/amazon/listings/tasks/:task_id/submit", Module: "amazon-listing", Handler: amazonListingHandler.SubmitTask},
		)
	}

	if listingKitHandler != nil {
		routes = append(routes,
			routeDescriptor{Method: http.MethodPost, Path: "/api/v1/listing-kits/generate", Module: "listing-kit", Handler: listingKitHandler.GenerateListingKit},
			routeDescriptor{Method: http.MethodGet, Path: "/api/v1/listing-kits/settings/shein", Module: "listing-kit", Handler: listingKitHandler.GetSheinSettings},
			routeDescriptor{Method: http.MethodPut, Path: "/api/v1/listing-kits/settings/shein", Module: "listing-kit", Handler: listingKitHandler.UpdateSheinSettings},
			routeDescriptor{Method: http.MethodGet, Path: "/api/v1/listing-kits/settings/ai", Module: "listing-kit", Handler: listingKitHandler.GetAIClientSettings},
			routeDescriptor{Method: http.MethodPut, Path: "/api/v1/listing-kits/settings/ai", Module: "listing-kit", Handler: listingKitHandler.UpdateAIClientSettings},
			routeDescriptor{Method: http.MethodPost, Path: "/api/v1/listing-kits/studio/designs", Module: "listing-kit", Handler: listingKitHandler.GenerateStudioDesigns},
			routeDescriptor{Method: http.MethodPost, Path: "/api/v1/listing-kits/studio/product-images", Module: "listing-kit", Handler: listingKitHandler.GenerateStudioProductImages},
			routeDescriptor{Method: http.MethodPost, Path: "/api/v1/listing-kits/studio/async-jobs", Module: "listing-kit", Handler: listingKitHandler.StartStudioAsyncJob},
			routeDescriptor{Method: http.MethodGet, Path: "/api/v1/listing-kits/studio/async-jobs/:job_id", Module: "listing-kit", Handler: listingKitHandler.GetStudioAsyncJob},
			routeDescriptor{Method: http.MethodPost, Path: "/api/v1/listing-kits/tasks/:task_id/shein-images/regenerate", Module: "listing-kit", Handler: listingKitHandler.RegenerateSheinDataImage},
			routeDescriptor{Method: http.MethodPost, Path: "/api/v1/listing-kits/uploads/images", Module: "listing-kit", Handler: listingKitHandler.UploadListingKitImages},
			routeDescriptor{Method: http.MethodGet, Path: "/api/v1/listing-kits/uploads/files/*key", Module: "listing-kit", Handler: listingKitHandler.GetUploadedListingKitImage},
			routeDescriptor{Method: http.MethodGet, Path: "/api/v1/listing-kits/tasks", Module: "listing-kit", Handler: listingKitHandler.ListTasks},
			routeDescriptor{Method: http.MethodGet, Path: "/api/v1/listing-kits/tasks/:task_id", Module: "listing-kit", Handler: listingKitHandler.GetTaskResult},
			routeDescriptor{Method: http.MethodGet, Path: "/api/v1/listing-kits/tasks/:task_id/preview", Module: "listing-kit", Handler: listingKitHandler.GetTaskPreview},
			routeDescriptor{Method: http.MethodGet, Path: "/api/v1/listing-kits/tasks/:task_id/generation-tasks", Module: "listing-kit", Handler: listingKitHandler.GetTaskGenerationTasks},
			routeDescriptor{Method: http.MethodGet, Path: "/api/v1/listing-kits/tasks/:task_id/generation-queue", Module: "listing-kit", Handler: listingKitHandler.GetTaskGenerationQueue},
			routeDescriptor{Method: http.MethodGet, Path: "/api/v1/listing-kits/tasks/:task_id/generation-review-session", Module: "listing-kit", Handler: listingKitHandler.GetTaskGenerationReviewSession},
			routeDescriptor{Method: http.MethodGet, Path: "/api/v1/listing-kits/tasks/:task_id/generation-review-preview", Module: "listing-kit", Handler: listingKitHandler.GetTaskGenerationReviewPreview},
			routeDescriptor{Method: http.MethodPost, Path: "/api/v1/listing-kits/tasks/:task_id/generation-navigation/dispatch", Module: "listing-kit", Handler: listingKitHandler.DispatchTaskGenerationNavigation},
			routeDescriptor{Method: http.MethodPost, Path: "/api/v1/listing-kits/tasks/:task_id/generation-tasks/retry", Module: "listing-kit", Handler: listingKitHandler.RetryTaskGenerationTasks},
			routeDescriptor{Method: http.MethodPost, Path: "/api/v1/listing-kits/tasks/:task_id/generation-actions/execute", Module: "listing-kit", Handler: listingKitHandler.ExecuteTaskGenerationAction},
			routeDescriptor{Method: http.MethodGet, Path: "/api/v1/listing-kits/tasks/:task_id/revision-history", Module: "listing-kit", Handler: listingKitHandler.GetTaskRevisionHistory},
			routeDescriptor{Method: http.MethodGet, Path: "/api/v1/listing-kits/tasks/:task_id/revision-history/:revision_id", Module: "listing-kit", Handler: listingKitHandler.GetTaskRevisionHistoryDetail},
			routeDescriptor{Method: http.MethodGet, Path: "/api/v1/listing-kits/tasks/:task_id/export", Module: "listing-kit", Handler: listingKitHandler.GetTaskExport},
			routeDescriptor{Method: http.MethodPost, Path: "/api/v1/listing-kits/tasks/:task_id/revision", Module: "listing-kit", Handler: listingKitHandler.ApplyTaskRevision},
			routeDescriptor{Method: http.MethodPost, Path: "/api/v1/listing-kits/tasks/:task_id/revision/validate", Module: "listing-kit", Handler: listingKitHandler.ValidateTaskRevision},
			routeDescriptor{Method: http.MethodPost, Path: "/api/v1/listing-kits/tasks/:task_id/shein/price-preview", Module: "listing-kit", Handler: listingKitHandler.PreviewSheinPrice},
			routeDescriptor{Method: http.MethodGet, Path: "/api/v1/listing-kits/tasks/:task_id/shein/categories", Module: "listing-kit", Handler: listingKitHandler.SearchSheinCategories},
			routeDescriptor{Method: http.MethodPatch, Path: "/api/v1/listing-kits/tasks/:task_id/shein/final-draft", Module: "listing-kit", Handler: listingKitHandler.UpdateSheinFinalDraft},
			routeDescriptor{Method: http.MethodGet, Path: "/api/v1/listing-kits/tasks/:task_id/submission-events", Module: "listing-kit", Handler: listingKitHandler.GetSubmissionEvents},
			routeDescriptor{Method: http.MethodPost, Path: "/api/v1/listing-kits/tasks/:task_id/submit", Module: "listing-kit", Handler: listingKitHandler.SubmitTask},
			routeDescriptor{Method: http.MethodPost, Path: "/api/v1/listing-kits/tasks/:task_id/submission-status/refresh", Module: "listing-kit", Handler: listingKitHandler.RefreshSubmissionStatus},
			routeDescriptor{Method: http.MethodDelete, Path: "/api/v1/listing-kits/tasks/:task_id/shein-resolution-cache", Module: "listing-kit", Handler: listingKitHandler.ClearSheinResolutionCache},
		)
	}

	if studioSessionHandler != nil {
		routes = append(routes,
			routeDescriptor{Method: http.MethodGet, Path: "/api/v1/listing-kits/studio/sessions/gallery", Module: "listing-kit-studio", Handler: studioSessionHandler.ListStudioSessionGallery},
			routeDescriptor{Method: http.MethodPost, Path: "/api/v1/listing-kits/studio/sessions", Module: "listing-kit-studio", Handler: studioSessionHandler.EnsureStudioSession},
			routeDescriptor{Method: http.MethodGet, Path: "/api/v1/listing-kits/studio/sessions/:session_id", Module: "listing-kit-studio", Handler: studioSessionHandler.GetStudioSession},
			routeDescriptor{Method: http.MethodPatch, Path: "/api/v1/listing-kits/studio/sessions/:session_id", Module: "listing-kit-studio", Handler: studioSessionHandler.UpdateStudioSession},
			routeDescriptor{Method: http.MethodPost, Path: "/api/v1/listing-kits/studio/sessions/:session_id/designs", Module: "listing-kit-studio", Handler: studioSessionHandler.ReplaceStudioSessionDesigns},
		)
	}

	var sdsCatalogHandler sdsCatalogRouteHandler
	if len(sdsCatalogHandlers) > 0 {
		sdsCatalogHandler = sdsCatalogHandlers[0]
	}
	if sdsCatalogHandler != nil {
		routes = append(routes,
			routeDescriptor{Method: http.MethodGet, Path: "/api/v1/sds/products", Module: "sds", Handler: sdsCatalogHandler.ListSDSProducts},
			routeDescriptor{Method: http.MethodGet, Path: "/api/v1/sds/products/:product_id", Module: "sds", Handler: sdsCatalogHandler.GetSDSProduct},
			routeDescriptor{Method: http.MethodGet, Path: "/api/v1/sds/categories", Module: "sds", Handler: sdsCatalogHandler.ListSDSCategories},
			routeDescriptor{Method: http.MethodGet, Path: "/api/v1/sds/shipment-areas", Module: "sds", Handler: sdsCatalogHandler.ListSDSShipmentAreas},
		)
	}

	if taskRPCHandler != nil {
		routes = append(routes,
			routeDescriptor{Method: http.MethodGet, Path: "/api/v1/management/tasks/health", Module: "management", Handler: taskRPCHandler.GetHealth},
			routeDescriptor{Method: http.MethodGet, Path: "/api/v1/management/tasks/:task_id/status", Module: "management", Handler: taskRPCHandler.GetTaskStatus},
			routeDescriptor{Method: http.MethodPost, Path: "/api/v1/management/tasks/:task_id/retry", Module: "management", Handler: taskRPCHandler.RetryTask},
			routeDescriptor{Method: http.MethodPost, Path: "/api/v1/management/tasks/:task_id/cancel", Module: "management", Handler: taskRPCHandler.CancelTask},
			routeDescriptor{Method: http.MethodGet, Path: "/api/v1/management/tasks/queue-stats", Module: "management", Handler: taskRPCHandler.GetQueueStats},
		)
	}

	if sheinLoginHandler != nil {
		routes = append(routes,
			routeDescriptor{Method: http.MethodGet, Path: "/api/v1/shein-login/health", Module: "shein-login", Handler: sheinLoginHandler.Health},
			routeDescriptor{Method: http.MethodGet, Path: "/api/v1/shein-login/accounts", Module: "shein-login", Handler: sheinLoginHandler.ListAccounts},
			routeDescriptor{Method: http.MethodPost, Path: "/api/v1/shein-login/accounts/:store_id/login", Module: "shein-login", Handler: sheinLoginHandler.Login},
			routeDescriptor{Method: http.MethodGet, Path: "/api/v1/shein-login/accounts/:store_id/status", Module: "shein-login", Handler: sheinLoginHandler.Status},
			routeDescriptor{Method: http.MethodPost, Path: "/api/v1/shein-login/accounts/:store_id/verify-code", Module: "shein-login", Handler: sheinLoginHandler.SubmitVerifyCode},
			routeDescriptor{Method: http.MethodDelete, Path: "/api/v1/shein-login/accounts/:store_id/verify-code-wait", Module: "shein-login", Handler: sheinLoginHandler.CancelVerifyCodeWait},
			routeDescriptor{Method: http.MethodDelete, Path: "/api/v1/shein-login/accounts/:store_id/cookie", Module: "shein-login", Handler: sheinLoginHandler.ClearCookie},
			routeDescriptor{Method: http.MethodGet, Path: "/api/v1/shein-login/accounts/:store_id/last-failure", Module: "shein-login", Handler: sheinLoginHandler.GetLastFailure},
			routeDescriptor{Method: http.MethodDelete, Path: "/api/v1/shein-login/accounts/:store_id/last-failure", Module: "shein-login", Handler: sheinLoginHandler.ClearLastFailure},
		)
	}

	if sdsLoginHandler != nil {
		routes = append(routes,
			routeDescriptor{Method: http.MethodGet, Path: "/api/v1/sds-login/health", Module: "sds-login", Handler: sdsLoginHandler.Health},
			routeDescriptor{Method: http.MethodGet, Path: "/api/v1/sds-login/status", Module: "sds-login", Handler: sdsLoginHandler.Status},
			routeDescriptor{Method: http.MethodPost, Path: "/api/v1/sds-login/login", Module: "sds-login", Handler: sdsLoginHandler.Login},
			routeDescriptor{Method: http.MethodPost, Path: "/api/v1/sds-login/manual-login", Module: "sds-login", Handler: sdsLoginHandler.ManualLogin},
			routeDescriptor{Method: http.MethodGet, Path: "/api/v1/sds-login/auth-state", Module: "sds-login", Handler: sdsLoginHandler.GetAuthState},
			routeDescriptor{Method: http.MethodDelete, Path: "/api/v1/sds-login/state", Module: "sds-login", Handler: sdsLoginHandler.ClearState},
			routeDescriptor{Method: http.MethodGet, Path: "/sds-login", Module: "sds-login", Handler: sdsLoginHandler.AdminPage},
		)
	}

	return routes
}

func mountRoutes(r *gin.Engine, routes []routeDescriptor) {
	for _, route := range routes {
		r.Handle(route.Method, route.Path, route.Handler)
	}
}
