package httpapi

import (
	"context"
	"errors"
	"io"
	"mime"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
	"gorm.io/gorm"
	"task-processor/internal/app/productsourcing"
	"task-processor/internal/authz"
	"task-processor/internal/core/config"
	"task-processor/internal/httproute"
	a1688 "task-processor/internal/integration/acquisition/a1688"
	acquisitionstore "task-processor/internal/integration/persistence/product/acquisition"
	kernelmodule "task-processor/internal/kernel/module"
	"task-processor/internal/product/sourcing"
)

const browserCaptureBase = "/api/v1/workbench/sourcing/1688/browser-captures"

// NewCurrentApplicationWithBrowserCapture is an explicit opt-in composition.
// The existing 10-route and Public-only 13-route constructors remain unchanged.
// Caller-owned pools are inspected read-only; this is not a production switch.
func NewCurrentApplicationWithBrowserCapture(ctx context.Context, sourceAccountDB, commercialDB, productDB *gorm.DB, cfg *config.Config, logger *logrus.Logger) (*http.Server, error) {
	if ctx == nil || productDB == nil {
		return nil, sourcing.ErrAcquisitionUnavailable
	}
	factories := defaultCurrentApplicationFactories(ctx)
	factories.buildAcquisition = func(authorizer *authz.ListingKitAuthorizer, dependencies routeAuthDependencies) (kernelmodule.Module, error) {
		return buildProductAcquisitionModule(ctx, productDB, dependencies, authorizer, a1688.New())
	}
	factories.buildBrowserCapture = func(authorizer *authz.ListingKitAuthorizer, dependencies routeAuthDependencies) (kernelmodule.Module, error) {
		return buildBrowserCaptureModule(ctx, productDB, dependencies, authorizer)
	}
	return buildCurrentApplication(ctx, sourceAccountDB, commercialDB, cfg, logger, factories)
}

type browserCaptureService interface {
	Capture(context.Context, string, []byte) (sourcing.AcquisitionResult, error)
	Verify(context.Context, string, []byte) (sourcing.AcquisitionResult, error)
	ByKey(context.Context, string) (sourcing.AcquisitionResult, error)
	Read(context.Context, string) (sourcing.AcquisitionResult, error)
}

func browserCaptureRoutes(service browserCaptureService, bind func(context.Context, string) (context.Context, error)) []httproute.Descriptor {
	specs := []struct{ method, path, action string }{
		{http.MethodPost, browserCaptureBase, "capture"},
		{http.MethodPost, browserCaptureBase + "/verify", "verify"},
		{http.MethodGet, browserCaptureBase + "/by-key/:key", "by-key"},
		{http.MethodGet, browserCaptureBase + "/:operation_id", "read"},
	}
	routes := make([]httproute.Descriptor, 0, len(specs))
	for _, spec := range specs {
		routes = append(routes, httproute.Descriptor{Method: spec.method, Path: spec.path, Module: "browser-capture", Permission: "product_sourcing.write", AuthPolicy: httproute.AuthPolicyVerifiedIdentity, OrganizationAccessPolicy: httproute.OrganizationAccessPolicyLiveWrite, RequestTimeout: sourcing.AcquisitionTimeout, RejectUnreadRequestBody: true, Handler: func(c *gin.Context) {
			if service == nil || bind == nil {
				writeBrowserCaptureError(c, sourcing.ErrAcquisitionUnavailable)
				return
			}
			ctx, err := bind(c.Request.Context(), c.GetHeader("Authorization"))
			if err != nil {
				writeBrowserCaptureError(c, sourcing.ErrPublicationForbidden)
				return
			}
			if c.Request.URL.RawQuery != "" || c.Request.URL.ForceQuery {
				writeBrowserCaptureError(c, sourcing.ErrInvalidAcquisition)
				return
			}
			var result sourcing.AcquisitionResult
			if spec.method == http.MethodGet {
				if c.Request.Body != nil {
					raw, err := io.ReadAll(io.LimitReader(c.Request.Body, 1))
					if err != nil || len(raw) > 0 {
						writeBrowserCaptureError(c, sourcing.ErrInvalidAcquisition)
						return
					}
				}
				if spec.action == "by-key" {
					key := c.Param("key")
					if !acquisitionHTTPUUID(key) {
						writeBrowserCaptureError(c, sourcing.ErrInvalidAcquisition)
						return
					}
					result, err = service.ByKey(ctx, key)
				} else {
					id := c.Param("operation_id")
					if !acquisitionHTTPUUID(id) {
						writeBrowserCaptureError(c, sourcing.ErrInvalidAcquisition)
						return
					}
					result, err = service.Read(ctx, id)
				}
			} else {
				key, body, readErr := readBrowserCaptureRequest(c.Request)
				if readErr != nil {
					writeBrowserCaptureError(c, readErr)
					return
				}
				if spec.action == "verify" {
					result, err = service.Verify(ctx, key, body)
				} else {
					result, err = service.Capture(ctx, key, body)
				}
			}
			if err != nil {
				writeBrowserCaptureError(c, err)
				return
			}
			writeAcquisitionResult(c, result)
		}})
	}
	return routes
}

func readBrowserCaptureRequest(request *http.Request) (string, []byte, error) {
	keys := request.Header.Values("Idempotency-Key")
	if len(keys) != 1 || !acquisitionHTTPUUID(keys[0]) {
		return "", nil, sourcing.ErrInvalidAcquisition
	}
	media, parameters, err := mime.ParseMediaType(request.Header.Get("Content-Type"))
	if err != nil || media != "application/json" || (parameters["charset"] != "" && !strings.EqualFold(parameters["charset"], "utf-8")) || (request.Header.Get("Content-Encoding") != "" && request.Header.Get("Content-Encoding") != "identity") || request.Body == nil {
		return "", nil, sourcing.ErrInvalidAcquisition
	}
	if request.ContentLength > sourcing.BrowserCaptureMaxBytes {
		return "", nil, sourcing.ErrSourcePublicationTooLarge
	}
	body, err := io.ReadAll(io.LimitReader(request.Body, sourcing.BrowserCaptureMaxBytes+1))
	if len(body) > sourcing.BrowserCaptureMaxBytes {
		return "", nil, sourcing.ErrSourcePublicationTooLarge
	}
	if err != nil {
		return "", nil, sourcing.ErrInvalidAcquisition
	}
	if _, err := sourcing.ParseBrowserCapture(body); err != nil {
		return "", nil, err
	}
	return keys[0], body, nil
}

func writeBrowserCaptureError(c *gin.Context, err error) {
	if errors.Is(err, sourcing.ErrInvalidBrowserCapture) {
		err = sourcing.ErrInvalidAcquisition
	}
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		err = sourcing.ErrAcquisitionUnknown
	}
	writeAcquisitionError(c, err)
}

type browserCaptureModule struct{ routes []httproute.Descriptor }

func (browserCaptureModule) Name() string { return "browser-capture" }
func (browserCaptureModule) Enabled(cfg *config.Config) bool {
	return cfg != nil && cfg.Workbench.Enabled
}
func (m browserCaptureModule) Register(reg *kernelmodule.Registry) error {
	reg.AddRoutes(m.routes...)
	return nil
}

func buildBrowserCaptureModule(ctx context.Context, db *gorm.DB, dependencies routeAuthDependencies, authorizer *authz.ListingKitAuthorizer) (kernelmodule.Module, error) {
	if dependencies.organizationResolver == nil || authorizer == nil {
		return nil, sourcing.ErrAcquisitionUnavailable
	}
	if err := acquisitionstore.VerifyRuntimePermissions(ctx, db); err != nil {
		return nil, err
	}
	live := &productReviewLiveOrganizationAccess{resolver: dependencies.organizationResolver, now: time.Now}
	service, err := productsourcing.NewBrowserAcquisition(ctx, db, live, authorizer)
	if err != nil {
		return nil, err
	}
	binder := productReviewCapabilityBinder{now: time.Now}
	return browserCaptureModule{routes: browserCaptureRoutes(service, binder.Bind)}, nil
}
