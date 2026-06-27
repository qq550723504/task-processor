package sheinlogin

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"task-processor/internal/core/config"
	"task-processor/internal/core/logger"
	"task-processor/internal/listingadmin"
	sheinother "task-processor/internal/shein/api/other"
	sheinwarehouse "task-processor/internal/shein/api/warehouse"
	sheinclient "task-processor/internal/shein/client"
)

type LocalRefresher interface {
	ForceLogin(ctx context.Context, tenantID int64, storeID int64) error
}

type Service struct {
	provider           AccountProvider
	store              *RedisStore
	runtime            *Runtime
	automation         Automation
	defaultHeadless    bool
	profileRoot        string
	artifactDir        string
	browserPath        string
	chromeVersion      string
	chromeDownloadDir  string
	viewportWidth      int
	viewportHeight     int
	sessionsMu         sync.Mutex
	sessions           map[int64]VerifySession
	sheinAPIClientFor  func(account Account) *sheinclient.APIClient
	resolveStoreID     func(ctx context.Context, account Account) (int64, error)
	storeClientFor     func(tenantID int64) StoreSyncClient
	findDuplicateStore func(ctx context.Context, account Account, actualStoreID string) (*listingadmin.StoreRespDTO, error)
}

type StoreSyncClient interface {
	UpdateStoreId(req *listingadmin.StoreIdUpdateReqDTO) (bool, error)
	UpdateStoreStatus(req *listingadmin.StoreStatusUpdateReqDTO) (bool, error)
}

var sheinLoginServiceLogger = logger.GetGlobalLogger("sheinlogin_service")

const maxAutomaticVerifyCodesPerDay = 2

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
	if s.loadSession(account.StoreID) != nil {
		if !waiting {
			s.clearSession(account.StoreID)
			lastFailure = verifyCodeWaitExpiredFailureSummary(lastFailure, account)
			_ = s.store.RecordLastFailure(ctx, account.TenantID, account.StoreID, lastFailure, 30*24*time.Hour)
		} else if lastFailure == nil || !lastFailure.WaitingForVerifyCode {
			lastFailure = verifyCodeFailureSummary(account)
		}
	}
	if waiting {
		if lastFailure == nil || !lastFailure.WaitingForVerifyCode {
			lastFailure = verifyCodeFailureSummary(account)
		}
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

func (s *Service) ListWarehouses(ctx context.Context, tenantID int64, storeID int64) ([]WarehouseOption, error) {
	_, err := s.provider.GetAccount(ctx, tenantID, storeID)
	if err != nil {
		return nil, err
	}
	apiClient := s.sheinAPIClient(Account{StoreID: storeID, TenantID: tenantID})
	if apiClient == nil {
		return nil, fmt.Errorf("shein api client is nil")
	}
	if !apiClient.HasCookies() {
		if err := apiClient.ReloadCookies(); err != nil {
			return nil, err
		}
	}
	if !apiClient.HasCookies() {
		return nil, fmt.Errorf("shein store cookie is unavailable")
	}
	baseAPI := sheinclient.NewBaseAPIClient(
		apiClient.GetBaseURL(),
		apiClient.GetTenantID(),
		storeID,
		apiClient.GetHTTPClient(),
	)
	baseAPI.SetAuthRefreshFunc(apiClient.ForceRefreshCookies)
	warehouseAPI := sheinwarehouse.NewClient(baseAPI)
	warehouses, err := warehouseAPI.GetWarehouses()
	if err != nil {
		return nil, err
	}
	items := make([]WarehouseOption, 0, len(warehouses.Data))
	for _, item := range warehouses.Data {
		items = append(items, WarehouseOption{
			WarehouseCode:   strings.TrimSpace(item.WarehouseCode),
			WarehouseName:   strings.TrimSpace(item.WarehouseName),
			SaleCountryList: append([]string(nil), item.SaleCountryList...),
			WarehouseType:   item.WarehouseType,
		})
	}
	return items, nil
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
			summary := runResult.FailureSummary
			if summary == nil {
				summary = verifyCodeFailureSummary(account)
			}
			_ = s.store.RecordLastFailure(ctx, account.TenantID, account.StoreID, summary, 30*24*time.Hour)
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
		s.syncStoreIDAfterLogin(ctx, *account)
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
	day := time.Now().In(sheinLoginLocalLocation())
	if count, err := s.store.AutoVerifyCodeSendCount(ctx, tenantID, storeID, day); err != nil {
		return err
	} else if count >= maxAutomaticVerifyCodesPerDay {
		return fmt.Errorf("今天自动验证码已达到 %d 次，请手动登录处理", maxAutomaticVerifyCodesPerDay)
	}

	result, err := s.Login(ctx, tenantID, storeID, LoginRequest{ForceLogin: true})
	if err != nil {
		return err
	}
	if result != nil && result.WaitingForVerifyCode {
		if _, err := s.store.RecordAutoVerifyCodeSent(ctx, tenantID, storeID, day); err != nil {
			return err
		}
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
			s.syncStoreIDAfterLogin(ctx, *account)
			s.clearSession(storeID)
			return nil
		}
		if result != nil && result.WaitingForVerifyCode {
			summary := runResultFailureSummary(result)
			if summary == nil {
				summary = verifyCodeFailureSummary(account)
			}
			_ = s.store.RecordLastFailure(ctx, account.TenantID, account.StoreID, summary, 30*24*time.Hour)
			if err := s.store.SetVerifyWait(ctx, account.TenantID, account.StoreID, 10*time.Minute); err != nil {
				return err
			}
			return nil
		}
	}
	ttl := 5 * time.Minute
	if expireSeconds > 0 {
		ttl = time.Duration(expireSeconds) * time.Second
	}
	return s.store.SubmitVerifyCode(ctx, account.TenantID, account.StoreID, code, ttl)
}

func verifyCodeFailureSummary(account *Account) *FailureSummary {
	if account == nil {
		return &FailureSummary{
			ErrorCode:            "VERIFY_CODE_REQUIRED",
			ErrorMessage:         "登录等待验证码",
			PageState:            "verification",
			ActionKey:            "submit_verify_code",
			ActionMessage:        "提交验证码并继续当前登录会话",
			WaitingForVerifyCode: true,
		}
	}
	return &FailureSummary{
		ErrorCode:            "VERIFY_CODE_REQUIRED",
		ErrorMessage:         "登录等待验证码",
		PageState:            "verification",
		ActionKey:            "submit_verify_code",
		ActionMessage:        "提交验证码并继续当前登录会话",
		WaitingForVerifyCode: true,
		Stage:                "wait_login",
		URL:                  loginURLForAccount(*account),
		Title:                "SHEIN全球商家中心",
	}
}

func verifyCodeWaitExpiredFailureSummary(summary *FailureSummary, account *Account) *FailureSummary {
	expired := verifyCodeFailureSummary(account)
	if summary != nil {
		copySummary := *summary
		expired = &copySummary
	}
	expired.ErrorCode = "VERIFY_CODE_WAIT_EXPIRED"
	expired.ErrorMessage = "验证码等待已超时，请重新发起登录"
	expired.PageState = "verification_timeout"
	expired.ActionKey = "retry_login"
	expired.ActionMessage = "验证码等待已超时，请重新发起登录"
	expired.WaitingForVerifyCode = false
	if strings.TrimSpace(expired.Stage) == "" {
		expired.Stage = "wait_login"
	}
	return expired
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
	if len(metadata.NetworkPayloads) > 0 {
		detail.NetworkPayloads = metadata.NetworkPayloads
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

func (s *Service) syncStoreIDAfterLogin(ctx context.Context, account Account) {
	actualStoreID, err := s.resolveActualStoreID(ctx, account)
	if err != nil {
		sheinLoginServiceLogger.WithError(err).WithFields(map[string]any{
			"tenant_id": account.TenantID,
			"store_id":  account.StoreID,
			"username":  account.Username,
		}).Warn("resolve actual SHEIN store id after login failed")
		return
	}
	if actualStoreID <= 0 {
		return
	}

	storeClient := s.storeClient(account.TenantID)
	if storeClient == nil {
		return
	}

	actualStoreIDText := strconv.FormatInt(actualStoreID, 10)
	duplicateStore, err := s.lookupDuplicateStore(ctx, account, actualStoreIDText)
	if err != nil {
		sheinLoginServiceLogger.WithError(err).WithFields(map[string]any{
			"tenant_id":       account.TenantID,
			"store_id":        account.StoreID,
			"actual_store_id": actualStoreIDText,
		}).Warn("lookup duplicate SHEIN store id failed")
	} else if duplicateStore != nil {
		duplicateStoreName := strings.TrimSpace(duplicateStore.Name)
		if duplicateStoreName == "" {
			duplicateStoreName = "-"
		}
		if _, err := storeClient.UpdateStoreStatus(&listingadmin.StoreStatusUpdateReqDTO{
			ID:     account.StoreID,
			Status: 1,
			Remark: fmt.Sprintf(
				"自动登录识别到重复店铺ID，已禁用：store_id=%s duplicate_store_row_id=%d duplicate_tenant_id=%d duplicate_store_name=%s",
				actualStoreIDText,
				duplicateStore.ID,
				duplicateStore.TenantID,
				duplicateStoreName,
			),
		}); err != nil {
			sheinLoginServiceLogger.WithError(err).WithFields(map[string]any{
				"tenant_id":              account.TenantID,
				"store_id":               account.StoreID,
				"actual_store_id":        actualStoreIDText,
				"duplicate_store_row_id": duplicateStore.ID,
				"duplicate_tenant_id":    duplicateStore.TenantID,
				"duplicate_store_name":   duplicateStoreName,
			}).Warn("disable duplicate SHEIN store failed")
		}
		return
	}

	recordedStoreID := strings.TrimSpace(account.StoreName)
	if recordedStoreID == "" {
		if _, err := storeClient.UpdateStoreId(&listingadmin.StoreIdUpdateReqDTO{
			ID:      account.StoreID,
			StoreID: actualStoreIDText,
		}); err != nil {
			sheinLoginServiceLogger.WithError(err).WithFields(map[string]any{
				"tenant_id":       account.TenantID,
				"store_id":        account.StoreID,
				"actual_store_id": actualStoreIDText,
			}).Warn("persist actual SHEIN store id after login failed")
		}
		return
	}

	recordedNumericStoreID, err := strconv.ParseInt(recordedStoreID, 10, 64)
	if err != nil {
		sheinLoginServiceLogger.WithError(err).WithFields(map[string]any{
			"tenant_id":         account.TenantID,
			"store_id":          account.StoreID,
			"recorded_store_id": recordedStoreID,
		}).Warn("stored SHEIN store id is not numeric, skip mismatch handling")
		return
	}
	if recordedNumericStoreID == actualStoreID {
		return
	}

	if _, err := storeClient.UpdateStoreStatus(&listingadmin.StoreStatusUpdateReqDTO{
		ID:     account.StoreID,
		Status: 1,
		Remark: fmt.Sprintf("自动登录识别到店铺ID不一致，已禁用：stored=%s actual=%s", recordedStoreID, actualStoreIDText),
	}); err != nil {
		sheinLoginServiceLogger.WithError(err).WithFields(map[string]any{
			"tenant_id":         account.TenantID,
			"store_id":          account.StoreID,
			"recorded_store_id": recordedStoreID,
			"actual_store_id":   actualStoreIDText,
		}).Warn("disable store on SHEIN store id mismatch failed")
	}
}

func (s *Service) resolveActualStoreID(ctx context.Context, account Account) (int64, error) {
	if s != nil && s.resolveStoreID != nil {
		return s.resolveStoreID(ctx, account)
	}
	apiClient := s.sheinAPIClient(account)
	if apiClient == nil {
		return 0, fmt.Errorf("shein api client is nil")
	}
	if !apiClient.HasCookies() {
		if err := apiClient.ReloadCookies(); err != nil {
			return 0, err
		}
	}
	if !apiClient.HasCookies() {
		return 0, fmt.Errorf("shein store cookie is unavailable")
	}

	baseAPI := sheinclient.NewBaseAPIClient(
		apiClient.GetBaseURL(),
		apiClient.GetTenantID(),
		account.StoreID,
		apiClient.GetHTTPClient(),
	)
	baseAPI.SetAuthRefreshFunc(apiClient.ForceRefreshCookies)
	otherAPI := sheinother.NewClient(baseAPI)
	supplierInfo, err := otherAPI.GetSupplierOperateInfo()
	if err != nil {
		return 0, err
	}
	return supplierInfo.Info.StoreID, nil
}

func (s *Service) storeClient(tenantID int64) StoreSyncClient {
	if s != nil && s.storeClientFor != nil {
		return s.storeClientFor(tenantID)
	}
	return nil
}

func (s *Service) lookupDuplicateStore(ctx context.Context, account Account, actualStoreID string) (*listingadmin.StoreRespDTO, error) {
	if s != nil && s.findDuplicateStore != nil {
		return s.findDuplicateStore(ctx, account, actualStoreID)
	}
	return nil, nil
}

func (s *Service) ConfigureRuntimeSheinAPIClients() {
	if s == nil {
		return
	}
	s.sheinAPIClientFor = func(account Account) *sheinclient.APIClient {
		return sheinclient.NewAPIClientWithStoreConfig(account.StoreID, &sheinclient.StoreConfig{
			ID:       account.StoreID,
			TenantID: account.TenantID,
			Name:     strings.TrimSpace(account.ShopName),
			Platform: strings.TrimSpace(account.Platform),
			LoginURL: strings.TrimSpace(account.LoginURL),
			Proxy:    strings.TrimSpace(account.Proxy),
			StoreID:  strings.TrimSpace(account.StoreName),
		}, serviceCookieProvider{store: s.store, tenantID: account.TenantID})
	}
}

func (s *Service) ConfigureStoreSyncClientFactory(factory func(tenantID int64) StoreSyncClient) {
	if s == nil {
		return
	}
	s.storeClientFor = factory
}

func (s *Service) ConfigureDuplicateStoreLookup(lookup func(ctx context.Context, account Account, actualStoreID string) (*listingadmin.StoreRespDTO, error)) {
	if s == nil {
		return
	}
	s.findDuplicateStore = lookup
}

func (s *Service) sheinAPIClient(account Account) *sheinclient.APIClient {
	if s != nil && s.sheinAPIClientFor != nil {
		return s.sheinAPIClientFor(account)
	}
	return nil
}

type serviceCookieProvider struct {
	store    *RedisStore
	tenantID int64
}

func (p serviceCookieProvider) GetCookie(ctx context.Context, storeID int64) (*sheinclient.CookieLookupResult, error) {
	if p.store == nil || p.tenantID <= 0 || storeID <= 0 {
		return nil, nil
	}
	raw, ok, err := p.store.LoadCookieState(ctx, p.tenantID, storeID)
	if err != nil || !ok {
		return nil, err
	}
	return &sheinclient.CookieLookupResult{
		TenantID:   p.tenantID,
		CookieJSON: raw,
	}, nil
}
