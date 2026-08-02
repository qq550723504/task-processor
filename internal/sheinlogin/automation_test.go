package sheinlogin

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

type blockingBrowserLaunchManager struct {
	launchStarted  chan struct{}
	releaseLaunch  chan struct{}
	launchReturned chan struct{}
	closed         chan struct{}
	closeOnce      sync.Once
}

func newBlockingBrowserLaunchManager() *blockingBrowserLaunchManager {
	return &blockingBrowserLaunchManager{
		launchStarted:  make(chan struct{}),
		releaseLaunch:  make(chan struct{}),
		launchReturned: make(chan struct{}),
		closed:         make(chan struct{}),
	}
}

func (m *blockingBrowserLaunchManager) Launch() error {
	close(m.launchStarted)
	<-m.releaseLaunch
	close(m.launchReturned)
	return nil
}

func (m *blockingBrowserLaunchManager) Close() {
	m.closeOnce.Do(func() { close(m.closed) })
}

type countingBlockingBrowserLaunchManager struct {
	launchStarted  chan struct{}
	releaseLaunch  chan struct{}
	launchReturned chan struct{}
	firstClose     chan struct{}
	closeCalls     atomic.Int32
	earlyClose     atomic.Bool
	closeSignal    sync.Once
}

func newCountingBlockingBrowserLaunchManager() *countingBlockingBrowserLaunchManager {
	return &countingBlockingBrowserLaunchManager{
		launchStarted:  make(chan struct{}),
		releaseLaunch:  make(chan struct{}),
		launchReturned: make(chan struct{}),
		firstClose:     make(chan struct{}),
	}
}

func (m *countingBlockingBrowserLaunchManager) Launch() error {
	close(m.launchStarted)
	<-m.releaseLaunch
	close(m.launchReturned)
	return nil
}

func (m *countingBlockingBrowserLaunchManager) Close() {
	m.closeCalls.Add(1)
	select {
	case <-m.launchReturned:
	default:
		m.earlyClose.Store(true)
	}
	m.closeSignal.Do(func() { close(m.firstClose) })
}

func TestLaunchManagerWithTimeoutDefersCleanupUntilLaunchReturns(t *testing.T) {
	manager := newBlockingBrowserLaunchManager()

	done := make(chan error, 1)
	go func() {
		done <- launchManagerWithTimeout(manager, 20*time.Millisecond)
	}()

	select {
	case <-manager.launchStarted:
	case <-time.After(time.Second):
		t.Fatal("Launch was not started")
	}

	var err error
	select {
	case err = <-done:
	case <-time.After(time.Second):
		t.Fatal("launchManagerWithTimeout did not return promptly after timeout")
	}
	if err == nil {
		t.Fatal("launchManagerWithTimeout error = nil, want timeout")
	}
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("launchManagerWithTimeout error = %v, want context.DeadlineExceeded", err)
	}

	select {
	case <-manager.closed:
		t.Fatal("Close should wait until Launch returns")
	default:
	}

	close(manager.releaseLaunch)

	select {
	case <-manager.launchReturned:
	case <-time.After(time.Second):
		t.Fatal("Launch did not return after release")
	}
	if !strings.Contains(err.Error(), "SHEIN browser launch timed out after 20ms") {
		t.Fatalf("launchManagerWithTimeout error = %v", err)
	}
	if shouldCloseManagerAfterStageError(err) {
		t.Fatal("timeout error should skip immediate close because stage cleanup is deferred")
	}
	select {
	case <-manager.closed:
	case <-time.After(time.Second):
		t.Fatal("Close was not called after Launch returned")
	}
}

func TestRunBlockingStageWithContextCancellationDefersLaunchCleanup(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	manager := newCountingBlockingBrowserLaunchManager()
	launchTimeout := time.Minute
	sharedCleanup := newOnceCleanup(manager.Close)

	done := make(chan error, 1)
	go func() {
		done <- runBlockingStageWithContext(ctx, nil, func() error {
			return launchManagerWithTimeoutAndCleanupContext(ctx, manager, launchTimeout, sharedCleanup)
		})
	}()

	select {
	case <-manager.launchStarted:
	case <-time.After(time.Second):
		t.Fatal("Launch was not started")
	}

	if got := manager.closeCalls.Load(); got != 0 {
		t.Fatalf("manager.Close called %d times before Launch returned, want 0", got)
	}

	cancel()

	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("runBlockingStageWithContext error = %v, want context.Canceled", err)
		}
	case <-time.After(time.Second):
		t.Fatal("runBlockingStageWithContext did not return after cancellation")
	}

	if got := manager.closeCalls.Load(); got != 0 {
		t.Fatalf("manager.Close called %d times before Launch returned after cancellation, want 0", got)
	}

	close(manager.releaseLaunch)

	select {
	case <-manager.launchReturned:
	case <-time.After(time.Second):
		t.Fatal("Launch did not return after release")
	}

	select {
	case <-manager.firstClose:
	case <-time.After(time.Second):
		t.Fatal("cleanup did not run after Launch returned")
	}

	if manager.earlyClose.Load() {
		t.Fatal("cleanup ran before Launch returned")
	}

	time.Sleep(50 * time.Millisecond)

	if got := manager.closeCalls.Load(); got != 1 {
		t.Fatalf("manager.Close called %d times, want 1", got)
	}
}

func TestRunBlockingStageWithContextDefersCleanupUntilStageReturns(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	started := make(chan struct{})
	release := make(chan struct{})
	stageReturned := make(chan struct{})
	closed := make(chan struct{})

	done := make(chan error, 1)
	go func() {
		done <- runBlockingStageWithContext(ctx, func() {
			select {
			case <-closed:
			default:
				close(closed)
			}
		}, func() error {
			close(started)
			<-release
			close(stageReturned)
			return nil
		})
	}()

	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("blocking stage did not start")
	}

	cancel()

	select {
	case <-closed:
		t.Fatal("cleanup should wait until the blocking stage returns")
	default:
	}

	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("runBlockingStageWithContext error = %v, want context.Canceled", err)
		}
	case <-time.After(time.Second):
		t.Fatal("blocking stage did not return after cancellation")
	}

	close(release)

	select {
	case <-stageReturned:
	case <-time.After(time.Second):
		t.Fatal("blocking stage goroutine did not return after release")
	}
	select {
	case <-closed:
	case <-time.After(time.Second):
		t.Fatal("cleanup did not run after the blocking stage returned")
	}
}

func TestBlockingStageInterruptBeforeReturnDefersCleanupUntilReturn(t *testing.T) {
	started := make(chan struct{})
	release := make(chan struct{})
	returned := make(chan struct{})
	cleaned := make(chan struct{})
	var cleanupCalls atomic.Int32

	stage := startBlockingStage(func() error {
		close(started)
		<-release
		close(returned)
		return nil
	}, func() {
		if cleanupCalls.Add(1) == 1 {
			close(cleaned)
		}
	})

	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("blocking stage did not start")
	}

	stage.interrupt()

	select {
	case <-cleaned:
		t.Fatal("cleanup should wait until the blocking stage returns")
	default:
	}

	close(release)

	select {
	case <-returned:
	case <-time.After(time.Second):
		t.Fatal("blocking stage did not return after release")
	}

	select {
	case <-cleaned:
	case <-time.After(time.Second):
		t.Fatal("cleanup did not run after the blocking stage returned")
	}

	if err := <-stage.result; err != nil {
		t.Fatalf("blocking stage result = %v", err)
	}

	stage.interrupt()

	if got := cleanupCalls.Load(); got != 1 {
		t.Fatalf("cleanup called %d times, want 1", got)
	}
}

func TestBlockingStageInterruptAfterReturnRunsCleanupOnce(t *testing.T) {
	returned := make(chan struct{})
	cleaned := make(chan struct{})
	var cleanupCalls atomic.Int32

	stage := startBlockingStage(func() error {
		close(returned)
		return nil
	}, func() {
		if cleanupCalls.Add(1) == 1 {
			close(cleaned)
		}
	})

	select {
	case <-returned:
	case <-time.After(time.Second):
		t.Fatal("blocking stage did not return")
	}

	stage.interrupt()

	select {
	case <-cleaned:
	case <-time.After(time.Second):
		t.Fatal("cleanup did not run after interrupting a completed stage")
	}

	if err := <-stage.result; err != nil {
		t.Fatalf("blocking stage result = %v", err)
	}

	stage.interrupt()

	if got := cleanupCalls.Load(); got != 1 {
		t.Fatalf("cleanup called %d times, want 1", got)
	}
}

func TestBuildAutomationBrowserConfigEnablesCloakBrowserMode(t *testing.T) {
	account := Account{
		StoreID: 869,
		Proxy:   "http://10.42.0.1:31069",
	}
	cfg := AutomationConfig{
		Headless:          true,
		BrowserPath:       "/app/.cloakbrowser/chrome",
		ChromeVersion:     "144",
		ChromeDownloadDir: "/app/chrome",
		ViewportWidth:     1920,
		ViewportHeight:    1080,
	}

	managerCfg := buildAutomationBrowserConfig(account, cfg)

	if got := managerCfg.StealthProvider; got != "cloakbrowser" {
		t.Fatalf("StealthProvider = %q, want %q", got, "cloakbrowser")
	}
	if got := managerCfg.FingerprintSeed; got != int32(account.StoreID) {
		t.Fatalf("FingerprintSeed = %d, want %d", got, account.StoreID)
	}
}

func TestBuildAutomationBrowserConfigCanIgnoreStoreProxy(t *testing.T) {
	t.Setenv("TASK_PROCESSOR_SHEIN_IGNORE_STORE_PROXY", "1")
	account := Account{
		StoreID: 869,
		Proxy:   "http://10.42.0.1:31069",
	}

	managerCfg := buildAutomationBrowserConfig(account, AutomationConfig{})

	if got := managerCfg.ProxyServer; got != "" {
		t.Fatalf("ProxyServer = %q, want empty when proxy ignored", got)
	}
}

func TestBuildAutomationBrowserConfigKeepsDefaultModeForNonCloakBrowser(t *testing.T) {
	account := Account{StoreID: 869}
	cfg := AutomationConfig{
		BrowserPath:    "/app/chromium/chrome",
		ViewportWidth:  1600,
		ViewportHeight: 900,
	}

	managerCfg := buildAutomationBrowserConfig(account, cfg)

	if got := managerCfg.StealthProvider; got != "" {
		t.Fatalf("StealthProvider = %q, want empty", got)
	}
	if got := managerCfg.Language; got != "zh-CN" {
		t.Fatalf("Language = %q, want %q", got, "zh-CN")
	}
	if got := managerCfg.AcceptLanguage; got != "zh-CN,zh;q=0.9,en;q=0.8" {
		t.Fatalf("AcceptLanguage = %q, want %q", got, "zh-CN,zh;q=0.9,en;q=0.8")
	}
	if got := managerCfg.FingerprintPlatform; got != "windows" {
		t.Fatalf("FingerprintPlatform = %q, want %q", got, "windows")
	}
	if got := managerCfg.FingerprintBrand; got != "Chrome" {
		t.Fatalf("FingerprintBrand = %q, want %q", got, "Chrome")
	}
	if got := managerCfg.Timezone; got != "Asia/Shanghai" {
		t.Fatalf("Timezone = %q, want %q", got, "Asia/Shanghai")
	}
	if got := managerCfg.UserAgent; got != "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/144.0.0.0 Safari/537.36" {
		t.Fatalf("UserAgent = %q", got)
	}
	if !managerCfg.SkipDefaultLaunchArgs {
		t.Fatal("SkipDefaultLaunchArgs should be enabled")
	}
	if !managerCfg.UseMinimalFingerprintArgs {
		t.Fatal("UseMinimalFingerprintArgs should be enabled")
	}
	wantLaunchArgs := []string{"--no-sandbox", "--disable-dev-shm-usage", "--enable-unsafe-swiftshader"}
	if !reflect.DeepEqual(managerCfg.ExtraLaunchArgs, wantLaunchArgs) {
		t.Fatalf("ExtraLaunchArgs = %#v", managerCfg.ExtraLaunchArgs)
	}
}

func TestBuildAutomationFingerprintAlignsWithBrowserConfig(t *testing.T) {
	account := Account{StoreID: 869}
	cfg := buildAutomationBrowserConfig(account, AutomationConfig{ChromeVersion: "144"})

	fp := buildAutomationFingerprint(account, cfg)

	if fp == nil || !fp.Enable {
		t.Fatal("fingerprint should be enabled")
	}
	if got := fp.Languages.HTTP; got != "zh-CN,zh;q=0.9,en;q=0.8" {
		t.Fatalf("HTTP language = %q", got)
	}
	if got := fp.Languages.JS; got != "zh-CN" {
		t.Fatalf("JS language = %q", got)
	}
	if got := fp.GPU["renderer"]; got != "ANGLE (NVIDIA, NVIDIA GeForce GTX 1060 Direct3D11 vs_5_0 ps_5_0, D3D11)" {
		t.Fatalf("GPU renderer = %q", got)
	}
}

func TestBuildResponseCaptureItemWithoutBodyPreview(t *testing.T) {
	item := buildResponseCaptureItem("playwright_page_response", "https://sso.geiwohuo.com/sso/authenticate/login", 200, "")

	if got := item["channel"]; got != "playwright_page_response" {
		t.Fatalf("channel = %v", got)
	}
	if got := item["url"]; got != "https://sso.geiwohuo.com/sso/authenticate/login" {
		t.Fatalf("url = %v", got)
	}
	if got := item["status"]; got != 200 {
		t.Fatalf("status = %v", got)
	}
	if _, ok := item["bodyPreview"]; ok {
		t.Fatal("bodyPreview should be omitted when body is empty")
	}
}

func TestClassifyLoginError(t *testing.T) {
	cases := []struct {
		name    string
		message string
		want    string
	}{
		{name: "verify code", message: "请输入已发送至您手机的otp码以验证身份", want: "VERIFY_CODE_REQUIRED"},
		{name: "bad password", message: "账号或密码错误", want: "INVALID_CREDENTIALS"},
		{name: "role permission", message: "请联系主账号在系统【账号管理】页面为您设置角色权限", want: "ROLE_PERMISSION_REQUIRED"},
		{name: "sign permission", message: "子账号无签署权限，需主账号", want: "SIGN_PERMISSION_REQUIRED"},
		{name: "request failed", message: "请求失败,尝试刷新页面,或联系开发", want: "REQUEST_FAILED"},
		{name: "fallback", message: "something else", want: "LOGIN_FAILED"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := classifyLoginErrorText(tc.message); got != tc.want {
				t.Fatalf("classifyLoginErrorText(%q) = %q, want %q", tc.message, got, tc.want)
			}
		})
	}
}

func TestClassifyLoginFailurePrefersStructuredState(t *testing.T) {
	metadata := artifactMetadata{
		Error:      "something else",
		LoginError: "something else",
		BodyText:   "欢迎使用 SHEIN",
		Title:      "SHEIN Login",
		SelectorStates: map[string]bool{
			"agreement_text": true,
		},
	}
	agreementVisible := true
	metadata.AgreementVisible = &agreementVisible

	if got := classifyLoginFailure(metadata); got != "SIGN_PERMISSION_REQUIRED" {
		t.Fatalf("classifyLoginFailure() = %q, want %q", got, "SIGN_PERMISSION_REQUIRED")
	}
}

func TestClassifyLoginFailureFallsBackToText(t *testing.T) {
	metadata := artifactMetadata{
		Error: "请求失败,尝试刷新页面,或联系开发",
	}
	if got := classifyLoginFailure(metadata); got != "REQUEST_FAILED" {
		t.Fatalf("classifyLoginFailure() = %q, want %q", got, "REQUEST_FAILED")
	}
}

func TestDerivePageState(t *testing.T) {
	verifyVisible := true
	metadata := artifactMetadata{VerificationVisible: &verifyVisible}
	if got := derivePageState(metadata); got != "verification" {
		t.Fatalf("derivePageState() = %q, want %q", got, "verification")
	}

	permissionVisible := true
	metadata = artifactMetadata{PermissionVisible: &permissionVisible}
	if got := derivePageState(metadata); got != "permission_gate" {
		t.Fatalf("derivePageState() = %q, want %q", got, "permission_gate")
	}
}

func TestDeriveFailureAction(t *testing.T) {
	actionKey, actionMessage := deriveFailureAction("verification", true, "VERIFY_CODE_REQUIRED")
	if actionKey != "submit_verify_code" || actionMessage == "" {
		t.Fatalf("unexpected verification action: %q %q", actionKey, actionMessage)
	}

	actionKey, actionMessage = deriveFailureAction("permission_gate", false, "ROLE_PERMISSION_REQUIRED")
	if actionKey != "fix_account_permission" || actionMessage == "" {
		t.Fatalf("unexpected permission action: %q %q", actionKey, actionMessage)
	}
}

func TestDetectLoginErrorText(t *testing.T) {
	got := detectLoginErrorText(
		"欢迎使用 SHEIN",
		"请求失败,尝试刷新页面,或联系开发",
	)
	if got == "" {
		t.Fatal("expected request failure text to be detected")
	}
	if got != "请求失败,尝试刷新页面,或联系开发" {
		t.Fatalf("detectLoginErrorText returned %q", got)
	}
}

func TestSummarizeBodyText(t *testing.T) {
	got := summarizeBodyText("  a  \n b\t c  ", 100)
	if got != "a b c" {
		t.Fatalf("summarizeBodyText normalized = %q", got)
	}

	got = summarizeBodyText("abcdef", 4)
	if got != "abcd" {
		t.Fatalf("summarizeBodyText truncated = %q", got)
	}
}

func TestDeriveBusinessVisibility(t *testing.T) {
	loginForm, sellerHub, verification, permission, agreement, credential := deriveBusinessVisibility(map[string]bool{
		"username_input":           true,
		"seller_hub_cn_text":       true,
		"verify_confirm_button":    true,
		"permission_dialog_button": true,
		"agreement_sign_button":    true,
		"credential_error_inline":  true,
	})
	if !loginForm || !sellerHub || !verification || !permission || !agreement || !credential {
		t.Fatalf("unexpected derived visibility: loginForm=%v sellerHub=%v verification=%v permission=%v agreement=%v credential=%v", loginForm, sellerHub, verification, permission, agreement, credential)
	}
}

func TestHasLoginSurfaceSignals(t *testing.T) {
	if !hasLoginSurfaceSignals(map[string]bool{"login_button": true}, "") {
		t.Fatal("expected selector-based login surface signal")
	}
	if !hasLoginSurfaceSignals(map[string]bool{}, "欢迎来到 SHEIN 全球商家中心，请登录后继续") {
		t.Fatal("expected text-based login surface signal")
	}
	if hasLoginSurfaceSignals(map[string]bool{}, "   ") {
		t.Fatal("expected empty content not to be treated as login surface")
	}
}

func TestClassifyLoginFailureUsesCaptchaSignals(t *testing.T) {
	metadata := artifactMetadata{
		BodyText: "系统检测到异常，请完成滑块验证码后重试",
	}
	if got := classifyLoginFailure(metadata); got != "LOGIN_FAILED" {
		t.Fatalf("classifyLoginFailure() = %q, want %q", got, "LOGIN_FAILED")
	}
}

func TestClassifyLoginFailureUsesLoginAPIResponse(t *testing.T) {
	metadata := artifactMetadata{
		PageState: "login_form",
		NetworkPayloads: []map[string]any{
			{
				"url":         "https://sso.geiwohuo.com/sso/authenticate/login",
				"status":      200,
				"bodyPreview": `{"code":"022008","msg":"需验证码后登录","info":{"needValidCode":true}}`,
			},
		},
	}

	if got := classifyLoginFailure(metadata); got != "VERIFY_CODE_REQUIRED" {
		t.Fatalf("classifyLoginFailure() = %q, want %q", got, "VERIFY_CODE_REQUIRED")
	}
}

func TestClassifyLoginFailureFromNetworkPayloadsUsesLoginAPIResponse(t *testing.T) {
	payloads := []map[string]any{
		{
			"url":         "https://sso.geiwohuo.com/sso/authenticate/login",
			"status":      200,
			"bodyPreview": `{"code":"022008","msg":"需验证码后登录","info":{"needValidCode":true}}`,
		},
	}

	if got := classifyLoginFailureFromNetworkPayloads(payloads); got != "VERIFY_CODE_REQUIRED" {
		t.Fatalf("classifyLoginFailureFromNetworkPayloads() = %q, want %q", got, "VERIFY_CODE_REQUIRED")
	}
}

func TestClassifyLoginFailureFromLoginResponseBodyUsesNeedValidCode(t *testing.T) {
	body := `{"code":"022008","msg":"需验证码后登录","info":{"needValidCode":true}}`
	if got := classifyLoginFailureFromLoginResponseBody(body); got != "VERIFY_CODE_REQUIRED" {
		t.Fatalf("classifyLoginFailureFromLoginResponseBody() = %q, want %q", got, "VERIFY_CODE_REQUIRED")
	}
}

func TestNetworkPayloadsRequireVerifyCode(t *testing.T) {
	payloads := []map[string]any{
		{
			"url":         "https://sso.geiwohuo.com/sso/authenticate/login",
			"status":      200,
			"bodyPreview": `{"code":"022008","msg":"需验证码后登录","info":{"needValidCode":true}}`,
		},
	}

	if got := networkPayloadsRequireVerifyCode(payloads); !got {
		t.Fatal("networkPayloadsRequireVerifyCode() = false, want true")
	}
}

func TestIsDeviceContextReadySnapshot(t *testing.T) {
	ready := map[string]any{
		"blackboxLength":           26,
		"antiInResolvedLength":     64,
		"armorTokenResolvedLength": 48,
		"smDeviceIdResolvedLength": 36,
	}
	if !isDeviceContextReadySnapshot(ready) {
		t.Fatal("expected ready snapshot to be treated as ready")
	}

	notReady := map[string]any{
		"blackboxLength":           26,
		"antiInResolvedLength":     0,
		"armorTokenResolvedLength": 48,
		"smDeviceIdResolvedLength": 36,
	}
	if isDeviceContextReadySnapshot(notReady) {
		t.Fatal("expected incomplete snapshot to be treated as not ready")
	}
}
