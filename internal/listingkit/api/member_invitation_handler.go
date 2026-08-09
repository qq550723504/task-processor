package api

import (
	"errors"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"task-processor/internal/listingkit/memberinvite"
)

type inviteTenantMemberRequest struct {
	GivenName  string `json:"given_name"`
	FamilyName string `json:"family_name"`
	Email      string `json:"email"`
	Role       string `json:"role"`
}

func (h *handler) InviteTenantMember(c *gin.Context) {
	if !h.requirePlatformSubscriptionAccess(c) {
		return
	}
	if h.memberInvitationAudit == nil {
		writeMemberInvitationError(c, http.StatusServiceUnavailable, "member_invitation_unavailable", "member invitation service is not configured")
		return
	}

	tenantID := strings.TrimSpace(c.Param("tenant_id"))
	var body inviteTenantMemberRequest
	request := memberinvite.InviteRequest{TenantID: tenantID}
	if err := c.ShouldBindJSON(&body); err != nil {
		h.finishMemberInvitation(c, request, memberinvite.Invitation{}, memberinvite.OutcomeFailed, http.StatusBadRequest, "invalid_member_invitation", "member invitation request is invalid")
		return
	}
	request = memberinvite.InviteRequest{
		TenantID:   tenantID,
		GivenName:  strings.TrimSpace(body.GivenName),
		FamilyName: strings.TrimSpace(body.FamilyName),
		Email:      strings.TrimSpace(body.Email),
		Role:       strings.TrimSpace(body.Role),
	}
	if h.memberInvitationService == nil {
		h.finishMemberInvitation(c, request, memberinvite.Invitation{}, memberinvite.OutcomeFailed, http.StatusServiceUnavailable, "member_invitation_unavailable", "member invitation service is not configured")
		return
	}
	if !h.requireKnownInvitationTenant(c, request) {
		return
	}
	invitation, err := h.memberInvitationService.Invite(c.Request.Context(), request)
	if err != nil {
		h.writeMemberInvitationFailure(c, request, err)
		return
	}

	if !h.recordMemberInvitationOutcome(c, request, invitation, memberinvite.OutcomeSucceeded, "") {
		return
	}
	c.JSON(http.StatusCreated, gin.H{
		"tenant_id":             invitation.TenantID,
		"user_id":               invitation.UserID,
		"email":                 invitation.Email,
		"role":                  invitation.Role,
		"authorization_id":      invitation.AuthorizationID,
		"invitation_email_sent": true,
	})
}

func (h *handler) requireKnownInvitationTenant(c *gin.Context, request memberinvite.InviteRequest) bool {
	if request.TenantID == "" {
		h.finishMemberInvitation(c, request, memberinvite.Invitation{}, memberinvite.OutcomeFailed, http.StatusNotFound, "tenant_not_found", "tenant was not found")
		return false
	}
	if h.tenantDirectory == nil {
		h.finishMemberInvitation(c, request, memberinvite.Invitation{}, memberinvite.OutcomeFailed, http.StatusServiceUnavailable, "member_invitation_unavailable", "member invitation service is not configured")
		return false
	}
	tenants, err := h.tenantDirectory.List(c.Request.Context())
	if err != nil {
		h.finishMemberInvitation(c, request, memberinvite.Invitation{}, memberinvite.OutcomeFailed, http.StatusBadGateway, "zitadel_member_invitation_failed", "ZITADEL member invitation failed")
		return false
	}
	for _, tenant := range tenants {
		if strings.TrimSpace(tenant.ID) == request.TenantID {
			return true
		}
	}
	h.finishMemberInvitation(c, request, memberinvite.Invitation{}, memberinvite.OutcomeFailed, http.StatusNotFound, "tenant_not_found", "tenant was not found")
	return false
}

func (h *handler) writeMemberInvitationFailure(c *gin.Context, request memberinvite.InviteRequest, err error) {
	status := http.StatusBadGateway
	errorCode := "zitadel_member_invitation_failed"
	message := "ZITADEL member invitation failed"
	outcome := memberinvite.OutcomeFailed
	invitation := memberinvite.Invitation{}

	var incomplete *memberinvite.IncompleteError
	switch {
	case errors.Is(err, memberinvite.ErrInvalidRequest):
		status = http.StatusBadRequest
		errorCode = "invalid_member_invitation"
		message = "member invitation request is invalid"
	case errors.Is(err, memberinvite.ErrConflict):
		status = http.StatusConflict
		errorCode = "member_invitation_conflict"
		message = "member invitation conflicts with an existing identity or role assignment"
	case errors.As(err, &incomplete):
		errorCode = "zitadel_member_invitation_incomplete"
		message = "ZITADEL user was created, but ListingKit access was not assigned"
		outcome = memberinvite.OutcomeIncomplete
		invitation.UserID = strings.TrimSpace(incomplete.UserID)
	}

	h.finishMemberInvitation(c, request, invitation, outcome, status, errorCode, message)
}

func (h *handler) finishMemberInvitation(c *gin.Context, request memberinvite.InviteRequest, invitation memberinvite.Invitation, outcome memberinvite.Outcome, status int, errorCode, message string) {
	if !h.recordMemberInvitationOutcome(c, request, invitation, outcome, errorCode) {
		return
	}
	payload := gin.H{"error": errorCode, "message": message}
	if invitation.UserID != "" {
		payload["user_id"] = invitation.UserID
	}
	c.JSON(status, payload)
}

func (h *handler) recordMemberInvitationOutcome(c *gin.Context, request memberinvite.InviteRequest, invitation memberinvite.Invitation, outcome memberinvite.Outcome, errorCode string) bool {
	if h.memberInvitationAudit == nil {
		writeMemberInvitationError(c, http.StatusServiceUnavailable, "member_invitation_unavailable", "member invitation service is not configured")
		return false
	}
	err := h.memberInvitationAudit.Record(c.Request.Context(), memberinvite.AuditRecord{
		ActorUserID:     strings.TrimSpace(c.GetHeader("X-User-ID")),
		TenantID:        request.TenantID,
		Email:           request.Email,
		Role:            request.Role,
		UserID:          invitation.UserID,
		AuthorizationID: invitation.AuthorizationID,
		Outcome:         outcome,
		ErrorCode:       errorCode,
	})
	if err != nil {
		payload := gin.H{
			"error":   "member_invitation_unavailable",
			"message": "member invitation outcome could not be audited; inspect ZITADEL before retrying",
		}
		if invitation.UserID != "" {
			payload["user_id"] = invitation.UserID
		}
		if invitation.AuthorizationID != "" {
			payload["authorization_id"] = invitation.AuthorizationID
		}
		c.JSON(http.StatusServiceUnavailable, payload)
		return false
	}
	return true
}

func writeMemberInvitationError(c *gin.Context, status int, errorCode, message string) {
	c.JSON(status, gin.H{"error": errorCode, "message": message})
}
