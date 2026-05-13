package sdslogin

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"task-processor/internal/core/config"
	sdsclient "task-processor/internal/sds/client"
)

type loginTriggerKey string

const explicitLoginTriggerKey loginTriggerKey = "sdslogin-explicit-trigger"

type configuredAccount struct {
	TenantID     string
	Identifier   string
	MerchantName string
	Username     string
	Password     string
}

type Service struct {
	loginCfg         config.LoginServiceConfig
	browserCfg       config.BrowserConfig
	authFile         string
	cookieFile       string
	browserStateFile string
	payloadFile      string
	defaultLoginURL  string
	defaultTargetURL string
	authStore        *sdsclient.AuthStateStore
	sessionStore     *sdsclient.SessionStore
	mu               sync.Mutex
	inFlight         bool
	waitingForVerify bool
	lastError        string
	account          configuredAccount
}

var runSDSBrowserLogin = runBrowserLogin

func NewService(loginCfg config.LoginServiceConfig, browserCfg config.BrowserConfig) *Service {
	cfg := sdsclient.DefaultConfig()
	return &Service{
		loginCfg:         loginCfg,
		browserCfg:       browserCfg,
		authFile:         cfg.AuthFile,
		cookieFile:       cfg.CookieFile,
		browserStateFile: filepath.Join(filepath.Dir(cfg.AuthFile), "browser_state.json"),
		payloadFile:      filepath.Join(filepath.Dir(cfg.AuthFile), "login_state.json"),
		defaultLoginURL:  "https://www.sdsdiy.com/user/login?redirect=%2Fadmin%2Fmaterial",
		defaultTargetURL: "https://www.sdsdiy.com/admin/material",
		authStore:        sdsclient.NewAuthStateStore(cfg.AuthFile),
		sessionStore:     sdsclient.NewSessionStore(cfg.CookieFile),
		account: configuredAccount{
			TenantID:     strings.TrimSpace(loginCfg.TenantID),
			Identifier:   strings.TrimSpace(loginCfg.Identifier),
			MerchantName: strings.TrimSpace(loginCfg.MerchantName),
			Username:     strings.TrimSpace(loginCfg.Username),
			Password:     strings.TrimSpace(loginCfg.Password),
		},
	}
}

func (s *Service) Health(context.Context) ServiceHealth {
	return ServiceHealth{
		Initialized:        true,
		MaxConcurrentLogin: max(1, s.loginCfg.MaxConcurrentLogins),
		AdminPageEnabled:   s.loginCfg.AdminPageEnabled,
	}
}

func (s *Service) Status(context.Context) (*Status, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.statusLocked()
}

func (s *Service) statusLocked() (*Status, error) {
	payload, _ := s.loadPayloadLocked()
	status := &Status{
		TenantID:             s.account.TenantID,
		Identifier:           s.account.Identifier,
		MerchantName:         s.account.MerchantName,
		Username:             s.account.Username,
		WaitingForVerifyCode: s.waitingForVerify,
		LoginInProgress:      s.inFlight,
		LastError:            strings.TrimSpace(s.lastError),
	}
	if payload != nil {
		status.Username = coalesce(payload.Username, status.Username)
		status.MerchantName = coalesce(payload.MerchantName, status.MerchantName)
		status.HasCookie = len(payload.Cookies) > 0
		status.HasAccessToken = strings.TrimSpace(payload.AccessToken) != ""
		status.MerchantID = payload.MerchantID
		status.Source = payload.Source
		if !payload.IssuedAt.IsZero() {
			issuedAt := payload.IssuedAt
			status.IssuedAt = &issuedAt
		}
	}
	return status, nil
}

func (s *Service) Login(ctx context.Context, req LoginRequest) (*AuthPayload, error) {
	account := s.account
	return s.loginWithAccount(withExplicitLoginTrigger(ctx), account, req)
}

func (s *Service) ManualLogin(ctx context.Context, req ManualLoginRequest) (*AuthPayload, error) {
	account := configuredAccount{
		TenantID:     strings.TrimSpace(req.TenantID),
		Identifier:   strings.TrimSpace(req.Identifier),
		MerchantName: strings.TrimSpace(req.MerchantName),
		Username:     strings.TrimSpace(req.Username),
		Password:     strings.TrimSpace(req.Password),
	}
	if account.TenantID == "" {
		account.TenantID = s.account.TenantID
	}
	if account.Identifier == "" {
		account.Identifier = s.account.Identifier
	}
	return s.loginWithAccount(withExplicitLoginTrigger(ctx), account, LoginRequest{
		ForceLogin: req.ForceLogin,
		Headless:   req.Headless,
		TargetURL:  req.TargetURL,
	})
}

func (s *Service) loginWithAccount(ctx context.Context, account configuredAccount, req LoginRequest) (*AuthPayload, error) {
	if !hasExplicitLoginTrigger(ctx) {
		return nil, fmt.Errorf("SDS 登录仅允许通过显式登录入口触发")
	}
	if strings.TrimSpace(account.TenantID) == "" || strings.TrimSpace(account.Identifier) == "" {
		return nil, fmt.Errorf("tenant_id and identifier are required")
	}
	if strings.TrimSpace(account.MerchantName) == "" || strings.TrimSpace(account.Username) == "" || strings.TrimSpace(account.Password) == "" {
		return nil, fmt.Errorf("merchant_name, username and password are required")
	}

	s.mu.Lock()
	if s.inFlight {
		s.mu.Unlock()
		return nil, fmt.Errorf("SDS login already in progress")
	}
	s.inFlight = true
	s.lastError = ""
	s.waitingForVerify = false
	s.mu.Unlock()
	defer func() {
		s.mu.Lock()
		s.inFlight = false
		s.mu.Unlock()
	}()

	if !req.ForceLogin {
		if payload, _ := s.loadPayload(); payload != nil && payload.AccessToken != "" && len(payload.Cookies) > 0 {
			return payload, nil
		}
	}

	headless := s.loginCfg.DefaultHeadless
	if req.Headless != nil {
		headless = *req.Headless
	}
	payload, waiting, err := runSDSBrowserLogin(ctx, account, browserRunConfig{
		Headless:          headless,
		ProfileRoot:       s.loginCfg.ProfileRootDir,
		ArtifactDir:       s.loginCfg.ArtifactDir,
		BrowserPath:       s.browserCfg.BrowserPath,
		ChromeVersion:     "144",
		ChromeDownloadDir: "./chrome",
		ViewportWidth:     s.browserCfg.ViewportWidth,
		ViewportHeight:    s.browserCfg.ViewportHeight,
		LoginURL:          s.defaultLoginURL,
		TargetURL:         coalesce(strings.TrimSpace(req.TargetURL), s.defaultTargetURL),
	})
	if err != nil {
		s.mu.Lock()
		s.lastError = err.Error()
		s.waitingForVerify = waiting
		s.mu.Unlock()
		return nil, err
	}
	if err := s.persistPayload(payload); err != nil {
		s.mu.Lock()
		s.lastError = err.Error()
		s.mu.Unlock()
		return nil, err
	}
	s.mu.Lock()
	s.account = account
	s.lastError = ""
	s.waitingForVerify = false
	s.mu.Unlock()
	return payload, nil
}

func (s *Service) loadPayload() (*AuthPayload, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.loadPayloadLocked()
}

func (s *Service) loadPayloadLocked() (*AuthPayload, error) {
	data, err := os.ReadFile(s.payloadFile)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var payload AuthPayload
	if err := json.Unmarshal(data, &payload); err != nil {
		return nil, err
	}
	return &payload, nil
}

func (s *Service) persistPayload(payload *AuthPayload) error {
	if payload == nil {
		return fmt.Errorf("payload is nil")
	}
	if err := os.MkdirAll(filepath.Dir(s.payloadFile), 0o755); err != nil {
		return err
	}
	if err := s.authStore.Save(&sdsclient.AuthState{
		AccessToken: payload.AccessToken,
		OutToken:    payload.OutToken,
		MerchantID:  payload.MerchantID,
		UserID:      payload.UserID,
		Username:    payload.Username,
	}); err != nil {
		return err
	}
	cookies := make([]*http.Cookie, 0, len(payload.Cookies))
	for _, item := range payload.Cookies {
		cookie := &http.Cookie{
			Name:     item.Name,
			Value:    item.Value,
			Domain:   item.Domain,
			Path:     item.Path,
			Expires:  item.Expires,
			Secure:   item.Secure,
			HttpOnly: item.HTTPOnly,
		}
		if cookie.Path == "" {
			cookie.Path = "/"
		}
		cookies = append(cookies, cookie)
	}
	if err := s.sessionStore.Save(cookies); err != nil {
		return err
	}
	if payload.BrowserState != nil {
		body, err := json.MarshalIndent(payload.BrowserState, "", "  ")
		if err != nil {
			return err
		}
		if err := os.WriteFile(s.browserStateFile, body, 0o644); err != nil {
			return err
		}
	}
	body, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(s.payloadFile, body, 0o644)
}

func (s *Service) ClearState() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.waitingForVerify = false
	s.lastError = ""
	removeBestEffort(s.payloadFile)
	removeBestEffort(s.browserStateFile)
	if err := s.authStore.Clear(); err != nil {
		return err
	}
	return s.sessionStore.Clear()
}

func (s *Service) TriggerLogin(ctx context.Context, req sdsclient.LocalLoginRequest) error {
	return nil
}

func (s *Service) LoadAuthState(context.Context, string, string) (*sdsclient.LocalAuthPayload, error) {
	payload, err := s.loadPayload()
	if err != nil || payload == nil {
		return nil, err
	}
	result := &sdsclient.LocalAuthPayload{
		AccessToken: payload.AccessToken,
		OutToken:    payload.OutToken,
		MerchantID:  payload.MerchantID,
		UserID:      payload.UserID,
		Username:    payload.Username,
		Source:      payload.Source,
	}
	for _, item := range payload.Cookies {
		result.Cookies = append(result.Cookies, &sdsclient.PersistedCookie{
			Name:     item.Name,
			Value:    item.Value,
			Domain:   item.Domain,
			Path:     item.Path,
			Expires:  item.Expires,
			Secure:   item.Secure,
			HttpOnly: item.HTTPOnly,
		})
	}
	return result, nil
}

func removeBestEffort(path string) {
	if strings.TrimSpace(path) == "" {
		return
	}
	_ = os.Remove(path)
}

func coalesce(values ...string) string {
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return ""
}

func withExplicitLoginTrigger(ctx context.Context) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	return context.WithValue(ctx, explicitLoginTriggerKey, true)
}

func hasExplicitLoginTrigger(ctx context.Context) bool {
	if ctx == nil {
		return false
	}
	value, _ := ctx.Value(explicitLoginTriggerKey).(bool)
	return value
}
