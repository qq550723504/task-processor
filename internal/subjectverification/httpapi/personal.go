package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/gin-gonic/gin"
	"io"
	"net/http"
	"strconv"
	"strings"
	"task-processor/internal/authidentity"
	"task-processor/internal/httproute"
	domain "task-processor/internal/subjectverification"
	"time"
)

const PersonalBasePath = "/api/v1/account/verification"

type personalService interface {
	Start(context.Context, domain.Actor, domain.PersonalInput) error
	Read(context.Context, domain.Actor) (domain.PersonalView, error)
	Refresh(context.Context, domain.Actor, string) error
}
type PersonalHandler struct {
	Service personalService
	Profile authidentity.SelfProfileReader
}

func (h PersonalHandler) Routes() []httproute.Descriptor {
	routes := []httproute.Descriptor{}
	for _, r := range []struct {
		method, path string
		timeout      time.Duration
		handler      gin.HandlerFunc
	}{{"GET", PersonalBasePath, 10 * time.Second, h.read}, {"POST", PersonalBasePath + "/applications", 20 * time.Second, h.start}, {"POST", PersonalBasePath + "/refresh", 10 * time.Second, h.refresh}} {
		routes = append(routes, httproute.Descriptor{Method: r.method, Path: r.path, Module: "subject-verification", AuthPolicy: httproute.AuthPolicyCurrentIdentity, OrganizationAccessPolicy: httproute.OrganizationAccessPolicyNone, RequestTimeout: r.timeout, RejectUnreadRequestBody: r.method == "GET", Handler: withBodyDeadline(r.timeout, r.handler)})
	}
	return routes
}
func (h PersonalHandler) actor(c *gin.Context, requirePhone bool) (domain.Actor, bool) {
	identity, ok := authidentity.AuthenticatedIdentityFromContext(c.Request.Context())
	if !ok || !authidentity.IsBoundedIdentifier(identity.UserID) {
		fail(c, 401, "AUTHENTICATION_REQUIRED")
		return domain.Actor{}, false
	}
	a := domain.Actor{UserID: identity.UserID}
	if h.Service == nil {
		fail(c, 503, "VERIFICATION_UNAVAILABLE")
		return a, false
	}
	phone, err := (Handler{Profile: h.Profile}).phone(c, a)
	if err != nil && (requirePhone || !errors.Is(err, domain.ErrInvalid)) {
		if errors.Is(err, domain.ErrInvalid) {
			fail(c, 409, "VERIFIED_PHONE_REQUIRED")
		} else {
			writeError(c, err)
		}
		return a, false
	}
	a.VerifiedPhone = phone
	return a, true
}
func (h PersonalHandler) read(c *gin.Context) {
	if c.Request.URL.RawQuery != "" || c.Request.URL.ForceQuery || c.Request.ContentLength != 0 || len(c.Request.TransferEncoding) > 0 {
		fail(c, 400, "INVALID_REQUEST")
		return
	}
	a, ok := h.actor(c, false)
	if !ok {
		return
	}
	h.view(c, a)
}
func (h PersonalHandler) view(c *gin.Context, a domain.Actor) {
	v, err := h.Service.Read(c.Request.Context(), a)
	if err != nil {
		personalError(c, err)
		return
	}
	respond(c, 200, v)
}
func personalBody(c *gin.Context, v any) bool {
	if c.Request.URL.RawQuery != "" || c.Request.URL.ForceQuery || strings.Split(c.GetHeader("Content-Type"), ";")[0] != "application/json" {
		fail(c, 400, "INVALID_REQUEST")
		return false
	}
	raw, err := io.ReadAll(io.LimitReader(c.Request.Body, 16385))
	if err != nil || len(raw) > 16384 {
		fail(c, 400, "INVALID_REQUEST")
		return false
	}
	d := json.NewDecoder(strings.NewReader(string(raw)))
	d.DisallowUnknownFields()
	var extra any
	if d.Decode(v) != nil || d.Decode(&extra) != io.EOF {
		fail(c, 400, "INVALID_REQUEST")
		return false
	}
	return true
}
func (h PersonalHandler) start(c *gin.Context) {
	var in domain.PersonalInput
	if !personalBody(c, &in) {
		return
	}
	a, ok := h.actor(c, true)
	if !ok {
		return
	}
	if err := h.Service.Start(c.Request.Context(), a, in); err != nil {
		personalError(c, err)
		return
	}
	h.view(c, a)
}
func (h PersonalHandler) refresh(c *gin.Context) {
	var in struct {
		ApplicationID string `json:"applicationId"`
	}
	if !personalBody(c, &in) {
		return
	}
	a, ok := h.actor(c, true)
	if !ok {
		return
	}
	if err := h.Service.Refresh(c.Request.Context(), a, in.ApplicationID); err != nil {
		personalError(c, err)
		return
	}
	h.view(c, a)
}
func personalError(c *gin.Context, err error) {
	var limit *domain.PersonalLimitError
	if errors.As(err, &limit) {
		if limit.RetryAfter > 0 {
			c.Header("Retry-After", strconv.Itoa(limit.RetryAfter))
		}
		fail(c, http.StatusTooManyRequests, limit.Code)
		return
	}
	writeError(c, err)
}
