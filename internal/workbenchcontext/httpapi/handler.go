package httpapi

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"task-processor/internal/authidentity"
)

const switchOrganizationRequestBodyMaxBytes = 4096

// Handler exposes only the verified, resolved workbench identity projection.
type Handler struct{}

func NewHandler() *Handler { return &Handler{} }

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
		roles := append([]string(nil), grant.Roles...)
		if roles == nil {
			roles = []string{}
		}
		organizations = append(organizations, organizationResponse{
			ID:    grant.OrganizationID,
			Name:  grant.OrganizationName,
			Roles: roles,
		})
	}

	var effectiveOrganizationID *string
	if effective := strings.TrimSpace(identity.EffectiveOrganizationID); effective != "" {
		effectiveOrganizationID = &effective
	}
	c.JSON(http.StatusOK, contextResponse{
		User:                    userResponse{ID: identity.UserID},
		HomeOrganizationID:      identity.HomeOrganizationID,
		EffectiveOrganizationID: effectiveOrganizationID,
		SelectionRequired:       effectiveOrganizationID == nil && len(organizations) > 1,
		Organizations:           organizations,
	})
}

type contextResponse struct {
	User                    userResponse           `json:"user"`
	HomeOrganizationID      string                 `json:"homeOrganizationId"`
	EffectiveOrganizationID *string                `json:"effectiveOrganizationId"`
	SelectionRequired       bool                   `json:"selectionRequired"`
	Organizations           []organizationResponse `json:"organizations"`
}

type userResponse struct {
	ID string `json:"id"`
}

type organizationResponse struct {
	ID    string   `json:"id"`
	Name  string   `json:"name"`
	Roles []string `json:"roles"`
}

func writeProtocolError(c *gin.Context, status int, code string, message string) {
	c.AbortWithStatusJSON(status, gin.H{
		"code":        code,
		"message":     message,
		"requestId":   strings.TrimSpace(c.GetHeader("X-Request-ID")),
		"fieldErrors": []any{},
	})
}
