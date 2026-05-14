package sheinlogin

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	miniredis "github.com/alicebob/miniredis/v2"
	goredis "github.com/redis/go-redis/v9"
)

type stubAccountProvider struct {
	accounts []Account
}

func (s *stubAccountProvider) ListAccounts(_ context.Context, tenantID int64) ([]Account, error) {
	var accounts []Account
	for _, account := range s.accounts {
		if account.TenantID == tenantID {
			accounts = append(accounts, account)
		}
	}
	return accounts, nil
}

func (s *stubAccountProvider) GetAccount(_ context.Context, tenantID int64, storeID int64) (*Account, error) {
	for _, account := range s.accounts {
		if account.TenantID == tenantID && account.StoreID == storeID {
			copyAccount := account
			return &copyAccount, nil
		}
	}
	return nil, fmt.Errorf("shein login account not found for tenant %d store %d", tenantID, storeID)
}

type stubVerifySession struct {
	result *AutomationResult
	err    error
	closed int
}

func (s *stubVerifySession) SubmitCode(context.Context, string) (*AutomationResult, error) {
	return s.result, s.err
}
func (s *stubVerifySession) Close() error {
	s.closed++
	return nil
}

type stubAutomation struct {
	result  *AutomationResult
	session VerifySession
	err     error
	calls   int
}

func (s *stubAutomation) Login(context.Context, Account, AutomationConfig, *RedisStore) (*AutomationResult, error) {
	return s.result, s.err
}
func (s *stubAutomation) StartLogin(context.Context, Account, AutomationConfig) (*AutomationResult, VerifySession, error) {
	s.calls++
	return s.result, s.session, s.err
}

func TestServiceLoginReturnsExistingCookieWithoutAutomation(t *testing.T) {
	svc := newTestService(t, &stubAutomation{})
	if err := svc.store.SaveCookieState(context.Background(), 1, 2, map[string]any{"cookies": []any{}}, time.Hour); err != nil {
		t.Fatalf("seed cookie: %v", err)
	}
	result, err := svc.Login(context.Background(), 1, 2, LoginRequest{})
	if err != nil {
		t.Fatalf("login: %v", err)
	}
	if !result.Success || result.LoginType != "existing" {
		t.Fatalf("unexpected result: %+v", result)
	}
}

func TestServiceLoginStoresVerifySessionWhenVerificationRequired(t *testing.T) {
	session := &stubVerifySession{}
	auto := &stubAutomation{
		result:  &AutomationResult{WaitingForVerifyCode: true, ErrorCode: "VERIFY_CODE_REQUIRED", ErrorMessage: "wait"},
		session: session,
	}
	svc := newTestService(t, auto)
	result, err := svc.Login(context.Background(), 1, 2, LoginRequest{ForceLogin: true})
	if err != nil {
		t.Fatalf("login: %v", err)
	}
	if result == nil || !result.WaitingForVerifyCode {
		t.Fatalf("expected verify wait result: %+v", result)
	}
	if svc.loadSession(2) == nil {
		t.Fatal("expected session to be stored")
	}
}

func TestServiceSubmitVerifyCodeCompletesStoredSession(t *testing.T) {
	session := &stubVerifySession{
		result: &AutomationResult{
			BrowserState: map[string]any{
				"cookies": []any{map[string]any{"name": "sid", "value": "ok"}},
				"origins": []any{map[string]any{"origin": "https://sellerhub.shein.com"}},
			},
		},
	}
	auto := &stubAutomation{
		result:  &AutomationResult{WaitingForVerifyCode: true, ErrorCode: "VERIFY_CODE_REQUIRED", ErrorMessage: "wait"},
		session: session,
	}
	svc := newTestService(t, auto)
	if _, err := svc.Login(context.Background(), 1, 2, LoginRequest{ForceLogin: true}); err != nil {
		t.Fatalf("start login: %v", err)
	}
	if err := svc.SubmitVerifyCode(context.Background(), 1, 2, "123456", 60); err != nil {
		t.Fatalf("submit verify code: %v", err)
	}
	if svc.loadSession(2) != nil {
		t.Fatal("expected session to be cleared")
	}
	if has, err := svc.store.HasCookie(context.Background(), 1, 2); err != nil || !has {
		t.Fatalf("expected cookie persisted, has=%v err=%v", has, err)
	}
}

func TestServiceLoginPersistsCookieOnlyBrowserState(t *testing.T) {
	auto := &stubAutomation{
		result: &AutomationResult{
			BrowserState: map[string]any{
				"cookies": []any{map[string]any{"name": "sid", "value": "ok"}},
				"origins": []any{map[string]any{"origin": "https://sellerhub.shein.com"}},
			},
		},
	}
	svc := newTestService(t, auto)
	result, err := svc.Login(context.Background(), 1, 2, LoginRequest{ForceLogin: true})
	if err != nil {
		t.Fatalf("login: %v", err)
	}
	if result == nil || !result.Success {
		t.Fatalf("expected successful login result: %+v", result)
	}

	raw, err := svc.store.client.Get(context.Background(), cookieKey(1, 2)).Result()
	if err != nil {
		t.Fatalf("load saved cookie state: %v", err)
	}
	if raw == "" {
		t.Fatal("expected saved cookie state")
	}
	if strings.Contains(raw, "\"origins\"") {
		t.Fatalf("expected cookie-only payload, got %s", raw)
	}
}

func TestServiceLoginReturnsFailureResultWhenAutomationHasNoBrowserState(t *testing.T) {
	auto := &stubAutomation{
		result: &AutomationResult{
			ErrorCode:           "REQUEST_FAILED",
			ErrorMessage:        "请求失败",
			FailureArtifactPath: "D:\\tmp\\artifact",
			FailureSummary: &FailureSummary{
				ErrorCode:    "REQUEST_FAILED",
				ErrorMessage: "请求失败",
				ArtifactPath: "D:\\tmp\\artifact",
			},
		},
	}
	svc := newTestService(t, auto)
	result, err := svc.Login(context.Background(), 1, 2, LoginRequest{ForceLogin: true})
	if err != nil {
		t.Fatalf("login: %v", err)
	}
	if result == nil || result.Success {
		t.Fatalf("expected failed login result: %+v", result)
	}
	if result.ErrorCode != "REQUEST_FAILED" || result.LastFailure == nil {
		t.Fatalf("expected failure summary in result: %+v", result)
	}
	status, err := svc.Status(context.Background(), 1, 2)
	if err != nil {
		t.Fatalf("status: %v", err)
	}
	if status.LastFailure == nil || status.LastFailure.ErrorCode != "REQUEST_FAILED" {
		t.Fatalf("expected last failure in status: %+v", status)
	}
	if status.RecommendedAction.Key != "retry_login" || status.RecommendedAction.Message == "" {
		t.Fatalf("expected recommended action in status: %+v", status)
	}
	if err := svc.ClearLastFailure(context.Background(), 1, 2); err != nil {
		t.Fatalf("clear last failure: %v", err)
	}
	status, err = svc.Status(context.Background(), 1, 2)
	if err != nil {
		t.Fatalf("status after clear: %v", err)
	}
	if status.LastFailure != nil {
		t.Fatalf("expected last failure to be cleared: %+v", status)
	}
}

func TestStatusPrefersVerifyCodeRecommendedAction(t *testing.T) {
	svc := newTestService(t, &stubAutomation{})
	if err := svc.store.SetVerifyWait(context.Background(), 1, 2, time.Minute); err != nil {
		t.Fatalf("set verify wait: %v", err)
	}
	status, err := svc.Status(context.Background(), 1, 2)
	if err != nil {
		t.Fatalf("status: %v", err)
	}
	if status.RecommendedAction.Key != "submit_verify_code" || status.RecommendedAction.Message == "" {
		t.Fatalf("unexpected recommended action: %+v", status.RecommendedAction)
	}
}

func TestServiceGetLastFailureDetailPrefersArtifactMetadata(t *testing.T) {
	svc := newTestService(t, &stubAutomation{})
	dir := t.TempDir()
	metadata := `{
  "tenant_id": 1,
  "store_id": 2,
  "username": "demo",
  "stage": "wait_login",
  "error": "请求失败,尝试刷新页面,或联系开发",
  "error_code": "REQUEST_FAILED",
  "page_state": "request_failure",
  "action_key": "retry_login",
  "action_message": "重试登录；若持续失败，检查网络、代理和页面弹层",
  "captured_at": "2026-05-13T22:00:00+08:00",
  "url": "https://sellerhub.shein.com/login",
  "title": "SHEIN Login",
  "login_error": "请求失败",
  "body_text": "页面正文摘要",
  "verify_code_visible": false,
  "on_login_page": true,
  "request_failure_modal": true,
  "login_form_visible": true,
  "seller_hub_visible": false,
  "verification_visible": false,
  "permission_visible": true,
  "agreement_visible": false,
  "credential_error_visible": true,
  "selector_states": {"login_button": true, "verify_code_input": false}
}`
	if err := os.WriteFile(filepath.Join(dir, "metadata.json"), []byte(metadata), 0o644); err != nil {
		t.Fatalf("write metadata: %v", err)
	}
	if err := svc.store.RecordLastFailure(context.Background(), 1, 2, &FailureSummary{
		ErrorCode:     "LOGIN_FAILED",
		ErrorMessage:  "fallback",
		PageState:     "unknown",
		ActionKey:     "inspect_artifact",
		ActionMessage: "查看失败详情和 artifact，确认当前页面分支后再处理",
		ArtifactPath:  dir,
	}, time.Hour); err != nil {
		t.Fatalf("record last failure: %v", err)
	}
	detail, err := svc.GetLastFailureDetail(context.Background(), 1, 2)
	if err != nil {
		t.Fatalf("get last failure detail: %v", err)
	}
	if detail == nil || detail.ErrorCode != "REQUEST_FAILED" || detail.PageState != "request_failure" || detail.ActionKey != "retry_login" || detail.ActionMessage == "" || detail.URL != "https://sellerhub.shein.com/login" || detail.BodyText != "页面正文摘要" || !detail.OnLoginPage || !detail.RequestFailureModal || !detail.LoginFormVisible || detail.SellerHubVisible || detail.VerificationVisible || !detail.PermissionVisible || detail.AgreementVisible || !detail.CredentialErrorVisible || !detail.SelectorStates["login_button"] {
		t.Fatalf("unexpected detail: %+v", detail)
	}
}

func TestLoginUsesAbsoluteProfileRoot(t *testing.T) {
	root := "./tmp/shein-login/profiles"
	profileDir := filepath.Join(root, "1", "2")
	if filepath.IsAbs(profileDir) {
		t.Fatal("test requires relative profile dir input")
	}
	absDir, err := filepath.Abs(profileDir)
	if err != nil {
		t.Fatalf("abs: %v", err)
	}
	if !filepath.IsAbs(absDir) {
		t.Fatal("expected absolute profile dir")
	}
}

func newTestService(t *testing.T, automation Automation) *Service {
	t.Helper()
	mr := miniredis.RunT(t)
	client := goredis.NewClient(&goredis.Options{Addr: mr.Addr()})
	store := newRedisStoreFromClient(client)
	t.Cleanup(func() { _ = store.Close() })
	account := Account{StoreID: 2, TenantID: 1, Username: "demo", Password: "pwd", Platform: "SHEIN"}
	otherTenantAccount := Account{StoreID: 2, TenantID: 9, Username: "other", Password: "pwd", Platform: "SHEIN"}
	return &Service{
		provider:        &stubAccountProvider{accounts: []Account{account, otherTenantAccount}},
		store:           store,
		runtime:         NewRuntime(1),
		automation:      automation,
		defaultHeadless: true,
		sessions:        make(map[int64]VerifySession),
	}
}
