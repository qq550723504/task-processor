package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"task-processor/internal/authidentity"
	"task-processor/internal/authz"
	"task-processor/internal/httproute"
	domain "task-processor/internal/subjectverification"
)

const BasePath = "/api/v1/account/organization/verification"
const CallbackPath = "/api/v1/callbacks/tencent-esign/verification"

type service interface {
	Start(context.Context, domain.Actor, domain.Input) (domain.Application, error)
	Read(context.Context, domain.Actor) (domain.Application, string, error)
	Observe(context.Context, domain.Event) error
}
type callback interface {
	Parse([]byte, string) (domain.Event, bool, error)
}
type Handler struct {
	Service  service
	Profile  authidentity.SelfProfileReader
	Callback callback
}

func (h Handler) Routes(target httproute.OrganizationTargetResolver) []httproute.Descriptor {
	routes := []httproute.Descriptor{}
	for _, method := range []string{http.MethodGet, http.MethodPost} {
		path, handler := BasePath, h.read
		if method == http.MethodPost {
			path += "/applications"
			handler = h.start
		}
		routes = append(routes, httproute.Descriptor{Method: method, Path: path, Module: "subject-verification", AuthPolicy: httproute.AuthPolicyCurrentIdentity, Permission: authz.PermissionWorkbenchOrganizationMemberManage, OrganizationAccessPolicy: httproute.OrganizationAccessPolicyLiveWrite, OrganizationTargetResolver: target, RequestTimeout: 15 * time.Second, RejectUnreadRequestBody: method == http.MethodGet, Handler: handler})
	}
	return append(routes, httproute.Descriptor{Method: http.MethodPost, Path: CallbackPath, Module: "subject-verification", AuthPolicy: httproute.AuthPolicyPublic, OrganizationAccessPolicy: httproute.OrganizationAccessPolicyNone, RequestTimeout: 8 * time.Second, Handler: h.observe})
}
func respond(c *gin.Context, status int, value any) {
	c.Header("Cache-Control", "private, no-store")
	c.Header("X-Content-Type-Options", "nosniff")
	c.JSON(status, value)
}
func fail(c *gin.Context, status int, code string) { respond(c, status, gin.H{"code": code}) }
func actor(c *gin.Context) (domain.Actor, bool) {
	id, ok := authidentity.AuthenticatedIdentityFromContext(c.Request.Context())
	if !ok || !authidentity.IsBoundedIdentifier(id.UserID) || !authidentity.IsBoundedIdentifier(id.EffectiveOrganizationID) {
		fail(c, 401, "AUTHENTICATION_REQUIRED")
		return domain.Actor{}, false
	}
	return domain.Actor{OrganizationID: id.EffectiveOrganizationID, UserID: id.UserID}, true
}
func (h Handler) phone(c *gin.Context, a domain.Actor) (string, error) {
	if h.Profile == nil {
		return "", domain.ErrUnavailable
	}
	authorization := c.GetHeader("Authorization")
	if !strings.HasPrefix(authorization, "Bearer ") {
		return "", domain.ErrInvalid
	}
	p, err := h.Profile.ReadSelf(c.Request.Context(), strings.TrimPrefix(authorization, "Bearer "), a.UserID)
	if err != nil {
		return "", domain.ErrUnavailable
	}
	if p.UserID != a.UserID || p.PhoneNumberVerified == nil || !*p.PhoneNumberVerified || p.PhoneNumber == nil || !strings.HasPrefix(*p.PhoneNumber, "+86") || domain.NormalizePhone(*p.PhoneNumber) == "" {
		return "", domain.ErrInvalid
	}
	return *p.PhoneNumber, nil
}
func (h Handler) read(c *gin.Context) {
	a, ok := actor(c)
	if !ok {
		return
	}
	if c.Request.URL.RawQuery != "" || c.Request.URL.ForceQuery || c.Request.ContentLength != 0 || len(c.Request.TransferEncoding) > 0 {
		fail(c, 400, "INVALID_REQUEST")
		return
	}
	if h.Service == nil {
		fail(c, 503, "VERIFICATION_UNAVAILABLE")
		return
	}
	app, link, err := h.Service.Read(c.Request.Context(), a)
	if errors.Is(err, domain.ErrNotFound) {
		phone, phoneErr := h.phone(c, a)
		masked := ""
		if phoneErr == nil {
			p := domain.NormalizePhone(phone)
			masked = p[:3] + "****" + p[7:]
		}
		respond(c, 200, gin.H{"state": "NOT_STARTED", "organizationId": a.OrganizationID, "userId": a.UserID, "canStart": phoneErr == nil, "maskedPhone": masked})
		return
	}
	if err != nil {
		writeError(c, err)
		return
	}
	writeApplication(c, a, app, link)
}
func (h Handler) start(c *gin.Context) {
	a, ok := actor(c)
	if !ok {
		return
	}
	if h.Service == nil {
		fail(c, 503, "VERIFICATION_UNAVAILABLE")
		return
	}
	if c.Request.URL.RawQuery != "" || c.Request.URL.ForceQuery || strings.Split(c.GetHeader("Content-Type"), ";")[0] != "application/json" {
		fail(c, 400, "INVALID_REQUEST")
		return
	}
	raw, err := io.ReadAll(io.LimitReader(c.Request.Body, 8193))
	if err != nil || len(raw) > 8192 {
		fail(c, 400, "INVALID_REQUEST")
		return
	}
	var in domain.Input
	decoder := json.NewDecoder(strings.NewReader(string(raw)))
	decoder.DisallowUnknownFields()
	if decoder.Decode(&in) != nil {
		fail(c, 400, "INVALID_REQUEST")
		return
	}
	var extra any
	if decoder.Decode(&extra) != io.EOF {
		fail(c, 400, "INVALID_REQUEST")
		return
	}
	phone, err := h.phone(c, a)
	if err != nil {
		if errors.Is(err, domain.ErrInvalid) {
			fail(c, 409, "VERIFIED_PHONE_REQUIRED")
		} else {
			writeError(c, err)
		}
		return
	}
	a.VerifiedPhone = phone
	app, err := h.Service.Start(c.Request.Context(), a, in)
	if err != nil {
		writeError(c, err)
		return
	}
	// A lost create response is represented by the durable application, not an
	// invitation to create another external authentication flow.
	current, link, readErr := h.Service.Read(c.Request.Context(), a)
	if readErr == nil {
		app = current
	}
	writeApplication(c, a, app, link)
}
func writeApplication(c *gin.Context, actor domain.Actor, a domain.Application, link string) {
	state := a.State
	if state == domain.Pending && !a.ExpiresAt.After(time.Now()) {
		state = "EXPIRED"
		link = ""
	}
	value := gin.H{"state": state, "applicationId": a.ID, "organizationId": actor.OrganizationID, "userId": actor.UserID, "companyName": a.CompanyName, "creditCode": a.CreditCode, "maskedPhone": a.MaskedPhone, "canStart": false, "isApplicant": a.ActorID == actor.UserID}
	if link != "" {
		value["verificationUrl"] = link
	}
	if !a.ExpiresAt.IsZero() {
		value["expiresAt"] = a.ExpiresAt.UTC().Format(time.RFC3339)
	}
	if a.State == domain.Verified {
		value["verifiedAt"] = a.ProviderVerifiedAt.UTC().Format(time.RFC3339)
	}
	respond(c, 200, value)
}
func (h Handler) observe(c *gin.Context) {
	if h.Service == nil || h.Callback == nil {
		fail(c, 503, "VERIFICATION_UNAVAILABLE")
		return
	}
	raw, err := io.ReadAll(io.LimitReader(c.Request.Body, 65537))
	if err != nil || len(raw) > 65536 {
		fail(c, 400, "INVALID_REQUEST")
		return
	}
	event, supported, err := h.Callback.Parse(raw, c.GetHeader("Content-Signature"))
	if err != nil {
		fail(c, 401, "INVALID_SIGNATURE")
		return
	}
	if supported {
		if err = h.Service.Observe(c.Request.Context(), event); err != nil && !errors.Is(err, domain.ErrNotFound) {
			writeError(c, err)
			return
		}
	}
	respond(c, 200, gin.H{"code": "OK"})
}
func writeError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, domain.ErrInvalid):
		fail(c, 400, "INVALID_REQUEST")
	case errors.Is(err, domain.ErrConflict):
		fail(c, 409, "VERIFICATION_CONFLICT")
	default:
		fail(c, 503, "VERIFICATION_UNAVAILABLE")
	}
}
