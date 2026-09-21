package httpapi

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"task-processor/internal/authidentity"
	"task-processor/internal/core/config"
	"task-processor/internal/httproute"
	profileStore "task-processor/internal/integration/persistence/accountprofile"
	kernelmodule "task-processor/internal/kernel/module"
)

const accountBusinessProfilePath = "/api/v1/account/business-profile"
const accountBusinessProfileMaxBytes = 16 * 1024

type accountProfileModule struct{ repository *profileStore.Repository }

func (accountProfileModule) Name() string { return "account-profile" }
func (m accountProfileModule) Enabled(cfg *config.Config) bool {
	return m.repository != nil && cfg != nil && cfg.Workbench.Enabled
}
func (m accountProfileModule) Register(reg *kernelmodule.Registry) error {
	if m.repository == nil {
		return errors.New("account profile repository unavailable")
	}
	reg.AddRoutes(
		httproute.Descriptor{Method: http.MethodGet, Path: accountBusinessProfilePath, Module: m.Name(), AuthPolicy: httproute.AuthPolicyCurrentIdentity, OrganizationAccessPolicy: httproute.OrganizationAccessPolicyNone, RequestTimeout: 15 * time.Second, RejectUnreadRequestBody: true, Handler: m.read},
		httproute.Descriptor{Method: http.MethodPut, Path: accountBusinessProfilePath, Module: m.Name(), AuthPolicy: httproute.AuthPolicyCurrentIdentity, OrganizationAccessPolicy: httproute.OrganizationAccessPolicyNone, RequestTimeout: 15 * time.Second, Handler: m.update},
	)
	return nil
}

func (m accountProfileModule) read(c *gin.Context) {
	identity, ok := accountProfileIdentity(c)
	if !ok || !accountProfileReadRequest(c.Request) {
		if ok {
			writeAccountProfileError(c, http.StatusBadRequest, "INVALID_REQUEST")
		}
		return
	}
	profile, err := m.repository.Read(c.Request.Context(), identity.UserID)
	if err != nil {
		writeAccountProfileError(c, http.StatusServiceUnavailable, "DEPENDENCY_UNAVAILABLE")
		return
	}
	writeAccountProfileJSON(c, http.StatusOK, profile)
}

func (m accountProfileModule) update(c *gin.Context) {
	identity, ok := accountProfileIdentity(c)
	if !ok {
		return
	}
	input, err := decodeBusinessProfile(c.Request)
	if err != nil {
		writeAccountProfileError(c, http.StatusBadRequest, "INVALID_REQUEST")
		return
	}
	profile := profileStore.BusinessProfile{UserID: identity.UserID, UserRole: input.UserRole, ShopSituation: input.ShopSituation, FactorySituation: input.FactorySituation, Platforms: input.Platforms, Sites: input.Sites, ShopType: input.ShopType, Services: input.Services}
	saved, err := m.repository.SaveWithAudit(c.Request.Context(), profile, profileStore.AuditContext{OrganizationID: identity.EffectiveOrganizationID, ActorID: identity.UserID})
	if err != nil {
		writeAccountProfileError(c, http.StatusServiceUnavailable, "DEPENDENCY_UNAVAILABLE")
		return
	}
	writeAccountProfileJSON(c, http.StatusOK, saved)
}

type businessProfileInput struct {
	UserRole         string   `json:"userRole"`
	ShopSituation    string   `json:"shopSituation"`
	FactorySituation string   `json:"factorySituation"`
	Platforms        []string `json:"platforms"`
	Sites            []string `json:"sites"`
	ShopType         string   `json:"shopType"`
	Services         []string `json:"services"`
}

func decodeBusinessProfile(r *http.Request) (businessProfileInput, error) {
	if r.URL.RawQuery != "" || r.URL.ForceQuery || r.ContentLength == 0 || r.ContentLength > accountBusinessProfileMaxBytes || len(r.TransferEncoding) > 0 {
		return businessProfileInput{}, errors.New("invalid account profile request")
	}
	decoder := json.NewDecoder(io.LimitReader(r.Body, accountBusinessProfileMaxBytes))
	decoder.DisallowUnknownFields()
	var input businessProfileInput
	if err := decoder.Decode(&input); err != nil {
		return businessProfileInput{}, err
	}
	var extra any
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		return businessProfileInput{}, errors.New("trailing account profile body")
	}
	if err := validateBusinessProfileInput(&input); err != nil {
		return businessProfileInput{}, err
	}
	return input, nil
}

func validateBusinessProfileInput(input *businessProfileInput) error {
	for name, value := range map[string]string{"userRole": input.UserRole, "shopSituation": input.ShopSituation, "factorySituation": input.FactorySituation, "shopType": input.ShopType} {
		if strings.ContainsAny(value, "\r\n\x00") || len([]byte(value)) > 128 {
			return errors.New("invalid " + name)
		}
		inputValue := strings.TrimSpace(value)
		switch name {
		case "userRole":
			input.UserRole = inputValue
		case "shopSituation":
			input.ShopSituation = inputValue
		case "factorySituation":
			input.FactorySituation = inputValue
		case "shopType":
			input.ShopType = inputValue
		}
	}
	for name, values := range map[string][]string{"platforms": input.Platforms, "sites": input.Sites, "services": input.Services} {
		if len(values) > 16 {
			return errors.New("too many " + name)
		}
		for i, value := range values {
			value = strings.TrimSpace(value)
			if value == "" || strings.ContainsAny(value, "\r\n\x00") || len([]byte(value)) > 64 {
				return errors.New("invalid " + name)
			}
			values[i] = value
		}
		switch name {
		case "platforms":
			input.Platforms = values
		case "sites":
			input.Sites = values
		case "services":
			input.Services = values
		}
	}
	return nil
}

func accountProfileIdentity(c *gin.Context) (authidentity.AuthenticatedIdentity, bool) {
	identity, ok := authidentity.AuthenticatedIdentityFromContext(c.Request.Context())
	if !ok || !authidentity.IsBoundedIdentifier(identity.UserID) {
		writeAccountProfileError(c, http.StatusUnauthorized, "AUTHENTICATION_REQUIRED")
		return identity, false
	}
	return identity, true
}

func accountProfileReadRequest(r *http.Request) bool {
	return r.URL.RawQuery == "" && !r.URL.ForceQuery && r.ContentLength == 0 && len(r.TransferEncoding) == 0
}

func writeAccountProfileJSON(c *gin.Context, status int, profile profileStore.BusinessProfile) {
	var updatedAt *string
	if profile.UpdatedAt != nil {
		value := profile.UpdatedAt.UTC().Format(time.RFC3339Nano)
		updatedAt = &value
	}
	c.Header("Cache-Control", "private, no-store")
	c.Header("X-Content-Type-Options", "nosniff")
	c.JSON(status, gin.H{"schemaVersion": "account-business-profile-v1", "userId": profile.UserID, "userRole": nullableText(profile.UserRole), "shopSituation": nullableText(profile.ShopSituation), "factorySituation": nullableText(profile.FactorySituation), "platforms": profile.Platforms, "sites": profile.Sites, "shopType": nullableText(profile.ShopType), "services": profile.Services, "source": "account_profile", "updatedAt": updatedAt, "readAt": time.Now().UTC().Format(time.RFC3339Nano)})
}

func nullableText(value string) *string {
	if value == "" {
		return nil
	}
	return &value
}
func writeAccountProfileError(c *gin.Context, status int, code string) {
	c.Header("Cache-Control", "private, no-store")
	c.Header("X-Content-Type-Options", "nosniff")
	c.AbortWithStatusJSON(status, gin.H{"code": code, "message": "Account profile request could not be completed", "requestId": "", "fieldErrors": []any{}})
}
