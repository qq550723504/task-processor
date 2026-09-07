// Package httpapi exposes the current organization's bounded commercial read.
package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"task-processor/internal/authz"
	"task-processor/internal/core/config"
	"task-processor/internal/httproute"
	kernelmodule "task-processor/internal/kernel/module"
	"task-processor/internal/listingsubscription"
)

type Reader interface {
	Read(context.Context) (*listingsubscription.CommercialOverview, error)
}
type Handler struct{ reader Reader }

func NewHandler(reader Reader) *Handler { return &Handler{reader: reader} }

func (h *Handler) Get(c *gin.Context) {
	c.Header("Cache-Control", "private, no-store")
	c.Header("X-Content-Type-Options", "nosniff")
	request := c.Request
	if request.Method != http.MethodGet {
		writeError(c, 405, "INVALID_REQUEST")
		return
	}
	if request.URL.RawQuery != "" || request.URL.ForceQuery || request.ContentLength != 0 || len(request.TransferEncoding) != 0 || (request.Body != nil && request.Body != http.NoBody) {
		writeError(c, 400, "INVALID_REQUEST")
		return
	}
	if h == nil || h.reader == nil {
		writeError(c, 503, "DEPENDENCY_UNAVAILABLE")
		return
	}
	result, err := h.reader.Read(request.Context())
	if err != nil {
		switch {
		case errors.Is(err, listingsubscription.ErrCommercialForbidden):
			writeError(c, 403, "PERMISSION_DENIED")
		case errors.Is(err, context.DeadlineExceeded), errors.Is(err, context.Canceled):
			writeError(c, 504, "DEADLINE_EXCEEDED")
		default:
			writeError(c, 503, "DEPENDENCY_UNAVAILABLE")
		}
		return
	}
	body, err := json.Marshal(result)
	if err != nil || result == nil || len(body) > listingsubscription.CommercialResponseMaxBytes {
		writeError(c, 503, "DEPENDENCY_UNAVAILABLE")
		return
	}
	if request.Context().Err() != nil {
		writeError(c, 504, "DEADLINE_EXCEEDED")
		return
	}
	c.Data(200, "application/json; charset=utf-8", body)
}

func writeError(c *gin.Context, status int, code string) {
	c.AbortWithStatusJSON(status, gin.H{"code": code, "message": "Commercial read could not be completed", "requestId": uuid.NewString(), "fieldErrors": []any{}})
}

type commercialModule struct{ handler *Handler }

func NewModule(handler *Handler) kernelmodule.Module { return commercialModule{handler: handler} }
func (commercialModule) Name() string                { return "commercial-read" }
func (m commercialModule) Enabled(cfg *config.Config) bool {
	return m.handler != nil && cfg != nil && cfg.Workbench.Enabled
}
func (m commercialModule) Register(registry *kernelmodule.Registry) error {
	registry.AddRoutes(httproute.Descriptor{Method: http.MethodGet, Path: "/api/v1/workbench/commercial/overview", Module: m.Name(), Permission: authz.PermissionListingKitAdminRead, AuthPolicy: httproute.AuthPolicyVerifiedIdentity,
		// The existing LiveWrite policy denotes live grant validation; this GET
		// performs no business mutation and introduces no new IAM policy.
		OrganizationAccessPolicy: httproute.OrganizationAccessPolicyLiveWrite, Handler: m.handler.Get})
	return nil
}
