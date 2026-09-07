package httpapi

import (
	"encoding/json"
	"errors"
	"net/http"
	"regexp"
	"strings"
	"time"

	"task-processor/internal/authidentity"
	"task-processor/internal/workbenchcontext"

	"github.com/gin-gonic/gin"
)

const accountResponseMaxBytes = 16 * 1024

var accountID = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._:-]{0,127}$`)

type AccountProfile struct {
	SchemaVersion       string  `json:"schemaVersion"`
	UserID              string  `json:"userId"`
	HomeOrganizationID  string  `json:"homeOrganizationId"`
	DisplayName         *string `json:"displayName"`
	Email               *string `json:"email"`
	EmailVerified       *bool   `json:"emailVerified"`
	PhoneNumber         *string `json:"phoneNumber"`
	PhoneNumberVerified *bool   `json:"phoneNumberVerified"`
	Source              string  `json:"source"`
	ReadAt              string  `json:"readAt"`
}

type AccountOrganization struct {
	SchemaVersion              string   `json:"schemaVersion"`
	UserID                     string   `json:"userId"`
	HomeOrganizationID         string   `json:"homeOrganizationId"`
	EffectiveOrganizationID    string   `json:"effectiveOrganizationId"`
	Name                       *string  `json:"name"`
	Roles                      []string `json:"roles"`
	Source                     string   `json:"source"`
	ReadAt                     string   `json:"readAt"`
	AuthorizationMaxAgeSeconds int      `json:"authorizationMaxAgeSeconds"`
}

// ResolveAccountOrganizationTarget requires an explicit selector. This prevents
// the resolver's default Home selection from changing the meaning of this GET.
func ResolveAccountOrganizationTarget(r *http.Request) (string, error) {
	if err := validateAccountReadRequest(r); err != nil {
		return "", err
	}
	if len(r.Header.Values("X-Requested-Organization-ID")) != 1 {
		return "", workbenchcontext.ErrOrganizationSelectionRequired
	}
	selected := r.Header.Get("X-Requested-Organization-ID")
	if !accountID.MatchString(selected) {
		return "", workbenchcontext.ErrOrganizationSelectionRequired
	}
	return selected, nil
}

func (h *Handler) GetAccountProfile(c *gin.Context) {
	identity, ok := accountIdentity(c)
	if !ok {
		return
	}
	if h.profileReader == nil {
		accountError(c, 503, "ACCOUNT_NOT_CONFIGURED")
		return
	}
	parts := strings.Fields(c.GetHeader("Authorization"))
	if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
		accountError(c, 401, "AUTHENTICATION_REQUIRED")
		return
	}
	profile, err := h.profileReader.ReadSelf(c.Request.Context(), parts[1], identity.UserID)
	if err != nil {
		switch {
		case errors.Is(err, authidentity.ErrProfileNotConfigured):
			accountError(c, 503, "ACCOUNT_NOT_CONFIGURED")
		case errors.Is(err, authidentity.ErrProfileAuthentication):
			accountError(c, 401, "AUTHENTICATION_REQUIRED")
		case errors.Is(err, authidentity.ErrProfilePermission):
			accountError(c, 403, "PERMISSION_DENIED")
		case errors.Is(err, authidentity.ErrProfileInvalid):
			accountError(c, 502, "INVALID_UPSTREAM_RESPONSE")
		default:
			accountError(c, 503, "DEPENDENCY_UNAVAILABLE")
		}
		return
	}
	if profile.UserID != identity.UserID {
		accountError(c, 502, "INVALID_UPSTREAM_RESPONSE")
		return
	}
	accountJSON(c, AccountProfile{SchemaVersion: "account-v1", UserID: identity.UserID, HomeOrganizationID: identity.HomeOrganizationID, DisplayName: profile.DisplayName, Email: profile.Email, EmailVerified: profile.EmailVerified, PhoneNumber: profile.PhoneNumber, PhoneNumberVerified: profile.PhoneNumberVerified, Source: "zitadel_userinfo", ReadAt: time.Now().UTC().Format(time.RFC3339Nano)})
}

func (h *Handler) GetAccountOrganization(c *gin.Context) {
	identity, ok := accountIdentity(c)
	if !ok {
		return
	}
	if !accountID.MatchString(identity.EffectiveOrganizationID) {
		accountError(c, 409, "ORGANIZATION_SELECTION_REQUIRED")
		return
	}
	for _, grant := range identity.OrganizationGrants {
		if grant.OrganizationID != identity.EffectiveOrganizationID {
			continue
		}
		if len(grant.OrganizationName) > 512 || len(grant.Roles) > 32 {
			accountError(c, 502, "INVALID_UPSTREAM_RESPONSE")
			return
		}
		roles := append([]string{}, grant.Roles...)
		for _, role := range roles {
			if len(role) > 128 {
				accountError(c, 502, "INVALID_UPSTREAM_RESPONSE")
				return
			}
		}
		var name *string
		if grant.OrganizationName != "" {
			name = &grant.OrganizationName
		}
		accountJSON(c, AccountOrganization{SchemaVersion: "account-v1", UserID: identity.UserID, HomeOrganizationID: identity.HomeOrganizationID, EffectiveOrganizationID: identity.EffectiveOrganizationID, Name: name, Roles: roles, Source: "zitadel_project_authorizations", ReadAt: time.Now().UTC().Format(time.RFC3339Nano), AuthorizationMaxAgeSeconds: 60})
		return
	}
	accountError(c, 403, "ORGANIZATION_ACCESS_REVOKED")
}

func accountIdentity(c *gin.Context) (authidentity.AuthenticatedIdentity, bool) {
	identity, ok := authidentity.AuthenticatedIdentityFromContext(c.Request.Context())
	if !ok {
		accountError(c, 401, "AUTHENTICATION_REQUIRED")
		return identity, false
	}
	if err := validateAccountReadRequest(c.Request); err != nil {
		accountError(c, 400, "INVALID_REQUEST")
		return identity, false
	}
	if !accountID.MatchString(identity.UserID) || !accountID.MatchString(identity.HomeOrganizationID) {
		accountError(c, 502, "INVALID_UPSTREAM_RESPONSE")
		return identity, false
	}
	return identity, true
}

func validateAccountReadRequest(r *http.Request) error {
	if r.URL.RawQuery == "" && !r.URL.ForceQuery && r.ContentLength == 0 && len(r.TransferEncoding) == 0 {
		return nil
	}
	if r.Body != nil {
		_ = r.Body.Close()
	}
	return errors.New("account read request must not include a query or body")
}

func accountJSON(c *gin.Context, value any) {
	if c.Request.Context().Err() != nil {
		accountError(c, 504, "DEADLINE_EXCEEDED")
		return
	}
	body, err := json.Marshal(value)
	if err != nil || len(body) > accountResponseMaxBytes {
		accountError(c, 502, "INVALID_UPSTREAM_RESPONSE")
		return
	}
	c.Header("Cache-Control", "private, no-store")
	c.Header("X-Content-Type-Options", "nosniff")
	c.Data(http.StatusOK, "application/json", body)
}
func accountError(c *gin.Context, status int, code string) {
	if c.Request.Context().Err() != nil {
		status = 504
		code = "DEADLINE_EXCEEDED"
	}
	c.Header("Cache-Control", "private, no-store")
	c.Header("X-Content-Type-Options", "nosniff")
	c.AbortWithStatusJSON(status, gin.H{"code": code, "message": "Account request could not be completed", "requestId": "", "fieldErrors": []any{}})
}
