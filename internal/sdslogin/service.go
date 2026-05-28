package sdslogin

import (
	"context"
	"encoding/json"
	"fmt"
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
	redisCfg         config.RedisConfig
	authFile         string
	cookieFile       string
	browserStateFile string
	payloadFile      string
	defaultLoginURL  string
	defaultTargetURL string
	authStore        *sdsclient.AuthStateStore
	sessionStore     *sdsclient.SessionStore
	redisStore       *RedisStateStore
	mu               sync.Mutex
	inFlight         bool
	waitingForVerify bool
	lastError        string
	account          configuredAccount
}

var runSDSBrowserLogin = runBrowserLogin

func NewService(loginCfg config.LoginServiceConfig, redisCfg config.RedisConfig, browserCfg config.BrowserConfig) (*Service, error) {
	cfg := sdsclient.DefaultConfig()
	redisStore, err := NewRedisStateStore(redisCfg)
	if err != nil {
		return nil, fmt.Errorf("initialize SDS redis state store: %w", err)
	}
	service := &Service{
		loginCfg:         loginCfg,
		browserCfg:       browserCfg,
		redisCfg:         redisCfg,
		authFile:         cfg.AuthFile,
		cookieFile:       cfg.CookieFile,
		browserStateFile: filepath.Join(filepath.Dir(cfg.AuthFile), "browser_state.json"),
		payloadFile:      filepath.Join(filepath.Dir(cfg.AuthFile), "login_state.json"),
		defaultLoginURL:  "https://www.sdsdiy.com/user/login?redirect=%2Fadmin%2Fmaterial",
		defaultTargetURL: "https://www.sdsdiy.com/admin/material",
		authStore:        sdsclient.NewAuthStateStore(cfg.AuthFile),
		sessionStore:     sdsclient.NewSessionStore(cfg.CookieFile),
		redisStore:       redisStore,
		account: configuredAccount{
			TenantID:     strings.TrimSpace(loginCfg.TenantID),
			Identifier:   strings.TrimSpace(loginCfg.Identifier),
			MerchantName: strings.TrimSpace(loginCfg.MerchantName),
			Username:     strings.TrimSpace(loginCfg.Username),
			Password:     strings.TrimSpace(loginCfg.Password),
		},
	}
	service.applyAccountStatePaths(service.account)
	return service, nil
}

func (s *Service) Health(context.Context) ServiceHealth {
	return ServiceHealth{
		Initialized:        true,
		MaxConcurrentLogin: max(1, s.loginCfg.MaxConcurrentLogins),
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
		status.TenantID = coalesce(payload.TenantID, status.TenantID)
		status.Identifier = coalesce(payload.Identifier, status.Identifier)
		status.Username = coalesce(payload.Username, status.Username)
		status.MerchantName = coalesce(payload.MerchantName, status.MerchantName)
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

func hasUsablePayload(payload *AuthPayload) bool {
	if payload == nil || strings.TrimSpace(payload.AccessToken) == "" {
		return false
	}
	return payload.MerchantID > 0 || payload.UserID > 0
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
		if payload, _ := s.loadPayload(); hasUsablePayload(payload) {
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
		BrowserPath:       coalesce(strings.TrimSpace(s.loginCfg.CloakBrowserPath), strings.TrimSpace(os.Getenv("CLOAKBROWSER_BINARY_PATH")), s.browserCfg.BrowserPath),
		ChromeVersion:     "144",
		ChromeDownloadDir: "./chrome",
		ViewportWidth:     s.browserCfg.ViewportWidth,
		ViewportHeight:    s.browserCfg.ViewportHeight,
		LoginURL:          s.defaultLoginURL,
		TargetURL:         coalesce(strings.TrimSpace(req.TargetURL), s.defaultTargetURL),
		UseCloakBrowser:   s.loginCfg.CloakBrowserEnabled,
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
	s.applyAccountStatePaths(account)
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
	if s.redisStore != nil {
		payload, err := s.redisStore.Load(context.Background())
		if err != nil {
			return nil, err
		}
		if payload != nil {
			return payload, nil
		}
	}
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
	account := configuredAccount{
		TenantID:     coalesce(payload.TenantID, s.account.TenantID),
		Identifier:   coalesce(payload.Identifier, payload.ShopID, s.account.Identifier),
		MerchantName: coalesce(payload.MerchantName, s.account.MerchantName),
		Username:     coalesce(payload.Username, s.account.Username),
		Password:     s.account.Password,
	}
	authStore, sessionStore, _, _, browserStateFile, payloadFile := s.accountState(account)
	payload.TenantID = account.TenantID
	payload.Identifier = coalesce(payload.Identifier, account.Identifier)
	payload.ShopID = coalesce(payload.ShopID, account.Identifier)
	if s.redisStore != nil {
		if err := s.redisStore.Save(context.Background(), payload); err != nil {
			return err
		}
	}
	if err := os.MkdirAll(filepath.Dir(payloadFile), 0o755); err != nil {
		return err
	}
	if s.redisStore == nil {
		if err := authStore.Save(&sdsclient.AuthState{
			AccessToken: payload.AccessToken,
			OutToken:    payload.OutToken,
			MerchantID:  payload.MerchantID,
			UserID:      payload.UserID,
			Username:    payload.Username,
		}); err != nil {
			return err
		}
		if err := sessionStore.Clear(); err != nil {
			return err
		}
		if payload.BrowserState != nil {
			body, err := json.MarshalIndent(payload.BrowserState, "", "  ")
			if err != nil {
				return err
			}
			if err := os.WriteFile(browserStateFile, body, 0o644); err != nil {
				return err
			}
		}
		body, err := json.MarshalIndent(payload, "", "  ")
		if err != nil {
			return err
		}
		if err := os.WriteFile(payloadFile, body, 0o644); err != nil {
			return err
		}
	}
	s.mu.Lock()
	s.account = account
	s.applyAccountStatePaths(account)
	s.mu.Unlock()
	return nil
}

func (s *Service) ClearState() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.waitingForVerify = false
	s.lastError = ""
	authStore, sessionStore, _, _, browserStateFile, payloadFile := s.accountState(s.account)
	removeBestEffort(payloadFile)
	removeBestEffort(browserStateFile)
	if payloadFile != s.legacyPayloadFilePath() {
		removeBestEffort(s.legacyPayloadFilePath())
	}
	if browserStateFile != s.legacyBrowserStateFilePath() {
		removeBestEffort(s.legacyBrowserStateFilePath())
	}
	if s.redisStore != nil {
		if err := s.redisStore.Clear(context.Background()); err != nil {
			return err
		}
	}
	if s.redisStore == nil {
		if err := authStore.Clear(); err != nil {
			return err
		}
		if err := sessionStore.Clear(); err != nil {
			return err
		}
	}
	return nil
}

func (s *Service) TriggerLogin(ctx context.Context, req sdsclient.LocalLoginRequest) error {
	account := configuredAccount{
		TenantID:     coalesce(req.TenantID, s.account.TenantID),
		Identifier:   coalesce(req.Identifier, s.account.Identifier),
		MerchantName: coalesce(req.MerchantName, s.account.MerchantName),
		Username:     coalesce(req.Username, s.account.Username),
		Password:     coalesce(req.Password, s.account.Password),
	}
	if account.TenantID == "" || account.Identifier == "" {
		return fmt.Errorf("tenant_id and identifier are required")
	}
	if account.MerchantName == "" || account.Username == "" || account.Password == "" {
		return fmt.Errorf("merchant_name, username and password are required")
	}
	headless := s.loginCfg.DefaultHeadless
	if req.Headless {
		headless = true
	}
	_, err := s.loginWithAccount(withExplicitLoginTrigger(ctx), account, LoginRequest{
		ForceLogin: req.ForceLogin,
		Headless:   &headless,
	})
	return err
}

func (s *Service) LoadAuthState(_ context.Context, tenantID, identifier string) (*sdsclient.LocalAuthPayload, error) {
	account := configuredAccount{
		TenantID:   coalesce(s.account.TenantID, tenantID),
		Identifier: coalesce(s.account.Identifier, identifier),
	}
	payload, err := s.loadPayloadForAccount(account)
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
	return result, nil
}

func (s *Service) loadPayloadForAccount(account configuredAccount) (*AuthPayload, error) {
	if s.redisStore != nil {
		payload, err := s.redisStore.Load(context.Background())
		if err != nil {
			return nil, err
		}
		if payload != nil {
			return payload, nil
		}
	}
	_, _, _, _, _, payloadFile := s.accountState(account)
	data, err := os.ReadFile(payloadFile)
	if err != nil {
		if os.IsNotExist(err) && payloadFile != s.legacyPayloadFilePath() {
			data, err = os.ReadFile(s.legacyPayloadFilePath())
		}
		if err != nil {
			if os.IsNotExist(err) {
				return nil, nil
			}
			return nil, err
		}
	}
	var payload AuthPayload
	if err := json.Unmarshal(data, &payload); err != nil {
		return nil, err
	}
	return &payload, nil
}

func (s *Service) accountState(account configuredAccount) (*sdsclient.AuthStateStore, *sdsclient.SessionStore, string, string, string, string) {
	authFile, cookieFile, browserStateFile, payloadFile := s.accountStatePaths(account)
	return sdsclient.NewAuthStateStore(authFile), sdsclient.NewSessionStore(cookieFile), authFile, cookieFile, browserStateFile, payloadFile
}

func (s *Service) applyAccountStatePaths(account configuredAccount) {
	authFile, cookieFile, browserStateFile, payloadFile := s.accountStatePaths(account)
	s.authFile = authFile
	s.cookieFile = cookieFile
	s.browserStateFile = browserStateFile
	s.payloadFile = payloadFile
	s.authStore = sdsclient.NewAuthStateStore(authFile)
	s.sessionStore = sdsclient.NewSessionStore(cookieFile)
}

func (s *Service) accountStatePaths(_ configuredAccount) (string, string, string, string) {
	// SDS auth is shared globally across ListingKit tenants/users.
	// Keep a single persisted state so every request reuses the same account.
	return s.legacyAuthFilePath(), s.legacyCookieFilePath(), s.legacyBrowserStateFilePath(), s.legacyPayloadFilePath()
}

func (s *Service) legacyAuthFilePath() string {
	cfg := sdsclient.DefaultConfig()
	return cfg.AuthFile
}

func (s *Service) legacyCookieFilePath() string {
	cfg := sdsclient.DefaultConfig()
	return cfg.CookieFile
}

func (s *Service) legacyBrowserStateFilePath() string {
	return filepath.Join(filepath.Dir(s.legacyAuthFilePath()), "browser_state.json")
}

func (s *Service) legacyPayloadFilePath() string {
	return filepath.Join(filepath.Dir(s.legacyAuthFilePath()), "login_state.json")
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
