package zitadel

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptrace"
	"net/url"
	"strings"
	"time"
)

// SelfServiceOperation is an allowlisted, user-scoped ZITADEL Auth API action.
// It deliberately excludes administrator and management API operations.
type SelfServiceOperation string

const (
	SelfServiceSetProfile              SelfServiceOperation = "profile"
	SelfServiceSetEmail                SelfServiceOperation = "email"
	SelfServiceResendEmailVerification SelfServiceOperation = "email-resend"
	SelfServiceVerifyEmail             SelfServiceOperation = "email-verify"
	SelfServiceSetPhone                SelfServiceOperation = "phone"
	SelfServiceResendPhoneVerification SelfServiceOperation = "phone-resend"
	SelfServiceVerifyPhone             SelfServiceOperation = "phone-verify"
	SelfServiceUpdatePassword          SelfServiceOperation = "password"
)

type selfServiceRoute struct {
	method string
	path   string
}

var selfServiceRoutes = map[SelfServiceOperation]selfServiceRoute{
	SelfServiceSetProfile:              {method: http.MethodPut, path: "/profile"},
	SelfServiceSetEmail:                {method: http.MethodPut, path: "/email"},
	SelfServiceResendEmailVerification: {method: http.MethodPost, path: "/email/_resend_verification"},
	SelfServiceVerifyEmail:             {method: http.MethodPost, path: "/email/_verify"},
	SelfServiceSetPhone:                {method: http.MethodPut, path: "/phone"},
	SelfServiceResendPhoneVerification: {method: http.MethodPost, path: "/phone/_resend_verification"},
	SelfServiceVerifyPhone:             {method: http.MethodPost, path: "/phone/_verify"},
	SelfServiceUpdatePassword:          {method: http.MethodPut, path: "/password"},
}

// SelfServiceClient calls only the official authenticated-user API. The
// caller must provide the already verified user's access token; this client
// never accepts an administrator credential or a target user id.
type SelfServiceClient struct {
	baseURL string
	client  *http.Client
}

// SelfServiceError intentionally keeps only the upstream HTTP status. Bodies
// from an identity provider can contain sensitive or provider-internal data.
type SelfServiceError struct {
	StatusCode int
}

// SelfServiceOutcomeUnknownError means the authenticated-user mutation was
// dispatched, but the client could not establish whether ZITADEL completed
// it. Callers must reconcile readable state before offering a retry.
type SelfServiceOutcomeUnknownError struct{}

func (*SelfServiceOutcomeUnknownError) Error() string {
	return "ZITADEL self-service mutation outcome is unknown"
}

type SelfServiceProfile struct {
	FirstName         string `json:"firstName"`
	LastName          string `json:"lastName"`
	NickName          string `json:"nickName"`
	DisplayName       string `json:"displayName"`
	PreferredLanguage string `json:"preferredLanguage"`
	Gender            string `json:"gender"`
}

func (e *SelfServiceError) Error() string {
	if e == nil {
		return "ZITADEL self-service request failed"
	}
	return "ZITADEL self-service request failed"
}

func NewSelfServiceClient(issuer string, client *http.Client) *SelfServiceClient {
	result := &SelfServiceClient{}
	u, err := url.Parse(strings.TrimRight(strings.TrimSpace(issuer), "/"))
	if err != nil || u.Host == "" || (u.Scheme != "http" && u.Scheme != "https") || u.User != nil || u.RawQuery != "" || u.Fragment != "" {
		return result
	}
	basePath := strings.TrimRight(u.Path, "/") + "/auth/v1/users/me"
	u.Path = basePath
	u.RawPath = ""
	result.baseURL = strings.TrimRight(u.String(), "/")
	bounded := http.Client{Timeout: 5 * time.Second}
	if client != nil {
		bounded = *client
		bounded.Timeout = 5 * time.Second
	}
	bounded.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	result.client = &bounded
	return result
}

func (c *SelfServiceClient) Execute(ctx context.Context, token string, operation SelfServiceOperation, body []byte) error {
	if c == nil || c.baseURL == "" || c.client == nil {
		return errors.New("ZITADEL self-service is not configured")
	}
	if strings.TrimSpace(token) == "" {
		return errors.New("ZITADEL self-service token is required")
	}
	route, ok := selfServiceRoutes[operation]
	if !ok {
		return errors.New("ZITADEL self-service operation is not allowed")
	}
	if len(body) == 0 || len(body) > 16*1024 {
		return errors.New("ZITADEL self-service body is invalid")
	}
	req, err := http.NewRequestWithContext(ctx, route.method, c.baseURL+route.path, strings.NewReader(string(body)))
	if err != nil {
		return errors.New("create ZITADEL self-service request")
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", "application/json")
	dispatched := false
	trace := &httptrace.ClientTrace{WroteRequest: func(httptrace.WroteRequestInfo) { dispatched = true }}
	req = req.WithContext(httptrace.WithClientTrace(req.Context(), trace))
	response, err := c.client.Do(req)
	if err != nil {
		if dispatched {
			return &SelfServiceOutcomeUnknownError{}
		}
		return errors.New("ZITADEL self-service request unavailable")
	}
	defer response.Body.Close()
	_, _ = io.CopyN(io.Discard, response.Body, 1024)
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		return &SelfServiceError{StatusCode: response.StatusCode}
	}
	return nil
}

func (c *SelfServiceClient) ReadProfile(ctx context.Context, token string) (SelfServiceProfile, error) {
	var empty SelfServiceProfile
	if c == nil || c.baseURL == "" || c.client == nil {
		return empty, errors.New("ZITADEL self-service is not configured")
	}
	if strings.TrimSpace(token) == "" {
		return empty, errors.New("ZITADEL self-service token is required")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/profile", nil)
	if err != nil {
		return empty, errors.New("create ZITADEL self-service profile request")
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/json")
	response, err := c.client.Do(req)
	if err != nil {
		return empty, errors.New("ZITADEL self-service profile unavailable")
	}
	defer response.Body.Close()
	body, err := io.ReadAll(io.LimitReader(response.Body, 16*1024+1))
	if err != nil || len(body) > 16*1024 {
		return empty, errors.New("ZITADEL self-service profile response invalid")
	}
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		return empty, &SelfServiceError{StatusCode: response.StatusCode}
	}
	var payload struct {
		Profile SelfServiceProfile `json:"profile"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return empty, errors.New("ZITADEL self-service profile response invalid")
	}
	return payload.Profile, nil
}

type bearerTokenContextKey struct{}

// WithBearerToken keeps the already verified bearer token private to the
// ZITADEL runtime so user-scoped upstream actions can use the same proof.
func WithBearerToken(ctx context.Context, token string) context.Context {
	return context.WithValue(ctx, bearerTokenContextKey{}, strings.TrimSpace(token))
}

func BearerTokenFromContext(ctx context.Context) string {
	if ctx == nil {
		return ""
	}
	token, _ := ctx.Value(bearerTokenContextKey{}).(string)
	return token
}
