package sheinlogin

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	sharedbrowser "task-processor/internal/crawler/shared/browser"

	"github.com/playwright-community/playwright-go"
)

type AutomationConfig struct {
	Headless          bool
	ProfileRoot       string
	ArtifactDir       string
	BrowserPath       string
	ChromeVersion     string
	ChromeDownloadDir string
	ViewportWidth     int
	ViewportHeight    int
}

type AutomationResult struct {
	BrowserState         map[string]any
	WaitingForVerifyCode bool
	ErrorCode            string
	ErrorMessage         string
	FailureArtifactPath  string
	FailureSummary       *FailureSummary
}

type artifactMetadata struct {
	TenantID               int64           `json:"tenant_id"`
	StoreID                int64           `json:"store_id"`
	Username               string          `json:"username"`
	Stage                  string          `json:"stage"`
	Error                  string          `json:"error"`
	ErrorCode              string          `json:"error_code"`
	PageState              string          `json:"page_state,omitempty"`
	ActionKey              string          `json:"action_key,omitempty"`
	ActionMessage          string          `json:"action_message,omitempty"`
	CapturedAt             string          `json:"captured_at"`
	URL                    string          `json:"url,omitempty"`
	Title                  string          `json:"title,omitempty"`
	LoggedIn               *bool           `json:"logged_in,omitempty"`
	VerifyCodeVisible      *bool           `json:"verify_code_visible,omitempty"`
	OnLoginPage            *bool           `json:"on_login_page,omitempty"`
	RequestFailureModal    *bool           `json:"request_failure_modal,omitempty"`
	LoginFormVisible       *bool           `json:"login_form_visible,omitempty"`
	SellerHubVisible       *bool           `json:"seller_hub_visible,omitempty"`
	VerificationVisible    *bool           `json:"verification_visible,omitempty"`
	PermissionVisible      *bool           `json:"permission_visible,omitempty"`
	AgreementVisible       *bool           `json:"agreement_visible,omitempty"`
	CredentialErrorVisible *bool           `json:"credential_error_visible,omitempty"`
	LoginError             string          `json:"login_error,omitempty"`
	BodyText               string          `json:"body_text,omitempty"`
	SelectorStates         map[string]bool `json:"selector_states,omitempty"`
	NetworkPayloads        []map[string]any `json:"network_payloads,omitempty"`
}

type Automation interface {
	Login(ctx context.Context, account Account, cfg AutomationConfig, store *RedisStore) (*AutomationResult, error)
	StartLogin(ctx context.Context, account Account, cfg AutomationConfig) (*AutomationResult, VerifySession, error)
}

type VerifySession interface {
	SubmitCode(ctx context.Context, code string) (*AutomationResult, error)
	Close() error
}

type PlaywrightAutomation struct{}

func NewPlaywrightAutomation() *PlaywrightAutomation { return &PlaywrightAutomation{} }

var sheinLoginErrorSelectors = []string{
	".soui-dialog",
	".soui-dialog-body",
	".soui-form-error",
	".soui-input-error",
	"[class*='error']",
	"[role='dialog']",
}

func (a *PlaywrightAutomation) Login(ctx context.Context, account Account, cfg AutomationConfig, store *RedisStore) (*AutomationResult, error) {
	result, session, err := a.StartLogin(ctx, account, cfg)
	if err != nil {
		return nil, err
	}
	if result.WaitingForVerifyCode {
		defer session.Close()
		if store != nil {
			_ = store.SetVerifyWait(ctx, account.TenantID, account.StoreID, 10*time.Minute)
			if code, ok, consumeErr := store.WaitAndConsumeVerifyCode(ctx, account.TenantID, account.StoreID, 10*time.Minute); consumeErr != nil {
				return nil, consumeErr
			} else if ok && strings.TrimSpace(code) != "" {
				return session.SubmitCode(ctx, code)
			}
		} else {
			<-ctx.Done()
			return nil, ctx.Err()
		}
		return &AutomationResult{
			WaitingForVerifyCode: true,
			ErrorCode:            "VERIFY_CODE_REQUIRED",
			ErrorMessage:         "登录等待验证码",
		}, nil
	}
	if session != nil {
		defer session.Close()
	}
	return result, nil
}

func (a *PlaywrightAutomation) StartLogin(ctx context.Context, account Account, cfg AutomationConfig) (*AutomationResult, VerifySession, error) {
	profileDir, err := resolveProfileDir(cfg.ProfileRoot, account.TenantID, account.StoreID)
	if err != nil {
		return nil, nil, err
	}
	if err := os.MkdirAll(profileDir, 0o755); err != nil {
		return nil, nil, err
	}
	managerCfg := &sharedbrowser.BrowserConfig{
		Headless:          cfg.Headless,
		BrowserPath:       strings.TrimSpace(cfg.BrowserPath),
		ChromeVersion:     strings.TrimSpace(cfg.ChromeVersion),
		ChromeDownloadDir: strings.TrimSpace(cfg.ChromeDownloadDir),
		ProxyServer:       strings.TrimSpace(account.Proxy),
		ViewportWidth:     defaultViewport(cfg.ViewportWidth, 1440),
		ViewportHeight:    defaultViewport(cfg.ViewportHeight, 960),
		FingerprintSeed:   int32(account.StoreID),
		Language:          "en-US",
		AcceptLanguage:    "en-US,en;q=0.9",
	}
	manager := sharedbrowser.NewManager(managerCfg)
	manager.SetUserDataDir(profileDir)
	manager.SetFingerprint(manager.GenerateStableFingerprint(strconv.FormatInt(account.StoreID, 10)))
	if err := manager.Install(); err != nil {
		return nil, nil, err
	}
	if err := launchManagerWithProfileRecovery(manager, profileDir); err != nil {
		manager.Close()
		trimProfileDir(profileDir)
		return nil, nil, err
	}

	page, err := manager.NewPage()
	if err != nil {
		closeManagerProfile(manager, profileDir)
		return nil, nil, err
	}
	_ = installPageNetworkCapture(page)
	if _, err = page.Goto(loginURLForAccount(account), playwright.PageGotoOptions{
		WaitUntil: playwright.WaitUntilStateDomcontentloaded,
		Timeout:   playwright.Float(60000),
	}); err != nil {
		closeManagerProfile(manager, profileDir)
		return nil, nil, err
	}
	if err := waitForLoginSurface(ctx, page); err != nil {
		result, resultErr := artifactResult(page, cfg.ArtifactDir, account, "wait_login_surface", err)
		closeManagerProfile(manager, profileDir)
		return result, nil, resultErr
	}
	if err := fillLogin(page, account); err != nil {
		result, resultErr := artifactResult(page, cfg.ArtifactDir, account, "fill_login", err)
		closeManagerProfile(manager, profileDir)
		return result, nil, resultErr
	}
	if err := submitLogin(page); err != nil {
		result, resultErr := artifactResult(page, cfg.ArtifactDir, account, "submit_login", err)
		closeManagerProfile(manager, profileDir)
		return result, nil, resultErr
	}
	if waiting, err := waitForLoginOutcome(ctx, page); err != nil {
		settleAfterSubmit(page, 8*time.Second)
		if loggedIn, loginErr := isLoggedIn(page); loginErr == nil && loggedIn {
			storageState, stateErr := manager.GetContext().StorageState()
			if stateErr != nil {
				result, resultErr := artifactResult(page, cfg.ArtifactDir, account, "export_state_after_recover", stateErr)
				closeManagerProfile(manager, profileDir)
				return result, nil, resultErr
			}
			state := cookieOnlyBrowserState(map[string]any{"cookies": storageState.Cookies})
			closeManagerProfile(manager, profileDir)
			return &AutomationResult{BrowserState: state}, nil, nil
		}
		if verifyRequired, verifyErr := isVerifyCodeRequired(page); verifyErr == nil && verifyRequired {
			return &AutomationResult{
					WaitingForVerifyCode: true,
					ErrorCode:            "VERIFY_CODE_REQUIRED",
					ErrorMessage:         "登录等待验证码",
				}, &playwrightVerifySession{
					account:     account,
					manager:     manager,
					page:        page,
					artifactDir: cfg.ArtifactDir,
					profileDir:  profileDir,
				}, nil
		}
		result, resultErr := artifactResult(page, cfg.ArtifactDir, account, "wait_login", err)
		closeManagerProfile(manager, profileDir)
		return result, nil, resultErr
	} else if waiting {
		return &AutomationResult{
				WaitingForVerifyCode: true,
				ErrorCode:            "VERIFY_CODE_REQUIRED",
				ErrorMessage:         "登录等待验证码",
			}, &playwrightVerifySession{
				account:     account,
				manager:     manager,
				page:        page,
				artifactDir: cfg.ArtifactDir,
				profileDir:  profileDir,
			}, nil
	}

	storageState, err := manager.GetContext().StorageState()
	if err != nil {
		result, resultErr := artifactResult(page, cfg.ArtifactDir, account, "export_state", err)
		closeManagerProfile(manager, profileDir)
		return result, nil, resultErr
	}
	state := cookieOnlyBrowserState(map[string]any{"cookies": storageState.Cookies})
	closeManagerProfile(manager, profileDir)
	return &AutomationResult{BrowserState: state}, nil, nil
}

func installPageNetworkCapture(page playwright.Page) error {
	if page == nil {
		return nil
	}
	script := `
(() => {
  if (window.__codexAuthPayloadCaptureInstalled) return;
  window.__codexAuthPayloadCaptureInstalled = true;
  window.__codexAuthPayloads = [];
  const shouldCapture = (url) => {
    const lowered = String(url || '').toLowerCase();
    return lowered.includes('/sso/authenticate/login')
      || lowered.includes('/sso/geetest/ajax.php')
      || lowered.includes('/sso/geetest/reset.php');
  };
  const pushPayload = (item) => {
    try {
      window.__codexAuthPayloads.push({
        ...item,
        capturedAt: Date.now(),
      });
      if (window.__codexAuthPayloads.length > 30) {
        window.__codexAuthPayloads = window.__codexAuthPayloads.slice(-30);
      }
    } catch (e) {}
  };
  const origFetch = window.fetch;
  if (origFetch) {
    window.fetch = async function(...args) {
      const response = await origFetch.apply(this, args);
      try {
        const url = response && response.url ? response.url : (args[0] && args[0].url) || args[0];
        if (shouldCapture(url)) {
          const cloned = response.clone();
          let body = '';
          try { body = await cloned.text(); } catch (e) {}
          pushPayload({
            channel: 'fetch',
            url: String(url || ''),
            status: response.status,
            bodyPreview: String(body || '').replace(/\s+/g, ' ').slice(0, 1000),
          });
        }
      } catch (e) {}
      return response;
    };
  }
  const origOpen = XMLHttpRequest.prototype.open;
  const origSend = XMLHttpRequest.prototype.send;
  XMLHttpRequest.prototype.open = function(method, url, ...rest) {
    try {
      this.__codexCaptureUrl = url;
      this.__codexCaptureMethod = method;
    } catch (e) {}
    return origOpen.call(this, method, url, ...rest);
  };
  XMLHttpRequest.prototype.send = function(...args) {
    try {
      this.addEventListener('loadend', function() {
        try {
          const url = this.__codexCaptureUrl || this.responseURL;
          if (!shouldCapture(url)) return;
          const body = typeof this.responseText === 'string' ? this.responseText : '';
          pushPayload({
            channel: 'xhr',
            url: String(url || ''),
            status: this.status,
            bodyPreview: String(body || '').replace(/\s+/g, ' ').slice(0, 1000),
          });
        } catch (e) {}
      });
    } catch (e) {}
    return origSend.apply(this, args);
  };
})();
`
	return page.Context().AddInitScript(playwright.Script{Content: playwright.String(script)})
}

func getCapturedNetworkPayloads(page playwright.Page) []map[string]any {
	if page == nil {
		return nil
	}
	value, err := page.Evaluate(`() => Array.isArray(window.__codexAuthPayloads) ? window.__codexAuthPayloads.slice(-20) : []`, nil)
	if err != nil || value == nil {
		return nil
	}
	items, ok := value.([]interface{})
	if !ok {
		return nil
	}
	results := make([]map[string]any, 0, len(items))
	for _, item := range items {
		if payload, ok := item.(map[string]interface{}); ok {
			result := make(map[string]any, len(payload))
			for k, v := range payload {
				result[k] = v
			}
			results = append(results, result)
		}
	}
	return results
}

func loginURLForAccount(account Account) string {
	value := strings.TrimSpace(account.LoginURL)
	if value == "" {
		return "https://sellerhub.shein.com"
	}
	if strings.HasPrefix(value, "http://") || strings.HasPrefix(value, "https://") {
		return value
	}
	return "https://" + value
}

func defaultViewport(value, fallback int) int {
	if value > 0 {
		return value
	}
	return fallback
}

func waitForLoginSurface(ctx context.Context, page playwright.Page) error {
	deadline := time.Now().Add(45 * time.Second)
	for time.Now().Before(deadline) {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		if loggedIn, err := isLoggedIn(page); err == nil && loggedIn {
			return nil
		}
		if verifyRequired, err := isVerifyCodeRequired(page); err == nil && verifyRequired {
			return nil
		}
		states := collectSelectorStates(page)
		bodyText := ""
		if text, err := page.Locator("body").TextContent(); err == nil {
			bodyText = summarizeBodyText(text, 1200)
		}
		if hasLoginSurfaceSignals(states, bodyText) {
			return nil
		}
		time.Sleep(1 * time.Second)
	}
	return fmt.Errorf("login surface not ready")
}

func fillLogin(page playwright.Page, account Account) error {
	username, err := firstVisible(page, []string{
		"input.soui-input-input:first-of-type",
		`input.soui-input-input:not([type="password"])`,
		`input[type="text"].soui-input-input`,
		`input[type="text"]`,
	})
	if err != nil {
		return err
	}
	password, err := firstVisible(page, []string{
		`input[type="password"].soui-input-input`,
		`input[type="password"]`,
	})
	if err != nil {
		return err
	}
	if err := username.Click(); err != nil {
		return err
	}
	if err := username.Press("Control+A"); err != nil {
		return err
	}
	if err := username.Press("Backspace"); err != nil {
		return err
	}
	if err := username.Type(account.Username, playwright.LocatorTypeOptions{Delay: playwright.Float(60)}); err != nil {
		return err
	}
	if err := username.Press("Tab"); err != nil {
		return err
	}
	if err := password.Click(); err != nil {
		return err
	}
	if err := password.Press("Control+A"); err != nil {
		return err
	}
	if err := password.Press("Backspace"); err != nil {
		return err
	}
	if err := password.Type(account.Password, playwright.LocatorTypeOptions{Delay: playwright.Float(60)}); err != nil {
		return err
	}
	_ = password.Press("Tab")
	return nil
}

func submitLogin(page playwright.Page) error {
	button, err := firstVisible(page, []string{
		`button.soui-button-primary:has-text("登录")`,
		`button:has-text("登录")`,
		`button[type="submit"]`,
	})
	if err != nil {
		return err
	}
	if password, pwErr := firstVisible(page, []string{
		`input[type="password"].soui-input-input`,
		`input[type="password"]`,
	}); pwErr == nil {
		_ = password.Click()
		if err := password.Press("Enter"); err == nil {
			if advanced, waitErr := loginOutcomeAdvanced(page, 2*time.Second); waitErr == nil && advanced {
				return nil
			}
		}
	}
	if err := clickWithFallback(page, button); err == nil {
		if advanced, waitErr := loginOutcomeAdvanced(page, 2*time.Second); waitErr == nil && advanced {
			return nil
		}
	}
	if dismissed, dismissErr := dismissRequestFailure(page); dismissErr == nil && dismissed {
		if advanced, waitErr := loginOutcomeAdvanced(page, 2*time.Second); waitErr == nil && advanced {
			return nil
		}
	}
	if _, evalErr := button.Evaluate(`(el) => {
		el.dispatchEvent(new MouseEvent('mousedown', { bubbles: true }));
		el.dispatchEvent(new MouseEvent('mouseup', { bubbles: true }));
		el.dispatchEvent(new MouseEvent('click', { bubbles: true }));
		const form = el.closest('form');
		if (form) {
			if (typeof form.requestSubmit === 'function') {
				form.requestSubmit();
			} else {
				form.submit();
			}
		}
	}`, nil); evalErr == nil {
		if advanced, waitErr := loginOutcomeAdvanced(page, 2*time.Second); waitErr == nil && advanced {
			return nil
		}
	}
	return clickWithFallback(page, button)
}

func waitForLoginOutcome(ctx context.Context, page playwright.Page) (bool, error) {
	deadline := time.Now().Add(30 * time.Second)
	for time.Now().Before(deadline) {
		select {
		case <-ctx.Done():
			return false, ctx.Err()
		default:
		}
		if loggedIn, err := isLoggedIn(page); err == nil && loggedIn {
			return false, nil
		}
		if verifyRequired, err := isVerifyCodeRequired(page); err == nil && verifyRequired {
			return true, nil
		}
		if loginError, err := extractLoginError(page); err == nil && loginError != "" {
			return false, fmt.Errorf("%s", loginError)
		}
		if dismissed, _ := dismissRequestFailure(page); dismissed {
			continue
		}
		time.Sleep(1 * time.Second)
	}
	if loginError, err := extractLoginError(page); err == nil && loginError != "" {
		return false, fmt.Errorf("%s", loginError)
	}
	if shouldWaitForCaptcha(page) {
		return true, nil
	}
	return false, fmt.Errorf("login outcome timeout")
}

func loginOutcomeAdvanced(page playwright.Page, wait time.Duration) (bool, error) {
	deadline := time.Now().Add(wait)
	for time.Now().Before(deadline) {
		if loggedIn, err := isLoggedIn(page); err == nil && loggedIn {
			return true, nil
		}
		if verifyRequired, err := isVerifyCodeRequired(page); err == nil && verifyRequired {
			return true, nil
		}
		if loginError, err := extractLoginError(page); err == nil && loginError != "" {
			return true, nil
		}
		if dismissed, _ := dismissRequestFailure(page); dismissed {
			continue
		}
		time.Sleep(200 * time.Millisecond)
	}
	return false, nil
}

func settleAfterSubmit(page playwright.Page, wait time.Duration) {
	deadline := time.Now().Add(wait)
	for time.Now().Before(deadline) {
		if loggedIn, err := isLoggedIn(page); err == nil && loggedIn {
			return
		}
		if verifyRequired, err := isVerifyCodeRequired(page); err == nil && verifyRequired {
			return
		}
		if dismissed, _ := dismissRequestFailure(page); dismissed {
			continue
		}
		time.Sleep(200 * time.Millisecond)
	}
}

type playwrightVerifySession struct {
	account     Account
	manager     *sharedbrowser.Manager
	page        playwright.Page
	artifactDir string
	profileDir  string
}

func (s *playwrightVerifySession) SubmitCode(ctx context.Context, code string) (*AutomationResult, error) {
	if err := submitVerifyCode(s.page, code); err != nil {
		return artifactResult(s.page, s.artifactDir, s.account, "submit_verify_code", err)
	}
	waitCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	waiting, err := waitForLoginOutcome(waitCtx, s.page)
	if err != nil {
		return artifactResult(s.page, s.artifactDir, s.account, "wait_verify_code", err)
	}
	if waiting {
		result, _ := artifactResult(s.page, s.artifactDir, s.account, "wait_verify_code", fmt.Errorf("验证码提交后仍需继续验证"))
		if result == nil {
			return &AutomationResult{
				WaitingForVerifyCode: true,
				ErrorCode:            "VERIFY_CODE_REQUIRED",
				ErrorMessage:         "验证码提交后仍需继续验证",
			}, nil
		}
		result.WaitingForVerifyCode = true
		if strings.TrimSpace(result.ErrorCode) == "" {
			result.ErrorCode = "VERIFY_CODE_REQUIRED"
		}
		if strings.TrimSpace(result.ErrorMessage) == "" {
			result.ErrorMessage = "验证码提交后仍需继续验证"
		}
		if result.FailureSummary != nil {
			result.FailureSummary.WaitingForVerifyCode = true
			if strings.TrimSpace(result.FailureSummary.ErrorCode) == "" {
				result.FailureSummary.ErrorCode = "VERIFY_CODE_REQUIRED"
			}
			if strings.TrimSpace(result.FailureSummary.ErrorMessage) == "" {
				result.FailureSummary.ErrorMessage = "验证码提交后仍需继续验证"
			}
		}
		return result, nil
	}
	storageState, err := s.manager.GetContext().StorageState()
	if err != nil {
		return artifactResult(s.page, s.artifactDir, s.account, "export_state_after_verify", err)
	}
	return &AutomationResult{
		BrowserState: cookieOnlyBrowserState(map[string]any{"cookies": storageState.Cookies}),
	}, nil
}

func (s *playwrightVerifySession) Close() error {
	if s.manager != nil {
		closeManagerProfile(s.manager, s.profileDir)
	}
	return nil
}

func launchManagerWithProfileRecovery(manager *sharedbrowser.Manager, profileDir string) error {
	err := manager.Launch()
	if err == nil {
		return nil
	}
	if !isProfileInUseError(err) {
		return err
	}
	terminateProfileBrowserProcesses(profileDir)
	cleared := clearProfileLockFiles(profileDir)
	if !cleared {
		return fmt.Errorf("SHEIN 浏览器 profile 正在使用，请稍后重试或关闭当前登录窗口: %w", err)
	}
	if retryErr := manager.Launch(); retryErr != nil {
		if isProfileInUseError(retryErr) {
			return fmt.Errorf("SHEIN 浏览器 profile 正在使用，请稍后重试或关闭当前登录窗口: %w", retryErr)
		}
		return retryErr
	}
	return nil
}

func closeManagerProfile(manager *sharedbrowser.Manager, profileDir string) {
	if manager != nil {
		manager.Close()
	}
	trimProfileDir(profileDir)
}

func submitVerifyCode(page playwright.Page, code string) error {
	input, err := firstVisible(page, []string{
		"#verifyCode",
		`input[placeholder*="验证码"]`,
		`input[autocomplete="one-time-code"]`,
		`input[inputmode="numeric"]`,
	})
	if err != nil {
		return err
	}
	if err := input.Click(); err != nil {
		return err
	}
	if err := input.Press("Control+A"); err != nil {
		return err
	}
	if err := input.Press("Backspace"); err != nil {
		return err
	}
	if err := input.Type(code, playwright.LocatorTypeOptions{Delay: playwright.Float(80)}); err != nil {
		return err
	}
	_ = input.Press("Tab")
	button, err := firstVisible(page, []string{
		`button.soui-button-primary:has-text("确认")`,
		`button:has-text("确认")`,
		`button:has-text("提交")`,
		`button[type="submit"]`,
	})
	if err != nil {
		return err
	}
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		disabled, disabledErr := button.IsDisabled()
		if disabledErr == nil && !disabled {
			break
		}
		time.Sleep(200 * time.Millisecond)
	}
	return clickWithFallback(page, button)
}

func firstVisible(page playwright.Page, selectors []string) (playwright.Locator, error) {
	for _, selector := range selectors {
		loc := page.Locator(selector).First()
		visible, err := loc.IsVisible()
		if err == nil && visible {
			return loc, nil
		}
	}
	return nil, fmt.Errorf("visible element not found")
}

func isLoggedIn(page playwright.Page) (bool, error) {
	for _, selector := range []string{
		".dashboard",
		".main-content",
		`div:has-text("卖家中心")`,
		`div:has-text("Seller Hub")`,
	} {
		ok, err := page.Locator(selector).First().IsVisible()
		if err == nil && ok {
			return true, nil
		}
	}
	return false, nil
}

func isVerifyCodeRequired(page playwright.Page) (bool, error) {
	for _, selector := range []string{
		"#verifyCode",
		`input[placeholder*="验证码"]`,
		`input[autocomplete="one-time-code"]`,
		`input[inputmode="numeric"]`,
		`button:has-text("发送至邮箱")`,
	} {
		ok, err := page.Locator(selector).First().IsVisible()
		if err == nil && ok {
			return true, nil
		}
	}
	return false, nil
}

func dismissRequestFailure(page playwright.Page) (bool, error) {
	for _, selector := range []string{
		`button:has-text("确定")`,
		`[role="dialog"] button:has-text("确定")`,
		`button:has-text("刷新")`,
	} {
		button := page.Locator(selector).First()
		visible, err := button.IsVisible()
		if err != nil || !visible {
			continue
		}
		text, _ := page.Locator("body").TextContent()
		if !strings.Contains(normalizeText(text), "请求失败") {
			continue
		}
		if err := clickWithFallback(page, button); err != nil {
			return false, err
		}
		return true, nil
	}
	return false, nil
}

func clickWithFallback(page playwright.Page, loc playwright.Locator) error {
	if err := loc.Click(playwright.LocatorClickOptions{Timeout: playwright.Float(5000)}); err == nil {
		return nil
	}
	if loggedIn, _ := isLoggedIn(page); loggedIn {
		return nil
	}
	if err := loc.Click(playwright.LocatorClickOptions{Timeout: playwright.Float(5000), Force: playwright.Bool(true)}); err == nil {
		return nil
	}
	_, evalErr := loc.Evaluate("(el) => el.click()", nil)
	if evalErr == nil {
		return nil
	}
	return evalErr
}

func extractLoginError(page playwright.Page) (string, error) {
	candidates := make([]string, 0, len(sheinLoginErrorSelectors)+1)
	for _, selector := range sheinLoginErrorSelectors {
		loc := page.Locator(selector).First()
		visible, err := loc.IsVisible()
		if err != nil || !visible {
			continue
		}
		text, err := loc.TextContent()
		if err == nil && strings.TrimSpace(text) != "" {
			candidates = append(candidates, text)
		}
	}
	if body, err := page.Locator("body").TextContent(); err == nil && strings.TrimSpace(body) != "" {
		candidates = append(candidates, body)
	}
	return detectLoginErrorText(candidates...), nil
}

func detectLoginErrorText(candidates ...string) string {
	for _, raw := range candidates {
		normalized := normalizeText(raw)
		if normalized == "" {
			continue
		}
		for _, keyword := range []string{
			"账号或密码错误",
			"用户名或密码错误",
			"用户名密码错误",
			"账号密码错误",
			"账号未启用",
			"您输入的账号或绑定信息不正确或账号未启用",
			"请联系主账号在系统账号管理页面为您设置角色权限",
			"子账号无签署权限需主账号",
			"请求失败尝试刷新页面或联系开发",
			"请求失败",
			"otp码",
			"请输入已发送至您手机的otp码以验证身份",
			"password error",
		} {
			if strings.Contains(normalized, normalizeText(keyword)) {
				return raw
			}
		}
	}
	return ""
}

func classifyLoginError(message string) string {
	return classifyLoginErrorText(message)
}

func classifyLoginErrorText(message string) string {
	normalized := normalizeText(message)
	switch {
	case normalized == "":
		return "LOGIN_FAILED"
	case strings.Contains(normalized, "otp") ||
		strings.Contains(normalized, normalizeText("otp码")) ||
		strings.Contains(normalized, normalizeText("请输入已发送至您手机的otp码以验证身份")) ||
		strings.Contains(normalized, normalizeText("已发送验证码")):
		return "VERIFY_CODE_REQUIRED"
	case strings.Contains(normalized, normalizeText("账号或密码错误")) ||
		strings.Contains(normalized, normalizeText("用户名或密码错误")) ||
		strings.Contains(normalized, normalizeText("用户名密码错误")) ||
		strings.Contains(normalized, normalizeText("账号密码错误")) ||
		strings.Contains(normalized, normalizeText("账号未启用")) ||
		strings.Contains(normalized, normalizeText("password error")):
		return "INVALID_CREDENTIALS"
	case strings.Contains(normalized, normalizeText("请联系主账号在系统账号管理页面为您设置角色权限")):
		return "ROLE_PERMISSION_REQUIRED"
	case strings.Contains(normalized, normalizeText("子账号无签署权限")):
		return "SIGN_PERMISSION_REQUIRED"
	case strings.Contains(normalized, normalizeText("请求失败")):
		return "REQUEST_FAILED"
	default:
		return "LOGIN_FAILED"
	}
}

func classifyLoginFailure(metadata artifactMetadata) string {
	switch {
	case metadata.VerifyCodeVisible != nil && *metadata.VerifyCodeVisible:
		return "VERIFY_CODE_REQUIRED"
	case metadata.VerificationVisible != nil && *metadata.VerificationVisible:
		return "VERIFY_CODE_REQUIRED"
	case metadata.CredentialErrorVisible != nil && *metadata.CredentialErrorVisible:
		return "INVALID_CREDENTIALS"
	case metadata.AgreementVisible != nil && *metadata.AgreementVisible:
		return "SIGN_PERMISSION_REQUIRED"
	case metadata.PermissionVisible != nil && *metadata.PermissionVisible:
		return "ROLE_PERMISSION_REQUIRED"
	case metadata.RequestFailureModal != nil && *metadata.RequestFailureModal:
		return "REQUEST_FAILED"
	}

	for _, candidate := range []string{metadata.LoginError, metadata.Error, metadata.BodyText, metadata.Title, metadata.URL} {
		if code := classifyLoginErrorText(candidate); code != "LOGIN_FAILED" {
			return code
		}
	}
	return "LOGIN_FAILED"
}

func derivePageState(metadata artifactMetadata) string {
	switch {
	case metadata.LoggedIn != nil && *metadata.LoggedIn:
		return "seller_hub"
	case metadata.SellerHubVisible != nil && *metadata.SellerHubVisible:
		return "seller_hub"
	case metadata.VerifyCodeVisible != nil && *metadata.VerifyCodeVisible:
		return "verification"
	case metadata.VerificationVisible != nil && *metadata.VerificationVisible:
		return "verification"
	case metadata.CredentialErrorVisible != nil && *metadata.CredentialErrorVisible:
		return "credential_error"
	case metadata.AgreementVisible != nil && *metadata.AgreementVisible:
		return "agreement_gate"
	case metadata.PermissionVisible != nil && *metadata.PermissionVisible:
		return "permission_gate"
	case metadata.RequestFailureModal != nil && *metadata.RequestFailureModal:
		return "request_failure"
	case metadata.LoginFormVisible != nil && *metadata.LoginFormVisible:
		return "login_form"
	case metadata.OnLoginPage != nil && *metadata.OnLoginPage:
		return "login_form"
	default:
		return "unknown"
	}
}

func deriveFailureAction(pageState string, waitingForVerifyCode bool, errorCode string) (string, string) {
	switch {
	case waitingForVerifyCode || pageState == "verification" || errorCode == "VERIFY_CODE_REQUIRED":
		return "submit_verify_code", "提交验证码并继续当前登录会话"
	case pageState == "agreement_gate" || errorCode == "SIGN_PERMISSION_REQUIRED":
		return "use_master_account", "切换主账号完成协议签署后再重试登录"
	case pageState == "permission_gate" || errorCode == "ROLE_PERMISSION_REQUIRED":
		return "fix_account_permission", "联系主账号在 SHEIN 后台补齐角色权限后再重试"
	case pageState == "credential_error" || errorCode == "INVALID_CREDENTIALS":
		return "check_credentials", "核对账号密码或账号启用状态后重新登录"
	case pageState == "request_failure" || errorCode == "REQUEST_FAILED":
		return "retry_login", "重试登录；若持续失败，检查网络、代理和页面弹层"
	case pageState == "login_form":
		return "retry_login", "重新触发登录并观察是否进入验证码或权限页面"
	case pageState == "seller_hub":
		return "check_cookie_persistence", "页面已进入卖家中心，优先检查 Cookie 持久化和状态导出"
	default:
		return "inspect_artifact", "查看失败详情和 artifact，确认当前页面分支后再处理"
	}
}

func normalizeText(value string) string {
	trimmed := strings.TrimSpace(strings.ToLower(value))
	if trimmed == "" {
		return ""
	}
	replacer := strings.NewReplacer(
		" ", "",
		"\n", "",
		"\r", "",
		"\t", "",
		"，", ",",
		"。", ".",
		"：", ":",
		"【", "",
		"】", "",
	)
	return replacer.Replace(trimmed)
}

func artifactResult(page playwright.Page, root string, account Account, stage string, cause error) (*AutomationResult, error) {
	if strings.TrimSpace(root) == "" {
		return nil, cause
	}
	dir := filepath.Join(root, fmt.Sprintf("%d_%d_%s_%d", account.TenantID, account.StoreID, stage, time.Now().Unix()))
	_ = os.MkdirAll(dir, 0o755)
	metadata := collectArtifactMetadata(page, account, stage, cause)
	if bytes, err := page.Screenshot(playwright.PageScreenshotOptions{FullPage: playwright.Bool(true)}); err == nil {
		_ = os.WriteFile(filepath.Join(dir, "page.png"), bytes, 0o644)
	}
	if html, err := page.Content(); err == nil {
		_ = os.WriteFile(filepath.Join(dir, "page.html"), []byte(html), 0o644)
	}
	if payload, err := json.MarshalIndent(metadata, "", "  "); err == nil {
		_ = os.WriteFile(filepath.Join(dir, "metadata.json"), payload, 0o644)
	}
	errorCode := classifyLoginFailure(metadata)
	actionKey, actionMessage := deriveFailureAction(metadata.PageState, metadata.VerifyCodeVisible != nil && *metadata.VerifyCodeVisible, errorCode)
	return &AutomationResult{
		ErrorCode:           errorCode,
		ErrorMessage:        cause.Error(),
		FailureArtifactPath: dir,
		FailureSummary: &FailureSummary{
			ErrorCode:            metadata.ErrorCode,
			ErrorMessage:         metadata.Error,
			PageState:            metadata.PageState,
			ActionKey:            actionKey,
			ActionMessage:        actionMessage,
			ArtifactPath:         dir,
			CapturedAt:           capturedAtFromMetadata(metadata),
			Stage:                metadata.Stage,
			URL:                  metadata.URL,
			Title:                metadata.Title,
			LoginError:           metadata.LoginError,
			WaitingForVerifyCode: metadata.VerifyCodeVisible != nil && *metadata.VerifyCodeVisible,
		},
	}, nil
}

func collectArtifactMetadata(page playwright.Page, account Account, stage string, cause error) artifactMetadata {
	metadata := artifactMetadata{
		TenantID:   account.TenantID,
		StoreID:    account.StoreID,
		Username:   account.Username,
		Stage:      stage,
		Error:      cause.Error(),
		CapturedAt: time.Now().Format(time.RFC3339),
	}
	if page == nil {
		metadata.ErrorCode = classifyLoginFailure(metadata)
		return metadata
	}
	if url := strings.TrimSpace(page.URL()); url != "" {
		metadata.URL = url
	}
	if title, err := page.Title(); err == nil {
		metadata.Title = strings.TrimSpace(title)
	}
	if loggedIn, err := isLoggedIn(page); err == nil {
		metadata.LoggedIn = &loggedIn
	}
	if verifyRequired, err := isVerifyCodeRequired(page); err == nil {
		metadata.VerifyCodeVisible = &verifyRequired
	}
	if onLoginPage, err := isOnLoginPage(page); err == nil {
		metadata.OnLoginPage = &onLoginPage
	}
	if hasModal, err := hasRequestFailureModal(page); err == nil {
		metadata.RequestFailureModal = &hasModal
	}
	if loginError, err := extractLoginError(page); err == nil {
		metadata.LoginError = strings.TrimSpace(loginError)
	}
	if bodyText, err := page.Locator("body").TextContent(); err == nil {
		metadata.BodyText = summarizeBodyText(bodyText, 4000)
	}
	if payloads := getCapturedNetworkPayloads(page); len(payloads) > 0 {
		metadata.NetworkPayloads = payloads
	}
	metadata.SelectorStates = collectSelectorStates(page)
	loginFormVisible, sellerHubVisible, verificationVisible, permissionVisible, agreementVisible, credentialErrorVisible := deriveBusinessVisibility(metadata.SelectorStates)
	metadata.LoginFormVisible = &loginFormVisible
	metadata.SellerHubVisible = &sellerHubVisible
	metadata.VerificationVisible = &verificationVisible
	metadata.PermissionVisible = &permissionVisible
	metadata.AgreementVisible = &agreementVisible
	metadata.CredentialErrorVisible = &credentialErrorVisible
	metadata.PageState = derivePageState(metadata)
	metadata.ErrorCode = classifyLoginFailure(metadata)
	metadata.ActionKey, metadata.ActionMessage = deriveFailureAction(metadata.PageState, metadata.VerifyCodeVisible != nil && *metadata.VerifyCodeVisible, metadata.ErrorCode)
	return metadata
}

func summarizeBodyText(value string, maxChars int) string {
	normalized := strings.Join(strings.Fields(strings.TrimSpace(value)), " ")
	if normalized == "" {
		return ""
	}
	if maxChars <= 0 || len(normalized) <= maxChars {
		return normalized
	}
	return normalized[:maxChars]
}

func isOnLoginPage(page playwright.Page) (bool, error) {
	currentURL := strings.ToLower(strings.TrimSpace(page.URL()))
	if currentURL == "" {
		return false, nil
	}
	return strings.Contains(currentURL, "login"), nil
}

func hasRequestFailureModal(page playwright.Page) (bool, error) {
	bodyText, err := page.Locator("body").TextContent()
	if err != nil || !strings.Contains(normalizeText(bodyText), "请求失败") {
		return false, err
	}
	for _, selector := range []string{
		`button:has-text("确定")`,
		`[role="dialog"] button:has-text("确定")`,
		`button:has-text("刷新")`,
	} {
		visible, visibleErr := page.Locator(selector).First().IsVisible()
		if visibleErr == nil && visible {
			return true, nil
		}
	}
	return false, nil
}

func collectSelectorStates(page playwright.Page) map[string]bool {
	states := map[string]bool{}
	selectors := map[string]string{
		"username_input":           `input.soui-input-input:not([type="password"])`,
		"password_input":           `input[type="password"]`,
		"login_button":             `button:has-text("登录")`,
		"captcha_iframe":           `iframe[src*="captcha"], iframe[src*="geetest"]`,
		"captcha_container":        `[class*="geetest"], [id*="captcha"], [class*="captcha"]`,
		"verify_code_input":        `#verifyCode`,
		"verify_send_email_button": `button:has-text("发送至邮箱")`,
		"verify_confirm_button":    `button:has-text("确认")`,
		"request_fail_ok":          `button:has-text("确定")`,
		"request_fail_retry":       `button:has-text("刷新")`,
		"seller_hub_text":          `div:has-text("Seller Hub")`,
		"seller_hub_cn_text":       `div:has-text("卖家中心")`,
		"permission_text":          `text=/未授权|没有已授权的系统权限|角色权限|请联系主账号/i`,
		"permission_dialog_button": `button:has-text("账号管理")`,
		"agreement_text":           `text=/签署协议|我已阅读并同意|签署此协议后才可访问系统|子账号无签署权限/i`,
		"agreement_checkbox":       `input[type="checkbox"]`,
		"agreement_confirm_button": `button:has-text("同意")`,
		"agreement_sign_button":    `button:has-text("签署")`,
		"credential_error_text":    `text=/账号或密码错误|用户名或密码错误|账号未启用|password error|invalid credentials/i`,
		"credential_error_inline":  `.soui-form-error`,
		"credential_error_input":   `.soui-input-error`,
		"credential_error_alert":   `[role="alert"]`,
	}
	for key, selector := range selectors {
		visible, err := page.Locator(selector).First().IsVisible()
		states[key] = err == nil && visible
	}
	return states
}

func shouldWaitForCaptcha(page playwright.Page) bool {
	states := collectSelectorStates(page)
	if states["verify_code_input"] || states["verify_send_email_button"] || states["verify_confirm_button"] {
		return true
	}
	if states["captcha_iframe"] || states["captcha_container"] {
		return true
	}
	bodyText := ""
	if text, err := page.Locator("body").TextContent(); err == nil {
		bodyText = normalizeText(text)
	}
	for _, keyword := range []string{
		"验证码",
		"校验",
		"滑块",
		"人机",
		"请勿频繁点击",
		"稍后重试",
		"captcha",
		"geetest",
	} {
		if strings.Contains(bodyText, normalizeText(keyword)) {
			return true
		}
	}
	return false
}

func deriveBusinessVisibility(selectorStates map[string]bool) (loginFormVisible, sellerHubVisible, verificationVisible, permissionVisible, agreementVisible, credentialErrorVisible bool) {
	loginFormVisible = selectorStates["username_input"] || selectorStates["password_input"] || selectorStates["login_button"]
	sellerHubVisible = selectorStates["seller_hub_text"] || selectorStates["seller_hub_cn_text"]
	verificationVisible = selectorStates["verify_code_input"] || selectorStates["verify_send_email_button"] || selectorStates["verify_confirm_button"]
	permissionVisible = selectorStates["permission_text"] || selectorStates["permission_dialog_button"]
	agreementVisible = selectorStates["agreement_text"] || selectorStates["agreement_checkbox"] || selectorStates["agreement_confirm_button"] || selectorStates["agreement_sign_button"]
	credentialErrorVisible = selectorStates["credential_error_text"] || selectorStates["credential_error_inline"] || selectorStates["credential_error_input"] || selectorStates["credential_error_alert"]
	return loginFormVisible, sellerHubVisible, verificationVisible, permissionVisible, agreementVisible, credentialErrorVisible
}

func hasLoginSurfaceSignals(selectorStates map[string]bool, bodyText string) bool {
	loginFormVisible, sellerHubVisible, verificationVisible, permissionVisible, agreementVisible, credentialErrorVisible := deriveBusinessVisibility(selectorStates)
	if loginFormVisible || sellerHubVisible || verificationVisible || permissionVisible || agreementVisible || credentialErrorVisible {
		return true
	}
	normalized := normalizeText(bodyText)
	if normalized == "" {
		return false
	}
	keywords := []string{
		"登录",
		"手机号",
		"账号",
		"密码",
		"验证码",
		"卖家中心",
		"seller hub",
		"商家中心",
	}
	for _, keyword := range keywords {
		if strings.Contains(normalized, normalizeText(keyword)) {
			return true
		}
	}
	return false
}

func capturedAtFromMetadata(metadata artifactMetadata) time.Time {
	if metadata.CapturedAt == "" {
		return time.Time{}
	}
	when, err := time.Parse(time.RFC3339, metadata.CapturedAt)
	if err != nil {
		return time.Time{}
	}
	return when
}
