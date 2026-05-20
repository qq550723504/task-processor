package client

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"path"
	"strconv"
	"strings"
	"time"

	"task-processor/internal/core/config"
	"task-processor/internal/core/logger"
	"task-processor/internal/infra/auth"
	"task-processor/internal/infra/clients/management"
	"task-processor/internal/pkg/jsonx"
)

type bootstrapMaterial struct {
	authState *AuthState
	cookies   []*http.Cookie
	username  string
	password  string
	source    string
}

func (c *Client) bootstrapAuth(ctx context.Context, force bool) (bool, error) {
	if c == nil || c.config == nil || !c.config.AuthBootstrap.HasSource() {
		return false, nil
	}
	if !force && c.hasUsableAuthState() {
		return false, nil
	}

	appliedAny := false

	if applied := c.applyStaticBootstrap(); applied {
		appliedAny = true
		if c.hasUsableAuthState() {
			return true, nil
		}
	}

	material, err := c.loadManagementBootstrap()
	if err != nil {
		return appliedAny, err
	}
	if material != nil {
		if c.applyBootstrapMaterial(material) {
			appliedAny = true
		}
	}
	if c.hasUsableAuthState() && (!force || appliedAny) {
		return true, nil
	}

	if force {
		if err := c.triggerLoginServiceLogin(ctx, true); err != nil {
			return appliedAny, err
		}
	}
	loginServiceMaterial, err := c.loadLoginServiceBootstrap(ctx)
	if err != nil {
		return appliedAny, err
	}
	if loginServiceMaterial == nil && !force && c.hasLoginServiceBootstrap() {
		if err := c.triggerLoginServiceLogin(ctx, false); err != nil {
			return appliedAny, err
		}
		loginServiceMaterial, err = c.loadLoginServiceBootstrap(ctx)
		if err != nil {
			return appliedAny, err
		}
	}
	if loginServiceMaterial != nil {
		if c.applyBootstrapMaterial(loginServiceMaterial) {
			appliedAny = true
		}
	}
	if c.hasUsableAuthState() && (!force || appliedAny) {
		return true, nil
	}

	loginReq, ok := c.resolveLoginBootstrap(material)
	if ok {
		if _, err := c.Login(ctx, loginReq); err != nil {
			return appliedAny, fmt.Errorf("bootstrap sds login from %s: %w", loginReq.Username, err)
		}
		return true, nil
	}

	return appliedAny, nil
}

func (c *Client) hasLoginServiceBootstrap() bool {
	if c == nil || c.config == nil {
		return false
	}
	cfg := c.config.AuthBootstrap
	tenantID := strings.TrimSpace(cfg.LoginServiceTenantID)
	identifier := strings.TrimSpace(cfg.LoginServiceIdentifier)
	if tenantID == "" || identifier == "" {
		return false
	}
	if loadLocalLoginProvider() != nil {
		return true
	}
	return strings.TrimSpace(cfg.LoginServiceBaseURL) != ""
}

func (c *Client) triggerLoginServiceLogin(ctx context.Context, force bool) error {
	if !c.hasLoginServiceBootstrap() {
		return nil
	}
	cfg := c.config.AuthBootstrap
	baseURL := strings.TrimSpace(cfg.LoginServiceBaseURL)
	tenantID := strings.TrimSpace(cfg.LoginServiceTenantID)
	identifier := strings.TrimSpace(cfg.LoginServiceIdentifier)
	headless := c.loginServiceHeadless()

	if localProvider := loadLocalLoginProvider(); localProvider != nil {
		req := LocalLoginRequest{
			TenantID:     tenantID,
			Identifier:   identifier,
			MerchantName: strings.TrimSpace(cfg.LoginMerchantName),
			Username:     strings.TrimSpace(cfg.LoginUsername),
			Password:     strings.TrimSpace(cfg.LoginPassword),
			Headless:     headless,
			ForceLogin:   force,
		}
		return localProvider.TriggerLogin(ctx, req)
	}

	requestCtx := ctx
	if requestCtx == nil {
		requestCtx = context.Background()
	}
	requestCtx, cancel := context.WithTimeout(requestCtx, 2*time.Minute)
	defer cancel()

	endpoint := strings.TrimRight(baseURL, "/") + path.Join("/", "api/platforms/sds/login")
	payloadMap := map[string]any{
		"tenant_id":         tenantID,
		"identifier":        identifier,
		"headless":          headless,
		"force_login":       force,
		"keep_browser_open": false,
	}
	if c.hasLoginServiceManualCredentials() {
		endpoint = strings.TrimRight(baseURL, "/") + path.Join("/", "api/platforms/sds/manual-login")
		payloadMap["merchant_name"] = strings.TrimSpace(cfg.LoginMerchantName)
		payloadMap["username"] = strings.TrimSpace(cfg.LoginUsername)
		payloadMap["password"] = strings.TrimSpace(cfg.LoginPassword)
	}
	payload, err := json.Marshal(payloadMap)
	if err != nil {
		return fmt.Errorf("marshal login service login request: %w", err)
	}
	req, err := http.NewRequestWithContext(requestCtx, http.MethodPost, endpoint, bytes.NewReader(payload))
	if err != nil {
		return fmt.Errorf("build login service login request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	if sharedKey := strings.TrimSpace(cfg.LoginServiceSharedKey); sharedKey != "" {
		req.Header.Set("X-Login-Shared-Key", sharedKey)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("call SDS login service auth refresh: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("read SDS login service login response: %w", err)
	}
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return fmt.Errorf("login service login status=%d body=%s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	var payloadResp struct {
		Success bool   `json:"success"`
		Message string `json:"message"`
	}
	if err := jsonx.UnmarshalBytes(body, &payloadResp, "parse login service force login"); err != nil {
		return err
	}
	if !payloadResp.Success {
		return fmt.Errorf("login service login failed: %s", strings.TrimSpace(payloadResp.Message))
	}

	return nil
}

func (c *Client) loginServiceHeadless() bool {
	if c == nil || c.config == nil {
		return true
	}
	return c.config.LoginService.DefaultHeadless
}

func (c *Client) hasLoginServiceManualCredentials() bool {
	if c == nil || c.config == nil {
		return false
	}
	cfg := c.config.AuthBootstrap
	return strings.TrimSpace(cfg.LoginMerchantName) != "" &&
		strings.TrimSpace(cfg.LoginUsername) != "" &&
		strings.TrimSpace(cfg.LoginPassword) != ""
}

func (c *Client) hasUsableAuthState() bool {
	if c.authState == nil || strings.TrimSpace(c.authState.AccessToken) == "" {
		return false
	}
	return c.authState.MerchantID > 0 || c.authState.UserID > 0 || len(c.cookies) > 0
}

func (c *Client) applyStaticBootstrap() bool {
	cfg := c.config.AuthBootstrap
	applied := false

	if strings.TrimSpace(cfg.StaticCookie) != "" {
		cookies := ParseFlexibleCookieInput(cfg.StaticCookie, ".sdsdiy.com")
		if len(cookies) > 0 {
			c.SetCookies(cookies)
			if err := c.SaveCookies(); err != nil {
				c.logger.WithError(err).Warn("persist static SDS cookies failed")
			}
			applied = true
		}
	}

	if strings.TrimSpace(cfg.StaticAccessToken) != "" {
		state := &AuthState{
			AccessToken: cfg.StaticAccessToken,
			OutToken:    cfg.StaticOutToken,
			MerchantID:  cfg.StaticMerchantID,
		}
		c.SetAuthState(state)
		if err := c.SaveAuthState(); err != nil {
			c.logger.WithError(err).Warn("persist static SDS auth state failed")
		}
		applied = true
	}

	return applied
}

func (c *Client) resolveLoginBootstrap(material *bootstrapMaterial) (LoginRequest, bool) {
	cfg := c.config.AuthBootstrap

	username := strings.TrimSpace(cfg.LoginUsername)
	password := strings.TrimSpace(cfg.LoginPassword)
	if username == "" && material != nil {
		username = strings.TrimSpace(material.username)
	}
	if password == "" && material != nil {
		password = strings.TrimSpace(material.password)
	}
	if username == "" || password == "" {
		return LoginRequest{}, false
	}

	domain := strings.TrimSpace(cfg.LoginDomainName)
	if domain == "" {
		domain = "www.sdsdiy.com"
	}

	return LoginRequest{
		MerchantName:       strings.TrimSpace(cfg.LoginMerchantName),
		Username:           username,
		Password:           password,
		DomainName:         domain,
		VerifyCaptchaParam: strings.TrimSpace(cfg.LoginVerifyCaptchaParam),
		ExtraInfo:          strings.TrimSpace(cfg.LoginExtraInfo),
	}, true
}

func (c *Client) applyBootstrapMaterial(material *bootstrapMaterial) bool {
	if material == nil {
		return false
	}
	applied := false
	if len(material.cookies) > 0 {
		c.SetCookies(material.cookies)
		if err := c.SaveCookies(); err != nil {
			c.logger.WithError(err).Warn("persist management SDS cookies failed")
		}
		applied = true
	}
	if material.authState != nil && strings.TrimSpace(material.authState.AccessToken) != "" {
		c.SetAuthState(material.authState)
		if err := c.SaveAuthState(); err != nil {
			c.logger.WithError(err).Warn("persist management SDS auth state failed")
		}
		applied = true
	}
	return applied
}

func (c *Client) loadManagementBootstrap() (*bootstrapMaterial, error) {
	storeID := c.config.AuthBootstrap.ManagementStoreID
	if storeID <= 0 {
		return nil, nil
	}

	mgr, err := newManagementClientFromConfig(c.config.Management)
	if err != nil {
		return nil, err
	}
	storeClient := mgr.GetStoreClient()
	store, err := storeClient.GetStore(storeID)
	if err != nil {
		return nil, fmt.Errorf("load SDS management store %d: %w", storeID, err)
	}

	material := &bootstrapMaterial{
		username: strings.TrimSpace(store.Username),
		password: strings.TrimSpace(store.Password),
		source:   "management-store",
	}

	cookieStr, err := storeClient.GetStoreCookie(storeID)
	if err != nil {
		return nil, fmt.Errorf("load SDS store cookie %d: %w", storeID, err)
	}
	if strings.TrimSpace(cookieStr) != "" {
		material.cookies = ParseFlexibleCookieInput(cookieStr, ".sdsdiy.com")
	}

	return material, nil
}

type loginServiceBootstrapResponse struct {
	Success bool `json:"success"`
	Data    struct {
		AccessToken string `json:"access_token"`
		OutToken    string `json:"out_token"`
		MerchantID  int64  `json:"merchant_id"`
		UserID      int64  `json:"user_id"`
		Username    string `json:"username"`
		Cookies     []struct {
			Name     string `json:"name"`
			Value    string `json:"value"`
			Domain   string `json:"domain"`
			Path     string `json:"path"`
			Expires  any    `json:"expires"`
			Secure   bool   `json:"secure"`
			HTTPOnly bool   `json:"httpOnly"`
		} `json:"cookies"`
		Source string `json:"source"`
	} `json:"data"`
	Message string `json:"message"`
}

func (c *Client) loadLoginServiceBootstrap(ctx context.Context) (*bootstrapMaterial, error) {
	cfg := c.config.AuthBootstrap
	baseURL := strings.TrimSpace(cfg.LoginServiceBaseURL)
	tenantID := strings.TrimSpace(cfg.LoginServiceTenantID)
	identifier := strings.TrimSpace(cfg.LoginServiceIdentifier)
	if tenantID == "" || identifier == "" {
		return nil, nil
	}
	if localProvider := loadLocalLoginProvider(); localProvider != nil {
		payload, err := localProvider.LoadAuthState(ctx, tenantID, identifier)
		if err != nil {
			return nil, fmt.Errorf("load local SDS auth state: %w", err)
		}
		if payload == nil || strings.TrimSpace(payload.AccessToken) == "" {
			return nil, nil
		}
		material := &bootstrapMaterial{
			authState: &AuthState{
				AccessToken: payload.AccessToken,
				OutToken:    payload.OutToken,
				MerchantID:  payload.MerchantID,
				UserID:      payload.UserID,
				Username:    payload.Username,
			},
			source: payload.Source,
		}
		for _, item := range payload.Cookies {
			if item == nil || strings.TrimSpace(item.Name) == "" {
				continue
			}
			material.cookies = append(material.cookies, item.toHTTPCookie())
		}
		return material, nil
	}
	if baseURL == "" {
		return nil, nil
	}

	requestCtx := ctx
	if requestCtx == nil {
		requestCtx = context.Background()
	}
	requestCtx, cancel := context.WithTimeout(requestCtx, 15*time.Second)
	defer cancel()

	endpoint := strings.TrimRight(baseURL, "/")
	endpoint = endpoint + path.Join("/", "api/platforms/sds/auth-state", tenantID, identifier)
	req, err := http.NewRequestWithContext(requestCtx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("build login service bootstrap request: %w", err)
	}
	if sharedKey := strings.TrimSpace(cfg.LoginServiceSharedKey); sharedKey != "" {
		req.Header.Set("X-Login-Shared-Key", sharedKey)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetch SDS login service auth state: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read SDS login service auth state: %w", err)
	}
	if resp.StatusCode == http.StatusNotFound || resp.StatusCode == http.StatusBadRequest {
		return nil, nil
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("login service auth state status=%d body=%s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	var payload loginServiceBootstrapResponse
	if err := jsonx.UnmarshalBytes(body, &payload, "parse login service bootstrap"); err != nil {
		return nil, err
	}
	if !payload.Success || strings.TrimSpace(payload.Data.AccessToken) == "" {
		return nil, nil
	}

	material := &bootstrapMaterial{
		authState: &AuthState{
			AccessToken: payload.Data.AccessToken,
			OutToken:    payload.Data.OutToken,
			MerchantID:  payload.Data.MerchantID,
			UserID:      payload.Data.UserID,
			Username:    payload.Data.Username,
		},
		source: "login-service",
	}
	for _, item := range payload.Data.Cookies {
		if strings.TrimSpace(item.Name) == "" {
			continue
		}
		cookie := &http.Cookie{
			Name:     item.Name,
			Value:    item.Value,
			Domain:   item.Domain,
			Path:     item.Path,
			Secure:   item.Secure,
			HttpOnly: item.HTTPOnly,
		}
		if cookie.Path == "" {
			cookie.Path = "/"
		}
		if parsed := parseLoginServiceCookieExpires(item.Expires); !parsed.IsZero() {
			cookie.Expires = parsed
		}
		material.cookies = append(material.cookies, cookie)
	}

	return material, nil
}

func newManagementClientFromConfig(cfg *config.ManagementConfig) (*management.ClientManager, error) {
	if cfg == nil {
		return nil, fmt.Errorf("management config is nil")
	}

	baseURL := strings.TrimSpace(cfg.BaseURL)
	clientID := strings.TrimSpace(cfg.ClientID)
	clientSecret := strings.TrimSpace(cfg.ClientSecret)
	tenantID := strings.TrimSpace(cfg.TenantID)
	if tenantID == "" {
		tenantID = "1"
	}

	if baseURL == "" || clientID == "" || clientSecret == "" {
		return nil, fmt.Errorf("management config is incomplete")
	}

	log := logger.GetGlobalLogger("sds/client/bootstrap")
	authClient := auth.NewClientCredentialsAuthClient(baseURL, clientID, clientSecret, tenantID, log.Logger)
	accessToken, err := authClient.GetAccessToken()
	if err != nil {
		return nil, fmt.Errorf("get management access token: %w", err)
	}

	mgr := management.NewClientManager(&config.ManagementConfig{BaseURL: baseURL})
	baseClient := mgr.GetClient()
	baseClient.SetUserToken(accessToken, tenantID)
	return mgr, nil
}

func parseLoginServiceCookieExpires(value any) time.Time {
	switch typed := value.(type) {
	case string:
		trimmed := strings.TrimSpace(typed)
		if trimmed == "" {
			return time.Time{}
		}
		if parsed, err := time.Parse(time.RFC3339, trimmed); err == nil {
			return parsed
		}
		if seconds, err := strconv.ParseFloat(trimmed, 64); err == nil && seconds > 0 {
			return time.Unix(int64(seconds), 0).UTC()
		}
	case float64:
		if typed > 0 {
			return time.Unix(int64(typed), 0).UTC()
		}
	case int64:
		if typed > 0 {
			return time.Unix(typed, 0).UTC()
		}
	case int:
		if typed > 0 {
			return time.Unix(int64(typed), 0).UTC()
		}
	}
	return time.Time{}
}

func (c *Client) shouldBootstrapOnInit() bool {
	return c.config != nil && c.config.AuthBootstrap.HasSource() && !c.hasUsableAuthState()
}

func (c *Client) logBootstrapFailure(err error) {
	if err == nil {
		return
	}
	entry := c.logger
	if entry == nil {
		entry = logger.GetGlobalLogger("sds/client").WithField("baseURL", c.config.BaseURL)
	}
	entry.WithError(err).Warn("bootstrap SDS auth state failed")
}

func ParseFlexibleCookieInput(raw, defaultDomain string) []*http.Cookie {
	if strings.TrimSpace(raw) == "" {
		return nil
	}

	var persisted []PersistedCookie
	if err := jsonx.UnmarshalString(raw, &persisted, "parse persisted cookies"); err == nil && len(persisted) > 0 {
		cookies := make([]*http.Cookie, 0, len(persisted))
		for _, item := range persisted {
			cookies = append(cookies, item.toHTTPCookie())
		}
		return cookies
	}

	return ParseCookieHeader(raw, defaultDomain)
}
