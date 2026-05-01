package client

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"github.com/imroc/req/v3"
)

// LoginRequest 表示 SDS 登录请求。
type LoginRequest struct {
	MerchantName       string `json:"merchant_name"`
	Username           string `json:"username"`
	Password           string `json:"password"`
	DomainName         string `json:"domainName"`
	VerifyCaptchaParam string `json:"verifyCaptchaParam"`
	ExtraInfo          string `json:"extraInfo,omitempty"`
}

// LoginResponse 表示 SDS 登录响应。
type LoginResponse struct {
	Ret  int             `json:"ret"`
	Msg  string          `json:"msg"`
	Data LoginResultData `json:"data"`
}

// LoginResultData 是登录成功后的数据结构。
type LoginResultData struct {
	AccessToken            string              `json:"access_token"`
	IsDesigner             bool                `json:"is_designer"`
	Level                  float64             `json:"level"`
	IsBoss                 bool                `json:"is_boss"`
	IsInsider              bool                `json:"is_insider"`
	PermissionSetMealLevel string              `json:"permission_set_meal_level"`
	ID                     int64               `json:"id"`
	MerchantID             int64               `json:"merchant_id"`
	EnableMFA              int                 `json:"enableMfa"`
	IsZiguang              bool                `json:"is_ziguang"`
	Username               string              `json:"username"`
	VerifyCaptcha          *LoginVerifyCaptcha `json:"verifyCaptcha,omitempty"`
}

// LoginVerifyCaptcha 是 SDS 登录返回的验证码验证结果。
type LoginVerifyCaptcha struct {
	Code      string                   `json:"code"`
	Message   string                   `json:"message"`
	RequestID string                   `json:"requestId"`
	Result    LoginVerifyCaptchaResult `json:"result"`
	Success   bool                     `json:"success"`
}

// LoginVerifyCaptchaResult 表示验证码校验明细。
type LoginVerifyCaptchaResult struct {
	VerifyCode   string `json:"verifyCode"`
	VerifyResult bool   `json:"verifyResult"`
	OTT          string `json:"ott,omitempty"`
	Token        string `json:"token,omitempty"`
}

// SetAuthState 设置鉴权状态并写入公共请求头。
func (c *Client) SetAuthState(state *AuthState) {
	c.authState = state
	c.applyAuthHeaders()
}

// AuthState 返回当前鉴权状态快照。
func (c *Client) AuthState() *AuthState {
	if c.authState == nil {
		return nil
	}

	copied := *c.authState
	return &copied
}

// SaveAuthState 持久化鉴权状态。
func (c *Client) SaveAuthState() error {
	if c.authState == nil {
		return fmt.Errorf("auth state is nil")
	}
	return c.authStore.Save(c.authState)
}

// Login 使用 SDS 登录接口换取 access-token。
func (c *Client) Login(ctx context.Context, req LoginRequest) (*LoginResponse, error) {
	if strings.TrimSpace(req.Username) == "" {
		return nil, fmt.Errorf("username is required")
	}
	if strings.TrimSpace(req.Password) == "" {
		return nil, fmt.Errorf("password is required")
	}
	if strings.TrimSpace(req.DomainName) == "" {
		req.DomainName = "www.sdsdiy.com"
	}

	result := new(LoginResponse)
	resp, err := c.Do(ctx, "POST", c.config.Endpoints.LoginPath, nil, req, result)
	if err != nil {
		return nil, err
	}

	if result.Ret != 0 {
		return nil, &Error{
			Op:      "POST /login",
			Message: result.Msg,
		}
	}

	if strings.TrimSpace(result.Data.AccessToken) == "" {
		if challenge := result.Data.VerifyCaptcha; challenge != nil {
			return result, &CaptchaRequiredError{
				Op:          "POST /login",
				Message:     coalesceNonEmpty(challenge.Message, result.Msg, challenge.Code),
				RequestID:   strings.TrimSpace(challenge.RequestID),
				VerifyCode:  strings.TrimSpace(challenge.Result.VerifyCode),
				VerifyState: challenge.Result.VerifyResult,
			}
		}
		return result, &Error{
			Op:      "POST /login",
			Message: "login response did not include access token",
		}
	}

	state := &AuthState{
		AccessToken: result.Data.AccessToken,
		MerchantID:  result.Data.MerchantID,
		UserID:      result.Data.ID,
		Username:    result.Data.Username,
	}
	c.SetAuthState(state)

	if err := c.SaveAuthState(); err != nil {
		return nil, err
	}
	if resp != nil {
		c.captureResponseCookies(resp)
	}
	if len(c.cookies) > 0 {
		if err := c.SaveCookies(); err != nil {
			return nil, err
		}
	}

	return result, nil
}

func coalesceNonEmpty(values ...string) string {
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed != "" {
			return trimmed
		}
	}
	return ""
}

func (c *Client) applyAuthHeaders() {
	headers := buildDefaultHeaders(c.config)
	if c.authState != nil {
		if c.authState.AccessToken != "" {
			headers["access-token"] = c.authState.AccessToken
		}
		if c.authState.OutToken != "" {
			headers["out-access-token"] = c.authState.OutToken
		}
	}

	c.httpClient.SetCommonHeaders(headers)
}

func (c *Client) captureResponseCookies(resp *req.Response) {
	if resp == nil || resp.Response == nil {
		return
	}
	responseCookies := resp.Cookies()
	if len(responseCookies) == 0 {
		return
	}
	c.SetCookies(mergeCookies(c.cookies, responseCookies))
}

func mergeCookies(existing, incoming []*http.Cookie) []*http.Cookie {
	type cookieKey struct {
		name   string
		domain string
		path   string
	}

	merged := make(map[cookieKey]*http.Cookie, len(existing)+len(incoming))
	order := make([]cookieKey, 0, len(existing)+len(incoming))

	for _, item := range existing {
		if item == nil || item.Name == "" {
			continue
		}
		key := cookieKey{name: item.Name, domain: item.Domain, path: item.Path}
		copied := *item
		merged[key] = &copied
		order = append(order, key)
	}

	for _, item := range incoming {
		if item == nil || item.Name == "" {
			continue
		}
		key := cookieKey{name: item.Name, domain: item.Domain, path: item.Path}
		copied := *item
		if _, exists := merged[key]; !exists {
			order = append(order, key)
		}
		merged[key] = &copied
	}

	result := make([]*http.Cookie, 0, len(merged))
	seen := make(map[cookieKey]struct{}, len(order))
	for _, key := range order {
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		if item, ok := merged[key]; ok {
			result = append(result, item)
		}
	}
	return result
}
