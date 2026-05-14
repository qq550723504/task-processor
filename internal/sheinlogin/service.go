package sheinlogin

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"task-processor/internal/core/config"
)

type LocalRefresher interface {
	ForceLogin(ctx context.Context, tenantID int64, storeID int64) error
}

type Service struct {
	provider          AccountProvider
	store             *RedisStore
	runtime           *Runtime
	automation        Automation
	defaultHeadless   bool
	profileRoot       string
	artifactDir       string
	browserPath       string
	chromeVersion     string
	chromeDownloadDir string
	viewportWidth     int
	viewportHeight    int
	adminPageEnabled  bool
	sessionsMu        sync.Mutex
	sessions          map[int64]VerifySession
}

func NewService(cfg config.LoginServiceConfig, redisCfg config.RedisConfig, browserCfg config.BrowserConfig, provider AccountProvider) (*Service, error) {
	store, err := NewRedisStore(redisCfg)
	if err != nil {
		return nil, err
	}
	runtime := NewRuntime(cfg.MaxConcurrentLogins)
	return &Service{
		provider:          provider,
		store:             store,
		runtime:           runtime,
		automation:        NewPlaywrightAutomation(),
		defaultHeadless:   cfg.DefaultHeadless,
		profileRoot:       cfg.ProfileRootDir,
		artifactDir:       cfg.ArtifactDir,
		browserPath:       browserCfg.BrowserPath,
		chromeVersion:     "144",
		chromeDownloadDir: "./chrome",
		viewportWidth:     browserCfg.ViewportWidth,
		viewportHeight:    browserCfg.ViewportHeight,
		adminPageEnabled:  cfg.AdminPageEnabled,
		sessions:          make(map[int64]VerifySession),
	}, nil
}

func (s *Service) Close() error {
	s.sessionsMu.Lock()
	for storeID, session := range s.sessions {
		_ = session.Close()
		delete(s.sessions, storeID)
	}
	s.sessionsMu.Unlock()
	return s.store.Close()
}

func (s *Service) Health(ctx context.Context) ServiceHealth {
	return ServiceHealth{
		Initialized:         s != nil && s.provider != nil && s.store != nil,
		RedisReady:          s.store.Ready(ctx),
		ManagementReady:     s.provider != nil,
		MaxConcurrentLogins: s.runtime.MaxConcurrent(),
		AdminPageEnabled:    s.adminPageEnabled,
	}
}

func (s *Service) ListAccounts(ctx context.Context, tenantID int64) ([]AccountStatus, error) {
	accounts, err := s.provider.ListAccounts(ctx, tenantID)
	if err != nil {
		return nil, err
	}
	result := make([]AccountStatus, 0, len(accounts))
	for _, account := range accounts {
		status, statusErr := s.Status(ctx, tenantID, account.StoreID)
		if statusErr == nil {
			result = append(result, *status)
		}
	}
	return result, nil
}

func (s *Service) Status(ctx context.Context, tenantID int64, storeID int64) (*AccountStatus, error) {
	account, err := s.provider.GetAccount(ctx, tenantID, storeID)
	if err != nil {
		return nil, err
	}
	ttl, hasCookie, err := s.store.CookieTTL(ctx, account.TenantID, account.StoreID)
	if err != nil {
		return nil, err
	}
	waiting, err := s.store.IsWaitingVerifyCode(ctx, account.TenantID, account.StoreID)
	if err != nil {
		return nil, err
	}
	lastLogin, err := s.store.LastLoginTime(ctx, account.TenantID, account.StoreID)
	if err != nil {
		return nil, err
	}
	lastFailure, err := s.store.LastFailure(ctx, account.TenantID, account.StoreID)
	if err != nil {
		return nil, err
	}
	recommendedAction := deriveRecommendedAction(waiting, lastFailure)
	return &AccountStatus{
		Account:              *account,
		HasCookie:            hasCookie,
		CookieTTL:            int64(ttl.Seconds()),
		WaitingForVerifyCode: waiting,
		LastLoginTime:        lastLogin,
		LoginInProgress:      s.runtime.IsInFlight(account.StoreID),
		LastFailure:          lastFailure,
		RecommendedAction:    recommendedAction,
	}, nil
}

func (s *Service) Login(ctx context.Context, tenantID int64, storeID int64, req LoginRequest) (*LoginResult, error) {
	account, err := s.provider.GetAccount(ctx, tenantID, storeID)
	if err != nil {
		return nil, err
	}
	if !req.ForceLogin {
		if ttl, ok, err := s.store.CookieTTL(ctx, account.TenantID, account.StoreID); err == nil && ok && ttl > 0 {
			return &LoginResult{
				Success:   true,
				Message:   "账号已有可用 Cookie",
				StoreID:   account.StoreID,
				TenantID:  account.TenantID,
				Username:  account.Username,
				CookieTTL: int64(ttl.Seconds()),
				LoginType: "existing",
			}, nil
		}
	}

	var result *LoginResult
	err = s.runtime.withStoreLock(account.StoreID, func() error {
		if req.ForceLogin {
			if err := s.store.ClearCookie(ctx, account.TenantID, account.StoreID); err != nil {
				return err
			}
		}
		headless := s.defaultHeadless
		if req.Headless != nil {
			headless = *req.Headless
		}
		runResult, session, runErr := s.automation.StartLogin(ctx, *account, AutomationConfig{
			Headless:          headless,
			ProfileRoot:       s.profileRoot,
			ArtifactDir:       s.artifactDir,
			BrowserPath:       s.browserPath,
			ChromeVersion:     s.chromeVersion,
			ChromeDownloadDir: s.chromeDownloadDir,
			ViewportWidth:     s.viewportWidth,
			ViewportHeight:    s.viewportHeight,
		})
		if runErr != nil {
			return runErr
		}
		if runResult.WaitingForVerifyCode {
			if runResult.FailureSummary != nil {
				_ = s.store.RecordLastFailure(ctx, account.TenantID, account.StoreID, runResult.FailureSummary, 30*24*time.Hour)
			}
			if err := s.store.SetVerifyWait(ctx, account.TenantID, account.StoreID, 10*time.Minute); err != nil {
				if session != nil {
					_ = session.Close()
				}
				return err
			}
			s.setSession(account.StoreID, session)
			result = &LoginResult{
				Success:              false,
				Message:              runResult.ErrorMessage,
				StoreID:              account.StoreID,
				TenantID:             account.TenantID,
				Username:             account.Username,
				WaitingForVerifyCode: true,
				ErrorCode:            runResult.ErrorCode,
				FailureArtifactPath:  runResult.FailureArtifactPath,
			}
			return nil
		}
		if session != nil {
			defer session.Close()
		}
		if runResult == nil || runResult.BrowserState == nil {
			summary := runResultFailureSummary(runResult)
			if summary != nil {
				_ = s.store.RecordLastFailure(ctx, account.TenantID, account.StoreID, summary, 30*24*time.Hour)
			}
			result = &LoginResult{
				Success:             false,
				Message:             failureMessage(runResult),
				StoreID:             account.StoreID,
				TenantID:            account.TenantID,
				Username:            account.Username,
				ErrorCode:           failureCode(runResult),
				FailureArtifactPath: failureArtifactPath(runResult),
				LastFailure:         summary,
			}
			return nil
		}
		cookies, _ := runResult.BrowserState["cookies"].([]any)
		if err := s.store.SaveCookieState(ctx, account.TenantID, account.StoreID, runResult.BrowserState, 30*24*time.Hour); err != nil {
			return err
		}
		now := time.Now()
		_ = s.store.RecordLastLoginTime(ctx, account.TenantID, account.StoreID, now)
		_ = s.store.ClearLastFailure(ctx, account.TenantID, account.StoreID)
		_ = s.store.ClearPauseKeys(ctx, account.TenantID, account.StoreID)
		_, _ = s.store.CancelVerifyWait(ctx, account.TenantID, account.StoreID)
		result = &LoginResult{
			Success:     true,
			Message:     "登录成功",
			StoreID:     account.StoreID,
			TenantID:    account.TenantID,
			Username:    account.Username,
			CookieCount: len(cookies),
			LoginType:   "new",
			LoginTime:   now,
		}
		return nil
	})
	if err != nil {
		return &LoginResult{
			Success:   false,
			Message:   err.Error(),
			StoreID:   account.StoreID,
			TenantID:  account.TenantID,
			Username:  account.Username,
			ErrorCode: "LOGIN_FAILED",
		}, nil
	}
	return result, nil
}

func (s *Service) ForceLogin(ctx context.Context, tenantID int64, storeID int64) error {
	result, err := s.Login(ctx, tenantID, storeID, LoginRequest{ForceLogin: true})
	if err != nil {
		return err
	}
	if result == nil || !result.Success {
		return fmt.Errorf("shein login failed: %s", result.Message)
	}
	return nil
}

func (s *Service) SubmitVerifyCode(ctx context.Context, tenantID int64, storeID int64, code string, expireSeconds int) error {
	account, err := s.provider.GetAccount(ctx, tenantID, storeID)
	if err != nil {
		return err
	}
	if session := s.loadSession(storeID); session != nil {
		result, runErr := session.SubmitCode(ctx, code)
		if runErr != nil {
			return runErr
		}
		if result != nil && result.BrowserState != nil {
			if err := s.store.SaveCookieState(ctx, account.TenantID, account.StoreID, result.BrowserState, 30*24*time.Hour); err != nil {
				return err
			}
			now := time.Now()
			_ = s.store.RecordLastLoginTime(ctx, account.TenantID, account.StoreID, now)
			_ = s.store.ClearLastFailure(ctx, account.TenantID, account.StoreID)
			_ = s.store.ClearPauseKeys(ctx, account.TenantID, account.StoreID)
			_, _ = s.store.CancelVerifyWait(ctx, account.TenantID, account.StoreID)
			s.clearSession(storeID)
			return nil
		}
		if result != nil && result.WaitingForVerifyCode {
			return nil
		}
	}
	ttl := 5 * time.Minute
	if expireSeconds > 0 {
		ttl = time.Duration(expireSeconds) * time.Second
	}
	return s.store.SubmitVerifyCode(ctx, account.TenantID, account.StoreID, code, ttl)
}

func runResultFailureSummary(runResult *AutomationResult) *FailureSummary {
	if runResult == nil {
		return &FailureSummary{ErrorCode: "LOGIN_FAILED", ErrorMessage: "login failed"}
	}
	if runResult.FailureSummary != nil {
		return runResult.FailureSummary
	}
	if runResult.ErrorCode == "" && runResult.ErrorMessage == "" && runResult.FailureArtifactPath == "" {
		return nil
	}
	return &FailureSummary{
		ErrorCode:            failureCode(runResult),
		ErrorMessage:         failureMessage(runResult),
		ArtifactPath:         failureArtifactPath(runResult),
		WaitingForVerifyCode: runResult.WaitingForVerifyCode,
		ActionKey:            "inspect_artifact",
		ActionMessage:        "查看失败详情和 artifact，确认当前页面分支后再处理",
	}
}

func failureMessage(runResult *AutomationResult) string {
	if runResult != nil && strings.TrimSpace(runResult.ErrorMessage) != "" {
		return runResult.ErrorMessage
	}
	return "login failed"
}

func failureCode(runResult *AutomationResult) string {
	if runResult != nil && strings.TrimSpace(runResult.ErrorCode) != "" {
		return runResult.ErrorCode
	}
	return "LOGIN_FAILED"
}

func failureArtifactPath(runResult *AutomationResult) string {
	if runResult == nil {
		return ""
	}
	return strings.TrimSpace(runResult.FailureArtifactPath)
}

func deriveRecommendedAction(waitingForVerifyCode bool, lastFailure *FailureSummary) RecommendedAction {
	if waitingForVerifyCode {
		return RecommendedAction{
			Key:     "submit_verify_code",
			Message: "提交验证码并继续当前登录会话",
		}
	}
	if lastFailure != nil {
		if strings.TrimSpace(lastFailure.ActionKey) != "" || strings.TrimSpace(lastFailure.ActionMessage) != "" {
			return RecommendedAction{
				Key:     strings.TrimSpace(lastFailure.ActionKey),
				Message: strings.TrimSpace(lastFailure.ActionMessage),
			}
		}
		key, message := deriveFailureAction(strings.TrimSpace(lastFailure.PageState), lastFailure.WaitingForVerifyCode, strings.TrimSpace(lastFailure.ErrorCode))
		if key != "" || message != "" {
			return RecommendedAction{Key: key, Message: message}
		}
	}
	return RecommendedAction{}
}

func failureDetailFromSummary(summary *FailureSummary) *FailureDetail {
	if summary == nil {
		return nil
	}
	return &FailureDetail{
		ErrorCode:            summary.ErrorCode,
		ErrorMessage:         summary.ErrorMessage,
		PageState:            summary.PageState,
		ActionKey:            summary.ActionKey,
		ActionMessage:        summary.ActionMessage,
		ArtifactPath:         summary.ArtifactPath,
		CapturedAt:           summary.CapturedAt,
		Stage:                summary.Stage,
		URL:                  summary.URL,
		Title:                summary.Title,
		LoginError:           summary.LoginError,
		WaitingForVerifyCode: summary.WaitingForVerifyCode,
	}
}

func failureDetailFromArtifact(summary *FailureSummary, metadata artifactMetadata) *FailureDetail {
	detail := failureDetailFromSummary(summary)
	if detail == nil {
		detail = &FailureDetail{}
	}
	if strings.TrimSpace(metadata.ErrorCode) != "" {
		detail.ErrorCode = metadata.ErrorCode
	}
	if strings.TrimSpace(metadata.Error) != "" {
		detail.ErrorMessage = metadata.Error
	}
	if strings.TrimSpace(metadata.PageState) != "" {
		detail.PageState = metadata.PageState
	}
	if strings.TrimSpace(metadata.ActionKey) != "" {
		detail.ActionKey = metadata.ActionKey
	}
	if strings.TrimSpace(metadata.ActionMessage) != "" {
		detail.ActionMessage = metadata.ActionMessage
	}
	if when := capturedAtFromMetadata(metadata); !when.IsZero() {
		detail.CapturedAt = when
	}
	if strings.TrimSpace(metadata.Stage) != "" {
		detail.Stage = metadata.Stage
	}
	if strings.TrimSpace(metadata.URL) != "" {
		detail.URL = metadata.URL
	}
	if strings.TrimSpace(metadata.Title) != "" {
		detail.Title = metadata.Title
	}
	if strings.TrimSpace(metadata.LoginError) != "" {
		detail.LoginError = metadata.LoginError
	}
	if metadata.VerifyCodeVisible != nil {
		detail.WaitingForVerifyCode = *metadata.VerifyCodeVisible
	}
	if metadata.OnLoginPage != nil {
		detail.OnLoginPage = *metadata.OnLoginPage
	}
	if metadata.RequestFailureModal != nil {
		detail.RequestFailureModal = *metadata.RequestFailureModal
	}
	if metadata.LoginFormVisible != nil {
		detail.LoginFormVisible = *metadata.LoginFormVisible
	}
	if metadata.SellerHubVisible != nil {
		detail.SellerHubVisible = *metadata.SellerHubVisible
	}
	if metadata.VerificationVisible != nil {
		detail.VerificationVisible = *metadata.VerificationVisible
	}
	if metadata.PermissionVisible != nil {
		detail.PermissionVisible = *metadata.PermissionVisible
	}
	if metadata.AgreementVisible != nil {
		detail.AgreementVisible = *metadata.AgreementVisible
	}
	if metadata.CredentialErrorVisible != nil {
		detail.CredentialErrorVisible = *metadata.CredentialErrorVisible
	}
	detail.BodyText = strings.TrimSpace(metadata.BodyText)
	if len(metadata.SelectorStates) > 0 {
		detail.SelectorStates = metadata.SelectorStates
	}
	return detail
}

func (s *Service) CancelVerifyCodeWait(ctx context.Context, tenantID int64, storeID int64) (bool, error) {
	account, err := s.provider.GetAccount(ctx, tenantID, storeID)
	if err != nil {
		return false, err
	}
	s.clearSession(storeID)
	return s.store.CancelVerifyWait(ctx, account.TenantID, account.StoreID)
}

func (s *Service) ClearCookie(ctx context.Context, tenantID int64, storeID int64) error {
	account, err := s.provider.GetAccount(ctx, tenantID, storeID)
	if err != nil {
		return err
	}
	s.clearSession(storeID)
	return s.store.ClearCookie(ctx, account.TenantID, account.StoreID)
}

func (s *Service) ClearLastFailure(ctx context.Context, tenantID int64, storeID int64) error {
	account, err := s.provider.GetAccount(ctx, tenantID, storeID)
	if err != nil {
		return err
	}
	return s.store.ClearLastFailure(ctx, account.TenantID, account.StoreID)
}

func (s *Service) GetLastFailureDetail(ctx context.Context, tenantID int64, storeID int64) (*FailureDetail, error) {
	account, err := s.provider.GetAccount(ctx, tenantID, storeID)
	if err != nil {
		return nil, err
	}
	summary, err := s.store.LastFailure(ctx, account.TenantID, account.StoreID)
	if err != nil {
		return nil, err
	}
	if summary == nil {
		return nil, nil
	}
	detail := failureDetailFromSummary(summary)
	if strings.TrimSpace(summary.ArtifactPath) == "" {
		return detail, nil
	}
	payload, err := os.ReadFile(filepath.Join(summary.ArtifactPath, "metadata.json"))
	if err != nil {
		return detail, nil
	}
	var metadata artifactMetadata
	if err := json.Unmarshal(payload, &metadata); err != nil {
		return detail, nil
	}
	return failureDetailFromArtifact(summary, metadata), nil
}

func (s *Service) setSession(storeID int64, session VerifySession) {
	if session == nil {
		return
	}
	s.sessionsMu.Lock()
	defer s.sessionsMu.Unlock()
	if existing := s.sessions[storeID]; existing != nil {
		_ = existing.Close()
	}
	s.sessions[storeID] = session
}

func (s *Service) loadSession(storeID int64) VerifySession {
	s.sessionsMu.Lock()
	defer s.sessionsMu.Unlock()
	return s.sessions[storeID]
}

func (s *Service) clearSession(storeID int64) {
	s.sessionsMu.Lock()
	defer s.sessionsMu.Unlock()
	if session := s.sessions[storeID]; session != nil {
		_ = session.Close()
	}
	delete(s.sessions, storeID)
}
