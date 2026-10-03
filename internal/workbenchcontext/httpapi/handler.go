package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"task-processor/internal/authidentity"
	"task-processor/internal/authz"
)

const switchOrganizationRequestBodyMaxBytes = 4096

// Handler exposes only the verified, resolved workbench identity projection.
type Handler struct {
	workbenchAuthorizer          *authz.ListingKitAuthorizer
	profileReader                authidentity.SelfProfileReader
	aiWorkbenchAvailable         bool
	aiWorkbenchAdmission         func(string) bool
	aiWorkbenchPlanningReadiness func(context.Context, string) string
	aiWorkbenchTitleReadiness    func(context.Context, string) string
}

// SetAIWorkbenchAvailable is called during composition, before HTTP serving.
func (h *Handler) SetAIWorkbenchAvailable(available bool) {
	if h != nil {
		h.aiWorkbenchAvailable = available
	}
}

// SetAIWorkbenchAdmission projects the mounted module's organization allowlist
// for the selected organization. It does not grant any Chat or Task permission.
func (h *Handler) SetAIWorkbenchAdmission(admitted func(string) bool) {
	if h != nil {
		h.aiWorkbenchAdmission = admitted
	}
}

// SetAIWorkbenchPlanningReadiness is wired to the planner's exact organization
// policy and credential resolver before HTTP serving. It returns no secrets.
func (h *Handler) SetAIWorkbenchPlanningReadiness(read func(context.Context, string) string) {
	if h != nil {
		h.aiWorkbenchPlanningReadiness = read
	}
}

// SetAIWorkbenchTitleReadiness projects the current organization's title
// execution route for existing proposals. It never authorizes confirmation.
func (h *Handler) SetAIWorkbenchTitleReadiness(read func(context.Context, string) string) {
	if h != nil {
		h.aiWorkbenchTitleReadiness = read
	}
}

func (h *Handler) SetSelfProfileReader(reader authidentity.SelfProfileReader) {
	h.profileReader = reader
}

func NewHandler() *Handler { return &Handler{} }

func NewHandlerWithWorkbenchAuthorizer(authorizer *authz.ListingKitAuthorizer) *Handler {
	return &Handler{workbenchAuthorizer: authorizer}
}

func (h *Handler) SetWorkbenchAuthorizer(authorizer *authz.ListingKitAuthorizer) {
	if h != nil {
		h.workbenchAuthorizer = authorizer
	}
}

// ResolveSwitchOrganizationTarget decodes a switch candidate and restores the
// request body for the downstream handler.
func ResolveSwitchOrganizationTarget(request *http.Request) (string, error) {
	if request == nil || request.Body == nil {
		return "", errors.New("request body is required")
	}
	body, err := io.ReadAll(io.LimitReader(request.Body, switchOrganizationRequestBodyMaxBytes+1))
	if err != nil {
		return "", errors.New("read request body")
	}
	request.Body = io.NopCloser(bytes.NewReader(body))
	if len(body) > switchOrganizationRequestBodyMaxBytes {
		return "", errors.New("request body is too large")
	}
	decoder := json.NewDecoder(bytes.NewReader(body))
	opening, err := decoder.Token()
	if err != nil || opening != json.Delim('{') {
		return "", errors.New("request body is invalid")
	}
	fieldCount := 0
	organizationID := ""
	for decoder.More() {
		fieldToken, tokenErr := decoder.Token()
		field, ok := fieldToken.(string)
		if tokenErr != nil || !ok || field != "organizationId" || fieldCount != 0 {
			return "", errors.New("request body contains an unexpected field")
		}
		fieldCount++
		if err := decoder.Decode(&organizationID); err != nil {
			return "", errors.New("organizationId must be a string")
		}
	}
	closing, err := decoder.Token()
	if err != nil || closing != json.Delim('}') || fieldCount != 1 {
		return "", errors.New("request body is invalid")
	}
	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF {
		return "", errors.New("request body has trailing JSON")
	}
	organizationID = strings.TrimSpace(organizationID)
	if organizationID == "" {
		return "", errors.New("organizationId is required")
	}
	if headerTarget := strings.TrimSpace(request.Header.Get("X-Requested-Organization-ID")); headerTarget != "" && headerTarget != organizationID {
		return "", errors.New("organization target mismatch")
	}
	return organizationID, nil
}

func (h *Handler) GetContext(c *gin.Context) {
	h.writeContext(c)
}

func (h *Handler) SwitchEffectiveOrganization(c *gin.Context) {
	if _, err := ResolveSwitchOrganizationTarget(c.Request); err != nil {
		writeProtocolError(c, http.StatusBadRequest, "INVALID_REQUEST", "Request is invalid")
		return
	}
	h.writeContext(c)
}

func (h *Handler) writeContext(c *gin.Context) {
	identity, ok := authidentity.AuthenticatedIdentityFromContext(c.Request.Context())
	if !ok {
		writeProtocolError(c, http.StatusUnauthorized, "AUTHENTICATION_REQUIRED", "Authentication is required")
		return
	}

	organizations := make([]organizationResponse, 0, len(identity.OrganizationGrants))
	for _, grant := range identity.OrganizationGrants {
		canManageSourceAccount := h.workbenchAuthorizer != nil && h.workbenchAuthorizer.Authorize(identity.UserID, grant.Roles, authz.PermissionWorkbenchSourceAccountManage)
		canUseChat := h.workbenchAuthorizer != nil && h.workbenchAuthorizer.Authorize(identity.UserID, grant.Roles, authz.PermissionWorkbenchChatUse)
		roles := append([]string(nil), grant.Roles...)
		if h.workbenchAuthorizer != nil && h.workbenchAuthorizer.Authorize(identity.UserID, roles, authz.PermissionWorkbenchStoreDelete) && !containsRole(roles, "platform_admin") {
			roles = append(roles, "platform_admin")
		}
		if roles == nil {
			roles = []string{}
		}
		organizations = append(organizations, organizationResponse{
			ID:           grant.OrganizationID,
			Name:         grant.OrganizationName,
			Roles:        roles,
			Capabilities: organizationCapabilitiesResponse{SourceAccountManage: canManageSourceAccount, ChatUse: canUseChat},
		})
	}

	var effectiveOrganizationID *string
	if effective := strings.TrimSpace(identity.EffectiveOrganizationID); effective != "" {
		effectiveOrganizationID = &effective
	}
	planningReadiness := ""
	titleReadiness := ""
	available := false
	if h.aiWorkbenchAvailable {
		for _, organization := range organizations {
			if effectiveOrganizationID != nil && organization.ID == *effectiveOrganizationID {
				available = h.aiWorkbenchAdmission != nil && h.aiWorkbenchAdmission(organization.ID)
				if !available {
					break
				}
				planningReadiness = "UNAVAILABLE"
				titleReadiness = "UNAVAILABLE"
				if h.aiWorkbenchPlanningReadiness != nil {
					candidate := h.aiWorkbenchPlanningReadiness(c.Request.Context(), organization.ID)
					if candidate == "AVAILABLE" || candidate == "NEEDS_CONFIGURATION" {
						planningReadiness = candidate
					}
				}
				if h.aiWorkbenchTitleReadiness != nil {
					candidate := h.aiWorkbenchTitleReadiness(c.Request.Context(), organization.ID)
					if candidate == "AVAILABLE" || candidate == "NEEDS_CONFIGURATION" {
						titleReadiness = candidate
					}
				}
				break
			}
		}
	}
	c.JSON(http.StatusOK, contextResponse{
		User:                         userResponse{ID: identity.UserID},
		HomeOrganizationID:           identity.HomeOrganizationID,
		EffectiveOrganizationID:      effectiveOrganizationID,
		SelectionRequired:            effectiveOrganizationID == nil && len(organizations) > 1,
		Organizations:                organizations,
		AIWorkbenchAvailable:         available,
		AIWorkbenchPlanningReadiness: planningReadiness,
		AIWorkbenchTitleReadiness:    titleReadiness,
	})
}

func containsRole(roles []string, want string) bool {
	for _, role := range roles {
		if role == want {
			return true
		}
	}
	return false
}

type contextResponse struct {
	User                         userResponse           `json:"user"`
	HomeOrganizationID           string                 `json:"homeOrganizationId"`
	EffectiveOrganizationID      *string                `json:"effectiveOrganizationId"`
	SelectionRequired            bool                   `json:"selectionRequired"`
	Organizations                []organizationResponse `json:"organizations"`
	AIWorkbenchAvailable         bool                   `json:"aiWorkbenchAvailable,omitempty"`
	AIWorkbenchPlanningReadiness string                 `json:"aiWorkbenchPlanningReadiness,omitempty"`
	AIWorkbenchTitleReadiness    string                 `json:"aiWorkbenchTitleReadiness,omitempty"`
}

type userResponse struct {
	ID string `json:"id"`
}

type organizationResponse struct {
	ID           string                           `json:"id"`
	Name         string                           `json:"name"`
	Roles        []string                         `json:"roles"`
	Capabilities organizationCapabilitiesResponse `json:"capabilities"`
}

type organizationCapabilitiesResponse struct {
	SourceAccountManage bool `json:"workbench.source_account.manage"`
	ChatUse             bool `json:"workbench.chat.use"`
}

func writeProtocolError(c *gin.Context, status int, code string, message string) {
	c.AbortWithStatusJSON(status, gin.H{
		"code":        code,
		"message":     message,
		"requestId":   strings.TrimSpace(c.GetHeader("X-Request-ID")),
		"fieldErrors": []any{},
	})
}
