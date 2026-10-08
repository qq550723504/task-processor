package httpapi

import (
	"context"
	"errors"
	"github.com/gin-gonic/gin"
	"io"
	"net/http"
	"strings"
	"task-processor/internal/authidentity"
	"task-processor/internal/authz"
	"task-processor/internal/httproute"
	kernel "task-processor/internal/kernel/module"
	domain "task-processor/internal/organization/membership"
	flow "task-processor/internal/organization/membership/inviteflow"
	"time"
)

type invitationView struct {
	flow.Invitation
	Permissions []string `json:"permissions"`
}

func projectInvitation(ctx context.Context, service *flow.Service, inv flow.Invitation) (invitationView, error) {
	view := invitationView{Invitation: inv, Permissions: []string{}}
	if service.ScopedPermissions != nil {
		permissions, err := service.ScopedPermissions(ctx, inv.OrganizationID, []string{inv.Role})
		if err != nil {
			return view, domain.ErrUnavailable
		}
		view.Permissions = permissions
		return view, nil
	}
	if strings.HasPrefix(inv.Role, "sumi_role_") {
		return view, domain.ErrUnavailable
	}
	if service.Authorize != nil {
		for _, permission := range authz.WorkbenchPermissions() {
			if service.Authorize("", []string{inv.Role}, permission) {
				view.Permissions = append(view.Permissions, permission)
			}
		}
	}
	return view, nil
}

type InvitationFactory func(*http.Request) (*flow.Service, error)

func (h *Handler) ConfigureInvitations(factory InvitationFactory) { h.invitations = factory }
func (h *Handler) registerInvitations(reg *kernel.Registry) {
	if h.invitations == nil {
		return
	}
	for _, r := range []struct {
		method, path       string
		recipient, summary bool
	}{
		{"GET", "/api/v1/account/member-invitations", false, false}, {"POST", "/api/v1/account/member-invitations", false, false}, {"GET", "/api/v1/account/member-invitations/summary", false, true}, {"GET", "/api/v1/account/member-invitations/:invitation_id", false, false}, {"POST", "/api/v1/account/member-invitations/:invitation_id/cancel", false, false}, {"POST", "/api/v1/account/member-invitations/:invitation_id/resend", false, false},
		{"GET", "/api/v1/account/invitations/:invitation_id", true, false}, {"POST", "/api/v1/account/invitations/:invitation_id/accept", true, false}, {"POST", "/api/v1/account/invitations/:invitation_id/decline", true, false},
	} {
		policy := httproute.OrganizationAccessPolicyLiveWrite
		permission := authz.PermissionWorkbenchOrganizationMemberManage
		resolver := ResolveMutationTarget
		if r.summary {
			permission = authz.PermissionWorkbenchOrganizationMemberRead
		}
		if r.recipient {
			policy = httproute.OrganizationAccessPolicyNone
			permission = ""
			resolver = nil
		}
		reg.AddRoutes(httproute.Descriptor{Method: r.method, Path: r.path, Module: ModuleName, AuthPolicy: httproute.AuthPolicyCurrentIdentity, OrganizationAccessPolicy: policy, Permission: permission, OrganizationTargetResolver: resolver, RejectUnreadRequestBody: true, RequestTimeout: 15 * time.Second, Handler: httproute.WithRequestBodyReadTimeout(10*time.Second, h.invitationHandler(r.recipient, r.summary))})
	}
}
func (h *Handler) invitationHandler(recipient, summary bool) gin.HandlerFunc {
	return func(c *gin.Context) {
		if c.Request.URL.RawQuery != "" || c.Request.URL.ForceQuery {
			writeError(c, domain.ErrInvalidRequest)
			return
		}
		if !recipient {
			if err := validateMutationScope(c); err != nil {
				writeError(c, err)
				return
			}
		}
		service, err := h.invitations(c.Request)
		if err != nil {
			writeError(c, err)
			return
		}
		ctx := c.Request.Context()
		identity, _ := authidentity.AuthenticatedIdentityFromContext(ctx)
		id := c.Param("invitation_id")
		var data any
		var inv flow.Invitation
		if c.Request.Method == "POST" && !recipient && id == "" {
			var input struct {
				Email string `json:"email"`
				Role  string `json:"role"`
			}
			if len(c.Request.Header.Values("Idempotency-Key")) != 1 || decodeCommand(c.Request, &input) != nil {
				writeError(c, domain.ErrInvalidRequest)
				return
			}
			inv, err = service.Create(ctx, c.GetHeader("Idempotency-Key"), input.Email, input.Role)
		} else {
			if !emptyBody(c.Request) {
				writeError(c, domain.ErrInvalidRequest)
				return
			}
			switch {
			case summary:
				var count int
				count, err = service.PendingCount(ctx)
				data = gin.H{"schemaVersion": "membership-invitation-summary-v1", "userId": identity.UserID, "organizationId": identity.EffectiveOrganizationID, "pending": count}
			case id == "":
				var page flow.Page
				page, err = service.List(ctx, 100, 0)
				views := make([]invitationView, 0, len(page.Items))
				for _, item := range page.Items {
					view, projectionErr := projectInvitation(ctx, service, item)
					if projectionErr != nil {
						err = projectionErr
						break
					}
					views = append(views, view)
				}
				data = gin.H{"schemaVersion": "membership-invitations-v1", "userId": identity.UserID, "organizationId": identity.EffectiveOrganizationID, "items": views, "total": page.Total, "pending": page.Pending, "canNotify": service.Notify != nil}
			case strings.HasSuffix(c.FullPath(), "/accept"):
				inv, err = service.Accept(ctx, id)
			case strings.HasSuffix(c.FullPath(), "/decline"):
				inv, err = service.Decline(ctx, id)
			case strings.HasSuffix(c.FullPath(), "/cancel"):
				inv, err = service.Cancel(ctx, id)
			case strings.HasSuffix(c.FullPath(), "/resend"):
				inv, err = service.Resend(ctx, id)
			case recipient:
				inv, err = service.ReadRecipient(ctx, id)
			default:
				inv, err = service.ReadAdmin(ctx, id)
			}
		}
		if err != nil {
			switch {
			case errors.Is(err, flow.ErrPermission):
				err = domain.ErrPermission
			case errors.Is(err, flow.ErrConflict):
				err = domain.ErrConflict
			case errors.Is(err, flow.ErrNotFound):
				err = domain.ErrNotFound
			case errors.Is(err, flow.ErrInvalid):
				err = domain.ErrInvalidRequest
			default:
				err = domain.ErrUnavailable
			}
			writeError(c, err)
			return
		}
		if data == nil {
			view, projectionErr := projectInvitation(ctx, service, inv)
			if projectionErr != nil {
				writeError(c, projectionErr)
				return
			}
			data = gin.H{"schemaVersion": "membership-invitation-v1", "userId": identity.UserID, "organizationId": inv.OrganizationID, "invitation": view}
		}
		c.Header("Cache-Control", "private, no-store")
		c.JSON(http.StatusOK, data)
	}
}

// Deadline middleware already bounds body reads; disallow content on action/read routes.
func emptyBody(r *http.Request) bool {
	if r.Body == nil {
		return true
	}
	var b [1]byte
	n, err := r.Body.Read(b[:])
	return n == 0 && errors.Is(err, io.EOF) && r.Context().Err() == nil
}
