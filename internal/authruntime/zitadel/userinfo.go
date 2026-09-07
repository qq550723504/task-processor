package zitadel

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
	"unicode/utf8"

	"task-processor/internal/authidentity"
)

// UserInfoClient reads the standard self endpoint using only the user's token.
type UserInfoClient struct {
	endpoint string
	client   *http.Client
}

func NewUserInfoClient(issuer string, client *http.Client) *UserInfoClient {
	result := &UserInfoClient{}
	u, err := url.Parse(strings.TrimRight(strings.TrimSpace(issuer), "/"))
	if err != nil || u.Host == "" || (u.Scheme != "http" && u.Scheme != "https") || u.User != nil || u.RawQuery != "" || u.Fragment != "" || u.Path != "" {
		return result
	}
	result.endpoint = u.String() + "/oidc/v1/userinfo"
	bounded := http.Client{Timeout: 5 * time.Second}
	if client != nil {
		bounded = *client
		bounded.Timeout = 5 * time.Second
	}
	bounded.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	result.client = &bounded
	return result
}

func (c *UserInfoClient) ReadSelf(ctx context.Context, token, subject string) (authidentity.SelfProfile, error) {
	empty := authidentity.SelfProfile{}
	if c == nil || c.endpoint == "" || c.client == nil {
		return empty, authidentity.ErrProfileNotConfigured
	}
	if strings.TrimSpace(token) == "" || strings.TrimSpace(subject) == "" {
		return empty, authidentity.ErrProfileAuthentication
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.endpoint, nil)
	if err != nil {
		return empty, authidentity.ErrProfileUnavailable
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/json")
	response, err := c.client.Do(req)
	if err != nil {
		return empty, authidentity.ErrProfileUnavailable
	}
	defer response.Body.Close()
	switch response.StatusCode {
	case http.StatusOK:
	case http.StatusUnauthorized:
		return empty, authidentity.ErrProfileAuthentication
	case http.StatusForbidden:
		return empty, authidentity.ErrProfilePermission
	default:
		return empty, authidentity.ErrProfileUnavailable
	}
	body, err := io.ReadAll(io.LimitReader(response.Body, 16*1024+1))
	if err != nil {
		return empty, authidentity.ErrProfileUnavailable
	}
	if len(body) > 16*1024 || !utf8.Valid(body) {
		return empty, authidentity.ErrProfileInvalid
	}
	var claims struct {
		Subject       string  `json:"sub"`
		Name          *string `json:"name"`
		Email         *string `json:"email"`
		EmailVerified *bool   `json:"email_verified"`
		Phone         *string `json:"phone_number"`
		PhoneVerified *bool   `json:"phone_number_verified"`
	}
	if json.Unmarshal(body, &claims) != nil || claims.Subject != subject {
		return empty, authidentity.ErrProfileInvalid
	}
	for _, field := range []**string{&claims.Name, &claims.Email, &claims.Phone} {
		if *field == nil {
			continue
		}
		value := strings.TrimSpace(**field)
		if len(value) > 512 {
			return empty, authidentity.ErrProfileInvalid
		}
		if value == "" {
			*field = nil
		} else {
			*field = &value
		}
	}
	if claims.Email != nil && strings.HasSuffix(strings.ToLower(*claims.Email), "@phone.invalid") {
		claims.Email = nil
	}
	if claims.Email == nil {
		claims.EmailVerified = nil
	}
	if claims.Phone == nil {
		claims.PhoneVerified = nil
	}
	return authidentity.SelfProfile{UserID: subject, DisplayName: claims.Name, Email: claims.Email, EmailVerified: claims.EmailVerified, PhoneNumber: claims.Phone, PhoneNumberVerified: claims.PhoneVerified}, nil
}
