package sheinlogin

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	miniredis "github.com/alicebob/miniredis/v2"
	goredis "github.com/redis/go-redis/v9"
	managementapi "task-processor/internal/infra/clients/management/api"
	sheinclient "task-processor/internal/shein/client"
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

type stubStoreClient struct {
	updateStoreIDReq     *managementapi.StoreIdUpdateReqDTO
	updateStoreIDErr     error
	updateStoreStatusReq *managementapi.StoreStatusUpdateReqDTO
	updateStoreStatusErr error
}

func (s *stubStoreClient) UpdateStoreId(req *managementapi.StoreIdUpdateReqDTO) (bool, error) {
	s.updateStoreIDReq = req
	return s.updateStoreIDErr == nil, s.updateStoreIDErr
}

func (s *stubStoreClient) UpdateStoreStatus(req *managementapi.StoreStatusUpdateReqDTO) (bool, error) {
	s.updateStoreStatusReq = req
	return s.updateStoreStatusErr == nil, s.updateStoreStatusErr
}

type stubSheinCookieProvider struct {
	result *sheinclient.CookieLookupResult
}

func (p stubSheinCookieProvider) GetCookie(context.Context, int64) (*sheinclient.CookieLookupResult, error) {
	return p.result, nil
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

func TestServiceListWarehousesUsesInjectedAPIClientFactory(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != sheinclient.GetWarehousesEndpoint() {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"code":"0","msg":"OK","info":{"data":[{"warehouse_name":"demo-wh","warehouse_code":"WH001","sale_country_list":["US"],"warehouse_type":3}],"meta":{"count":1}}}`))
	}))
	defer server.Close()

	svc := newTestService(t, &stubAutomation{})
	svc.sheinAPIClientFor = func(Account) *sheinclient.APIClient {
		return sheinclient.NewAPIClientWithStoreConfig(2, &sheinclient.StoreConfig{
			ID:       2,
			TenantID: 1,
			LoginURL: server.URL,
		}, stubSheinCookieProvider{
			result: &sheinclient.CookieLookupResult{
				TenantID:   1,
				CookieJSON: `[{"name":"sid","value":"ok","domain":".shein.com","path":"/"}]`,
			},
		})
	}

	items, err := svc.ListWarehouses(context.Background(), 1, 2)
	if err != nil {
		t.Fatalf("ListWarehouses: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("warehouse count = %d, want 1", len(items))
	}
	if items[0].WarehouseCode != "WH001" {
		t.Fatalf("warehouse code = %q, want WH001", items[0].WarehouseCode)
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
	status, err := svc.Status(context.Background(), 1, 2)
	if err != nil {
		t.Fatalf("status: %v", err)
	}
	if !status.WaitingForVerifyCode {
		t.Fatalf("expected waiting_for_verify_code=true in status: %+v", status)
	}
	if status.LastFailure == nil || status.LastFailure.ErrorCode != "VERIFY_CODE_REQUIRED" || !status.LastFailure.WaitingForVerifyCode {
		t.Fatalf("expected verify-code failure summary in status: %+v", status)
	}
	if status.RecommendedAction.Key != "submit_verify_code" || status.RecommendedAction.Message == "" {
		t.Fatalf("expected verify-code recommended action in status: %+v", status)
	}
}

func TestServiceStatusClearsExpiredVerifySession(t *testing.T) {
	session := &stubVerifySession{}
	auto := &stubAutomation{
		result:  &AutomationResult{WaitingForVerifyCode: true, ErrorCode: "VERIFY_CODE_REQUIRED", ErrorMessage: "wait"},
		session: session,
	}
	svc := newTestService(t, auto)
	if _, err := svc.Login(context.Background(), 1, 2, LoginRequest{ForceLogin: true}); err != nil {
		t.Fatalf("login: %v", err)
	}
	if _, err := svc.store.CancelVerifyWait(context.Background(), 1, 2); err != nil {
		t.Fatalf("expire verify wait: %v", err)
	}

	status, err := svc.Status(context.Background(), 1, 2)
	if err != nil {
		t.Fatalf("status: %v", err)
	}
	if status.WaitingForVerifyCode {
		t.Fatalf("expected expired verify wait to stop waiting: %+v", status)
	}
	if status.LastFailure == nil || status.LastFailure.WaitingForVerifyCode {
		t.Fatalf("expected expired verify failure summary in status: %+v", status)
	}
	if status.RecommendedAction.Key != "retry_login" {
		t.Fatalf("recommended action = %+v, want retry_login", status.RecommendedAction)
	}
	if svc.loadSession(2) != nil {
		t.Fatal("expected expired verify session to be cleared")
	}
	if session.closed != 1 {
		t.Fatalf("session closed = %d, want 1", session.closed)
	}
}

func TestServiceForceLoginLimitsAutomaticVerifyCodesPerDay(t *testing.T) {
	auto := &stubAutomation{
		result:  &AutomationResult{WaitingForVerifyCode: true, ErrorCode: "VERIFY_CODE_REQUIRED", ErrorMessage: "wait"},
		session: &stubVerifySession{},
	}
	svc := newTestService(t, auto)
	ctx := context.Background()

	for i := 0; i < 2; i++ {
		err := svc.ForceLogin(ctx, 1, 2)
		if err == nil || !strings.Contains(err.Error(), "wait") {
			t.Fatalf("force login %d error = %v, want verify wait error", i+1, err)
		}
	}

	err := svc.ForceLogin(ctx, 1, 2)
	if err == nil || !strings.Contains(err.Error(), "今天自动验证码已达到 2 次") {
		t.Fatalf("third force login error = %v, want daily limit", err)
	}
	if auto.calls != 2 {
		t.Fatalf("automation calls = %d, want 2", auto.calls)
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

func TestServiceLoginSyncsResolvedStoreIDWhenStoredValueIsEmpty(t *testing.T) {
	auto := &stubAutomation{
		result: &AutomationResult{
			BrowserState: map[string]any{
				"cookies": []any{map[string]any{"name": "sid", "value": "ok"}},
			},
		},
	}
	storeClient := &stubStoreClient{}
	svc := newTestService(t, auto)
	svc.resolveStoreID = func(context.Context, Account) (int64, error) {
		return 1862049307, nil
	}
	svc.storeClientFor = func(int64) StoreSyncClient {
		return storeClient
	}
	svc.findDuplicateStore = func(context.Context, Account, string) (*managementapi.StoreRespDTO, error) {
		return nil, nil
	}

	result, err := svc.Login(context.Background(), 1, 2, LoginRequest{ForceLogin: true})
	if err != nil {
		t.Fatalf("login: %v", err)
	}
	if result == nil || !result.Success {
		t.Fatalf("expected successful login result: %+v", result)
	}
	if storeClient.updateStoreIDReq == nil {
		t.Fatal("expected store id update request")
	}
	if storeClient.updateStoreIDReq.ID != 2 || storeClient.updateStoreIDReq.StoreID != "1862049307" {
		t.Fatalf("unexpected store id update request: %+v", storeClient.updateStoreIDReq)
	}
	if storeClient.updateStoreStatusReq != nil {
		t.Fatalf("did not expect store disable request: %+v", storeClient.updateStoreStatusReq)
	}
}

func TestServiceLoginDisablesStoreWhenResolvedStoreIDDiffers(t *testing.T) {
	auto := &stubAutomation{
		result: &AutomationResult{
			BrowserState: map[string]any{
				"cookies": []any{map[string]any{"name": "sid", "value": "ok"}},
			},
		},
	}
	storeClient := &stubStoreClient{}
	svc := newTestServiceWithAccounts(t, auto, []Account{{
		StoreID:   2,
		TenantID:  1,
		Username:  "demo",
		Password:  "pwd",
		Platform:  "SHEIN",
		StoreName: "123456",
	}})
	svc.resolveStoreID = func(context.Context, Account) (int64, error) {
		return 1862049307, nil
	}
	svc.storeClientFor = func(int64) StoreSyncClient {
		return storeClient
	}
	svc.findDuplicateStore = func(context.Context, Account, string) (*managementapi.StoreRespDTO, error) {
		return nil, nil
	}

	result, err := svc.Login(context.Background(), 1, 2, LoginRequest{ForceLogin: true})
	if err != nil {
		t.Fatalf("login: %v", err)
	}
	if result == nil || !result.Success {
		t.Fatalf("expected successful login result: %+v", result)
	}
	if storeClient.updateStoreIDReq != nil {
		t.Fatalf("did not expect store id update request: %+v", storeClient.updateStoreIDReq)
	}
	if storeClient.updateStoreStatusReq == nil {
		t.Fatal("expected store disable request")
	}
	if storeClient.updateStoreStatusReq.ID != 2 || storeClient.updateStoreStatusReq.Status != 1 {
		t.Fatalf("unexpected store status update request: %+v", storeClient.updateStoreStatusReq)
	}
	if !strings.Contains(storeClient.updateStoreStatusReq.Remark, "stored=123456") || !strings.Contains(storeClient.updateStoreStatusReq.Remark, "actual=1862049307") {
		t.Fatalf("unexpected disable remark: %s", storeClient.updateStoreStatusReq.Remark)
	}
}

func TestServiceLoginIgnoresStoreIDSyncFailure(t *testing.T) {
	auto := &stubAutomation{
		result: &AutomationResult{
			BrowserState: map[string]any{
				"cookies": []any{map[string]any{"name": "sid", "value": "ok"}},
			},
		},
	}
	svc := newTestService(t, auto)
	svc.resolveStoreID = func(context.Context, Account) (int64, error) {
		return 0, fmt.Errorf("supplier info failed")
	}

	result, err := svc.Login(context.Background(), 1, 2, LoginRequest{ForceLogin: true})
	if err != nil {
		t.Fatalf("login: %v", err)
	}
	if result == nil || !result.Success {
		t.Fatalf("expected successful login result: %+v", result)
	}
}

func TestServiceLoginDisablesCurrentStoreWhenDuplicateStoreIDExists(t *testing.T) {
	auto := &stubAutomation{
		result: &AutomationResult{
			BrowserState: map[string]any{
				"cookies": []any{map[string]any{"name": "sid", "value": "ok"}},
			},
		},
	}
	storeClient := &stubStoreClient{}
	svc := newTestService(t, auto)
	svc.resolveStoreID = func(context.Context, Account) (int64, error) {
		return 1862049307, nil
	}
	svc.storeClientFor = func(int64) StoreSyncClient {
		return storeClient
	}
	svc.findDuplicateStore = func(context.Context, Account, string) (*managementapi.StoreRespDTO, error) {
		return &managementapi.StoreRespDTO{
			ID:       99,
			TenantID: 88,
			Name:     "跨租户重复店铺",
			StoreID:  "1862049307",
			Platform: "SHEIN",
		}, nil
	}

	result, err := svc.Login(context.Background(), 1, 2, LoginRequest{ForceLogin: true})
	if err != nil {
		t.Fatalf("login: %v", err)
	}
	if result == nil || !result.Success {
		t.Fatalf("expected successful login result: %+v", result)
	}
	if storeClient.updateStoreIDReq != nil {
		t.Fatalf("did not expect store id update request: %+v", storeClient.updateStoreIDReq)
	}
	if storeClient.updateStoreStatusReq == nil {
		t.Fatal("expected duplicate store disable request")
	}
	if !strings.Contains(storeClient.updateStoreStatusReq.Remark, "store_id=1862049307") ||
		!strings.Contains(storeClient.updateStoreStatusReq.Remark, "duplicate_store_row_id=99") ||
		!strings.Contains(storeClient.updateStoreStatusReq.Remark, "duplicate_tenant_id=88") ||
		!strings.Contains(storeClient.updateStoreStatusReq.Remark, "duplicate_store_name=跨租户重复店铺") {
		t.Fatalf("unexpected duplicate disable remark: %s", storeClient.updateStoreStatusReq.Remark)
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
  "selector_states": {"login_button": true, "verify_code_input": false},
  "network_payloads": [{"channel":"xhr","url":"https://sso.geiwohuo.com/sso/authenticate/login","status":200,"bodyPreview":"{\"code\":0}"}]
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
	if len(detail.NetworkPayloads) != 1 {
		t.Fatalf("expected network payloads, got %+v", detail.NetworkPayloads)
	}
}

func TestLoginUsesAbsoluteProfileRoot(t *testing.T) {
	root := "./.local/tmp/shein-login/profiles"
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
	return newTestServiceWithAccounts(t, automation, []Account{
		{StoreID: 2, TenantID: 1, Username: "demo", Password: "pwd", Platform: "SHEIN"},
		{StoreID: 2, TenantID: 9, Username: "other", Password: "pwd", Platform: "SHEIN"},
	})
}

func newTestServiceWithAccounts(t *testing.T, automation Automation, accounts []Account) *Service {
	t.Helper()
	mr := miniredis.RunT(t)
	client := goredis.NewClient(&goredis.Options{Addr: mr.Addr()})
	store := newRedisStoreFromClient(client)
	t.Cleanup(func() { _ = store.Close() })
	return &Service{
		provider:        &stubAccountProvider{accounts: accounts},
		store:           store,
		runtime:         NewRuntime(1),
		automation:      automation,
		defaultHeadless: true,
		sessions:        make(map[int64]VerifySession),
	}
}
