package client

import (
	"context"
	"fmt"
	"strings"
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
	AccessToken            string  `json:"access_token"`
	IsDesigner             bool    `json:"is_designer"`
	Level                  float64 `json:"level"`
	IsBoss                 bool    `json:"is_boss"`
	IsInsider              bool    `json:"is_insider"`
	PermissionSetMealLevel string  `json:"permission_set_meal_level"`
	ID                     int64   `json:"id"`
	MerchantID             int64   `json:"merchant_id"`
	EnableMFA              int     `json:"enableMfa"`
	IsZiguang              bool    `json:"is_ziguang"`
	Username               string  `json:"username"`
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
	_, err := c.Do(ctx, "POST", c.config.Endpoints.LoginPath, nil, req, result)
	if err != nil {
		return nil, err
	}

	if result.Ret != 0 {
		return nil, &Error{
			Op:      "POST /login",
			Message: result.Msg,
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

	return result, nil
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
