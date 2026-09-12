package httpapi

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"strconv"

	"github.com/gin-gonic/gin"
	"task-processor/internal/authidentity"
	"task-processor/internal/organization/membership"
)

type Handler struct {
	service  *membership.Service
	commands CommandFactory
}

func NewHandler(service *membership.Service) *Handler { return &Handler{service: service} }

func ResolveOrganizationTarget(r *http.Request) (string, error) {
	if r.ContentLength != 0 || len(r.TransferEncoding) != 0 || len(r.Header.Values("X-Requested-Organization-ID")) != 1 {
		return "", membership.ErrInvalidRequest
	}
	org := r.Header.Get("X-Requested-Organization-ID")
	if !authidentity.IsBoundedIdentifier(org) {
		return "", membership.ErrInvalidRequest
	}
	return org, nil
}

func validateScope(c *gin.Context) error {
	org, err := ResolveOrganizationTarget(c.Request)
	if err != nil {
		return err
	}
	identity, ok := authidentity.AuthenticatedIdentityFromContext(c.Request.Context())
	if !ok {
		return membership.ErrAuthentication
	}
	if identity.EffectiveOrganizationID != org {
		return membership.ErrPermission
	}
	return nil
}

func (h *Handler) List(c *gin.Context) {
	if err := validateScope(c); err != nil {
		writeError(c, err)
		return
	}
	page, err := parsePage(c.Request.URL)
	if err != nil {
		writeError(c, err)
		return
	}
	result, err := h.service.List(c.Request.Context(), page)
	respond(c, result, err)
}
func (h *Handler) Read(c *gin.Context) {
	if err := validateScope(c); err != nil {
		writeError(c, err)
		return
	}
	if c.Request.URL.RawQuery != "" || c.Request.URL.ForceQuery {
		writeError(c, membership.ErrInvalidRequest)
		return
	}
	result, err := h.service.Read(c.Request.Context(), c.Param("member_id"))
	respond(c, result, err)
}

func parsePage(u *url.URL) (membership.PageRequest, error) {
	page := membership.PageRequest{Limit: 20}
	if len(u.RawQuery) > 128 || u.ForceQuery {
		return page, membership.ErrInvalidRequest
	}
	query, err := url.ParseQuery(u.RawQuery)
	if err != nil {
		return page, membership.ErrInvalidRequest
	}
	for key, values := range query {
		if (key != "limit" && key != "offset") || len(values) != 1 || values[0] == "" {
			return page, membership.ErrInvalidRequest
		}
		for _, c := range values[0] {
			if c < '0' || c > '9' {
				return page, membership.ErrInvalidRequest
			}
		}
		value, err := strconv.Atoi(values[0])
		if err != nil {
			return page, membership.ErrInvalidRequest
		}
		if key == "limit" {
			page.Limit = value
		} else {
			page.Offset = value
		}
	}
	if page.Limit < 1 || page.Limit > 100 || page.Offset < 0 || page.Offset > 10000 {
		return page, membership.ErrInvalidRequest
	}
	return page, nil
}

func respond(c *gin.Context, result membership.Result, err error) {
	if err != nil {
		writeError(c, err)
		return
	}
	if c.Request.Context().Err() != nil {
		writeError(c, membership.ErrUnavailable)
		return
	}
	body, err := json.Marshal(result)
	if err != nil || len(body) > 1024*1024 {
		writeError(c, membership.ErrInvalidResponse)
		return
	}
	responseHeaders(c)
	c.Data(http.StatusOK, "application/json", body)
}
func responseHeaders(c *gin.Context) {
	c.Header("Cache-Control", "private, no-store")
	c.Header("X-Content-Type-Options", "nosniff")
}
func writeError(c *gin.Context, err error) {
	status, code := 503, "DEPENDENCY_UNAVAILABLE"
	switch {
	case errors.Is(err, membership.ErrAuthentication):
		status, code = 401, "AUTHENTICATION_REQUIRED"
	case errors.Is(err, membership.ErrPermission):
		status, code = 403, "PERMISSION_DENIED"
	case errors.Is(err, membership.ErrInvalidRequest):
		status, code = 400, "INVALID_REQUEST"
	case errors.Is(err, membership.ErrNotFound):
		status, code = 404, "MEMBER_NOT_FOUND"
	case errors.Is(err, membership.ErrConflict):
		status, code = 409, "MEMBER_OPERATION_CONFLICT"
	case errors.Is(err, membership.ErrInvalidResponse):
		status, code = 502, "INVALID_UPSTREAM_RESPONSE"
	}
	if c.Request.Context().Err() != nil {
		status, code = 504, "DEADLINE_EXCEEDED"
	}
	responseHeaders(c)
	c.AbortWithStatusJSON(status, gin.H{"code": code, "message": "Member request could not be completed", "requestId": "", "fieldErrors": []any{}})
}
