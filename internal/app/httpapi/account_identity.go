package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/mail"
	"regexp"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"task-processor/internal/authidentity"
	"task-processor/internal/authruntime/zitadel"
	"task-processor/internal/core/config"
	"task-processor/internal/httproute"
	kernelmodule "task-processor/internal/kernel/module"
)

const (
	accountIdentityBasePath = "/api/v1/account/identity"
	accountIdentityMaxBytes = 16 * 1024
)

const (
	accountIdentityProfilePath     = accountIdentityBasePath + "/profile"
	accountIdentityEmailPath       = accountIdentityBasePath + "/email"
	accountIdentityEmailResendPath = accountIdentityBasePath + "/email/resend"
	accountIdentityEmailVerifyPath = accountIdentityBasePath + "/email/verify"
	accountIdentityPhonePath       = accountIdentityBasePath + "/phone"
	accountIdentityPhoneResendPath = accountIdentityBasePath + "/phone/resend"
	accountIdentityPhoneVerifyPath = accountIdentityBasePath + "/phone/verify"
	accountIdentityPasswordPath    = accountIdentityBasePath + "/password"
)

type accountIdentitySelfService interface {
	Execute(ctx context.Context, token string, operation zitadel.SelfServiceOperation, body []byte) error
	ReadProfile(ctx context.Context, token string) (zitadel.SelfServiceProfile, error)
}

type accountIdentityModule struct {
	client accountIdentitySelfService
}

func (accountIdentityModule) Name() string { return "account-identity" }

func (m accountIdentityModule) Enabled(cfg *config.Config) bool {
	return m.client != nil && cfg != nil && cfg.Workbench.Enabled
}

func (m accountIdentityModule) Register(reg *kernelmodule.Registry) error {
	if m.client == nil {
		return errors.New("account identity self-service unavailable")
	}
	for path, operation := range accountIdentityRoutes {
		method := http.MethodPut
		if isPostIdentityOperation(operation) {
			method = http.MethodPost
		}
		reg.AddRoutes(httproute.Descriptor{Method: method, Path: path, Module: m.Name(), AuthPolicy: httproute.AuthPolicyCurrentIdentity, RequestTimeout: 15 * time.Second, Handler: m.execute})
		if operation == zitadel.SelfServiceSetProfile {
			reg.AddRoutes(httproute.Descriptor{Method: http.MethodGet, Path: path, Module: m.Name(), AuthPolicy: httproute.AuthPolicyCurrentIdentity, RequestTimeout: 15 * time.Second, Handler: m.readProfile})
		}
	}
	return nil
}

var accountIdentityRoutes = map[string]zitadel.SelfServiceOperation{
	accountIdentityProfilePath:     zitadel.SelfServiceSetProfile,
	accountIdentityEmailPath:       zitadel.SelfServiceSetEmail,
	accountIdentityEmailResendPath: zitadel.SelfServiceResendEmailVerification,
	accountIdentityEmailVerifyPath: zitadel.SelfServiceVerifyEmail,
	accountIdentityPhonePath:       zitadel.SelfServiceSetPhone,
	accountIdentityPhoneResendPath: zitadel.SelfServiceResendPhoneVerification,
	accountIdentityPhoneVerifyPath: zitadel.SelfServiceVerifyPhone,
	accountIdentityPasswordPath:    zitadel.SelfServiceUpdatePassword,
}

var accountIdentityPhonePattern = regexp.MustCompile(`^\+[1-9][0-9]{6,14}$`)

func (m accountIdentityModule) execute(c *gin.Context) {
	identity, ok := authidentity.AuthenticatedIdentityFromContext(c.Request.Context())
	if !ok || !authidentity.IsBoundedIdentifier(identity.UserID) {
		writeAccountIdentityError(c, http.StatusUnauthorized, "AUTHENTICATION_REQUIRED")
		return
	}
	operation, ok := accountIdentityRoutes[c.Request.URL.Path]
	if !ok {
		writeAccountIdentityError(c, http.StatusBadRequest, "INVALID_REQUEST")
		return
	}
	body, err := decodeAccountIdentityBody(c.Request, operation)
	if err != nil {
		writeAccountIdentityError(c, http.StatusBadRequest, "INVALID_REQUEST")
		return
	}
	token := zitadel.BearerTokenFromContext(c.Request.Context())
	if token == "" {
		writeAccountIdentityError(c, http.StatusUnauthorized, "AUTHENTICATION_REQUIRED")
		return
	}
	if err := m.client.Execute(c.Request.Context(), token, operation, body); err != nil {
		writeAccountIdentityUpstreamError(c, err, isVerificationOperation(operation))
		return
	}
	c.Header("Cache-Control", "private, no-store")
	c.Header("X-Content-Type-Options", "nosniff")
	c.JSON(http.StatusOK, gin.H{
		"schemaVersion": "account-identity-operation-v1",
		"operation":     string(operation),
		"state":         accountIdentityState(operation),
		"source":        "zitadel_auth_v1",
	})
}

func (m accountIdentityModule) readProfile(c *gin.Context) {
	identity, ok := authidentity.AuthenticatedIdentityFromContext(c.Request.Context())
	if !ok || !authidentity.IsBoundedIdentifier(identity.UserID) {
		writeAccountIdentityError(c, http.StatusUnauthorized, "AUTHENTICATION_REQUIRED")
		return
	}
	token := zitadel.BearerTokenFromContext(c.Request.Context())
	if token == "" {
		writeAccountIdentityError(c, http.StatusUnauthorized, "AUTHENTICATION_REQUIRED")
		return
	}
	profile, err := m.client.ReadProfile(c.Request.Context(), token)
	if err != nil {
		writeAccountIdentityUpstreamError(c, err, false)
		return
	}
	c.Header("Cache-Control", "private, no-store")
	c.Header("X-Content-Type-Options", "nosniff")
	c.JSON(http.StatusOK, gin.H{
		"schemaVersion":     "account-identity-profile-v1",
		"userId":            identity.UserID,
		"firstName":         profile.FirstName,
		"lastName":          profile.LastName,
		"nickName":          profile.NickName,
		"displayName":       profile.DisplayName,
		"preferredLanguage": profile.PreferredLanguage,
		"gender":            profile.Gender,
		"source":            "zitadel_auth_v1",
	})
}

func writeAccountIdentityUpstreamError(c *gin.Context, err error, verificationFailure bool) {
	var unknown *zitadel.SelfServiceOutcomeUnknownError
	if errors.As(err, &unknown) {
		writeAccountIdentityError(c, http.StatusBadGateway, "RESULT_UNVERIFIED")
		return
	}
	var upstream *zitadel.SelfServiceError
	if errors.As(err, &upstream) {
		if upstream.StatusCode == http.StatusUnauthorized {
			writeAccountIdentityError(c, http.StatusUnauthorized, "AUTHENTICATION_REQUIRED")
			return
		}
		if upstream.StatusCode == http.StatusBadRequest || upstream.StatusCode == http.StatusUnprocessableEntity {
			if verificationFailure {
				writeAccountIdentityError(c, http.StatusUnprocessableEntity, "IDENTITY_VERIFICATION_FAILED")
			} else {
				writeAccountIdentityError(c, http.StatusUnprocessableEntity, "IDENTITY_PROVIDER_REJECTED")
			}
			return
		}
	}
	writeAccountIdentityError(c, http.StatusServiceUnavailable, "DEPENDENCY_UNAVAILABLE")
}

func decodeAccountIdentityBody(r *http.Request, operation zitadel.SelfServiceOperation) ([]byte, error) {
	if r.URL.RawQuery != "" || r.URL.ForceQuery || r.ContentLength <= 0 || r.ContentLength > accountIdentityMaxBytes || len(r.TransferEncoding) > 0 {
		return nil, errors.New("invalid identity request")
	}
	decoder := json.NewDecoder(io.LimitReader(r.Body, accountIdentityMaxBytes))
	decoder.DisallowUnknownFields()
	var value any
	if err := decoder.Decode(&value); err != nil {
		return nil, err
	}
	var trailing any
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		return nil, errors.New("trailing identity request body")
	}
	switch operation {
	case zitadel.SelfServiceSetEmail:
		var input struct {
			Email string `json:"email"`
		}
		if err := strictIdentityJSON(value, &input); err != nil || !validEmail(input.Email) {
			return nil, errors.New("invalid email")
		}
		return json.Marshal(input)
	case zitadel.SelfServiceSetPhone:
		var input struct {
			Phone string `json:"phone"`
		}
		if err := strictIdentityJSON(value, &input); err != nil || !accountIdentityPhonePattern.MatchString(input.Phone) {
			return nil, errors.New("invalid phone")
		}
		return json.Marshal(input)
	case zitadel.SelfServiceVerifyEmail, zitadel.SelfServiceVerifyPhone:
		var input struct {
			Code string `json:"code"`
		}
		if err := strictIdentityJSON(value, &input); err != nil || !validIdentityText(input.Code, 64) {
			return nil, errors.New("invalid verification code")
		}
		return json.Marshal(struct {
			VerificationCode string `json:"verificationCode"`
		}{VerificationCode: input.Code})
	case zitadel.SelfServiceResendEmailVerification, zitadel.SelfServiceResendPhoneVerification:
		var input struct{}
		if err := strictIdentityJSON(value, &input); err != nil {
			return nil, err
		}
		return []byte(`{}`), nil
	case zitadel.SelfServiceUpdatePassword:
		var input struct {
			OldPassword string `json:"oldPassword"`
			NewPassword string `json:"newPassword"`
		}
		if err := strictIdentityJSON(value, &input); err != nil || !validIdentityText(input.OldPassword, 512) || !validIdentityText(input.NewPassword, 512) {
			return nil, errors.New("invalid password")
		}
		return json.Marshal(input)
	case zitadel.SelfServiceSetProfile:
		var input struct {
			FirstName         string `json:"firstName"`
			LastName          string `json:"lastName"`
			DisplayName       string `json:"displayName"`
			NickName          string `json:"nickName,omitempty"`
			PreferredLanguage string `json:"preferredLanguage,omitempty"`
			Gender            string `json:"gender,omitempty"`
		}
		if err := strictIdentityJSON(value, &input); err != nil || !validIdentityText(input.FirstName, 200) || !validIdentityText(input.LastName, 200) || !validIdentityText(input.DisplayName, 200) || (input.NickName != "" && !validIdentityText(input.NickName, 200)) || (input.PreferredLanguage != "" && !validIdentityText(input.PreferredLanguage, 32)) || (input.Gender != "" && !map[string]bool{"GENDER_UNSPECIFIED": true, "GENDER_FEMALE": true, "GENDER_MALE": true, "GENDER_DIVERSE": true}[input.Gender]) {
			return nil, errors.New("invalid profile")
		}
		return json.Marshal(input)
	default:
		return nil, errors.New("unsupported identity operation")
	}
}

func strictIdentityJSON(value any, target any) error {
	b, err := json.Marshal(value)
	if err != nil {
		return err
	}
	decoder := json.NewDecoder(strings.NewReader(string(b)))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return err
	}
	var trailing any
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		return errors.New("trailing identity request body")
	}
	return nil
}

func validEmail(value string) bool {
	if !validIdentityText(value, 320) || strings.HasSuffix(strings.ToLower(value), "@phone.invalid") {
		return false
	}
	parsed, err := mail.ParseAddress(value)
	return err == nil && parsed.Address == value
}

func validIdentityText(value string, max int) bool {
	return value != "" && len([]byte(value)) <= max && !strings.ContainsAny(value, "\r\n\x00")
}

func isVerificationOperation(operation zitadel.SelfServiceOperation) bool {
	return operation == zitadel.SelfServiceVerifyEmail || operation == zitadel.SelfServiceVerifyPhone
}

func isPostIdentityOperation(operation zitadel.SelfServiceOperation) bool {
	return operation == zitadel.SelfServiceResendEmailVerification || operation == zitadel.SelfServiceVerifyEmail || operation == zitadel.SelfServiceResendPhoneVerification || operation == zitadel.SelfServiceVerifyPhone
}

func accountIdentityState(operation zitadel.SelfServiceOperation) string {
	switch operation {
	case zitadel.SelfServiceSetEmail, zitadel.SelfServiceSetPhone:
		return "verification_pending"
	case zitadel.SelfServiceResendEmailVerification, zitadel.SelfServiceResendPhoneVerification:
		return "verification_sent"
	case zitadel.SelfServiceVerifyEmail, zitadel.SelfServiceVerifyPhone:
		return "verified"
	default:
		return "updated"
	}
}

func writeAccountIdentityError(c *gin.Context, status int, code string) {
	c.Header("Cache-Control", "private, no-store")
	c.Header("X-Content-Type-Options", "nosniff")
	c.AbortWithStatusJSON(status, gin.H{"code": code, "message": "Account identity request could not be completed", "requestId": "", "fieldErrors": []any{}})
}
