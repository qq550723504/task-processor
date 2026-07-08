package sdslogin

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"

	"task-processor/internal/core/logger"
	sharedbrowser "task-processor/internal/crawler/shared/browser"

	"github.com/mxschmitt/playwright-go"
)

type browserRunConfig struct {
	Headless          bool
	ProfileRoot       string
	ArtifactDir       string
	BrowserPath       string
	ChromeVersion     string
	ChromeDownloadDir string
	ViewportWidth     int
	ViewportHeight    int
	LoginURL          string
	TargetURL         string
	WaitTimeout       time.Duration
	UseCloakBrowser   bool
}

type browserEventTracer struct {
	account configuredAccount
	mu      sync.Mutex
	nextID  int
	pageIDs map[playwright.Page]int
	log     interface {
		Infof(string, ...any)
		Warnf(string, ...any)
	}
}

func runBrowserLogin(ctx context.Context, account configuredAccount, cfg browserRunConfig) (*AuthPayload, bool, error) {
	startedAt := time.Now()
	profileDir, err := resolveProfileDir(cfg.ProfileRoot, account)
	if err != nil {
		return nil, false, err
	}
	if err := os.MkdirAll(profileDir, 0o755); err != nil {
		return nil, false, err
	}
	staleProcesses := terminateProfileBrowserProcesses(profileDir)
	staleLocks := clearProfileLockFiles(profileDir)
	log := logger.GetGlobalLogger("sdslogin/browser")
	log.Infof("sds login starting tenant=%s identifier=%s profile_dir=%s headless=%t", account.TenantID, account.Identifier, profileDir, cfg.Headless)
	log.Infof(
		"sds login config tenant=%s identifier=%s browser_path=%s cloak_enabled=%t artifact_dir=%s chrome_download_dir=%s wait_timeout=%s",
		account.TenantID,
		account.Identifier,
		strings.TrimSpace(cfg.BrowserPath),
		cfg.UseCloakBrowser,
		strings.TrimSpace(cfg.ArtifactDir),
		strings.TrimSpace(cfg.ChromeDownloadDir),
		cfg.WaitTimeout,
	)
	if staleProcesses > 0 || staleLocks {
		log.Infof("sds login profile preflight tenant=%s identifier=%s killed_processes=%d cleared_locks=%t", account.TenantID, account.Identifier, staleProcesses, staleLocks)
	}

	managerCfg := &sharedbrowser.BrowserConfig{
		Headless:          cfg.Headless,
		BrowserPath:       strings.TrimSpace(cfg.BrowserPath),
		ChromeVersion:     strings.TrimSpace(cfg.ChromeVersion),
		ChromeDownloadDir: strings.TrimSpace(cfg.ChromeDownloadDir),
		ViewportWidth:     defaultViewport(cfg.ViewportWidth, 1440),
		ViewportHeight:    defaultViewport(cfg.ViewportHeight, 960),
		FingerprintSeed:   stableFingerprintSeed(account.Identifier),
		Language:          "zh-CN",
		AcceptLanguage:    "zh-CN,zh;q=0.9,en;q=0.8",
		StealthProvider:   sharedbrowser.StealthProviderDefault,
	}
	if cfg.UseCloakBrowser {
		managerCfg.StealthProvider = sharedbrowser.StealthProviderCloakBrowser
		managerCfg.FingerprintSeed = 0
		managerCfg.ChromeVersion = ""
		managerCfg.ChromeDownloadDir = ""
	}
	manager := sharedbrowser.NewManager(managerCfg)
	manager.SetUserDataDir(profileDir)
	if !cfg.UseCloakBrowser {
		manager.SetFingerprint(manager.GenerateStableFingerprint(account.Identifier))
	}
	log.Infof("sds login installing browser tenant=%s identifier=%s", account.TenantID, account.Identifier)
	if err := manager.Install(); err != nil {
		log.WithError(err).Warnf("sds login browser install failed tenant=%s identifier=%s", account.TenantID, account.Identifier)
		return nil, false, err
	}
	log.Infof("sds login browser install completed tenant=%s identifier=%s elapsed=%s", account.TenantID, account.Identifier, time.Since(startedAt))
	log.Infof("sds login launching browser tenant=%s identifier=%s", account.TenantID, account.Identifier)
	if err := launchManagerWithProfileRecovery(manager, profileDir); err != nil {
		log.WithError(err).Warnf("sds login browser launch failed tenant=%s identifier=%s", account.TenantID, account.Identifier)
		manager.Close()
		return nil, false, err
	}
	log.Infof("sds login browser launched tenant=%s identifier=%s elapsed=%s", account.TenantID, account.Identifier, time.Since(startedAt))
	defer closeManagerWithTimeout(log, manager, profileDir)
	tracer := newBrowserEventTracer(account)
	tracer.attachContext(manager.GetContext())

	log.Infof("sds login acquiring page tenant=%s identifier=%s", account.TenantID, account.Identifier)
	page, err := acquireLoginPage(ctx, manager)
	if err != nil {
		log.WithError(err).Warnf("sds login acquire page failed tenant=%s identifier=%s", account.TenantID, account.Identifier)
		return nil, false, err
	}
	log.Infof("sds login page acquired tenant=%s identifier=%s elapsed=%s", account.TenantID, account.Identifier, time.Since(startedAt))
	tracer.attachPage(page, "login-root")
	defer closePageWithTimeout(log, page)

	loginURL := strings.TrimSpace(cfg.LoginURL)
	if loginURL == "" {
		loginURL = "https://www.sdsdiy.com/user/login?redirect=%2Fadmin%2Fmaterial"
	}
	log.Infof("sds login navigating tenant=%s identifier=%s url=%s", account.TenantID, account.Identifier, loginURL)
	if err = gotoWithTimeout(ctx, page, loginURL, 90*time.Second); err != nil {
		log.WithError(err).Warnf("sds login goto failed tenant=%s identifier=%s url=%s", account.TenantID, account.Identifier, loginURL)
		return nil, false, err
	}
	log.Infof("sds login page loaded tenant=%s identifier=%s url=%s elapsed=%s", account.TenantID, account.Identifier, page.URL(), time.Since(startedAt))

	log.Infof("sds login prefilling form tenant=%s identifier=%s", account.TenantID, account.Identifier)
	if err := prefillLoginForm(page, account); err != nil {
		log.WithError(err).Warnf("sds login prefill failed tenant=%s identifier=%s", account.TenantID, account.Identifier)
		return nil, false, err
	}
	log.Infof("sds login form prefilled tenant=%s identifier=%s elapsed=%s", account.TenantID, account.Identifier, time.Since(startedAt))
	log.Infof("sds login clicking submit tenant=%s identifier=%s", account.TenantID, account.Identifier)
	clicked, clickErr := clickLoginIfPossible(page)
	if clickErr != nil {
		log.WithError(clickErr).Warnf("sds login click failed tenant=%s identifier=%s", account.TenantID, account.Identifier)
	} else {
		log.Infof("sds login click attempted tenant=%s identifier=%s clicked=%t elapsed=%s", account.TenantID, account.Identifier, clicked, time.Since(startedAt))
	}

	waitTimeout := cfg.WaitTimeout
	if waitTimeout <= 0 {
		waitTimeout = 30 * time.Second
	}
	deadline := time.Now().Add(waitTimeout)
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()
	var lastState *pageLoginState
	lastHref := ""
	lastTokenPresent := false
	lastMerchantID := int64(0)
	for time.Now().Before(deadline) {
		select {
		case <-ctx.Done():
			return nil, false, ctx.Err()
		case <-ticker.C:
		}
		state, err := readCurrentLoginState(manager, page)
		if err == nil {
			lastState = state
			tokenPresent := strings.TrimSpace(state.Token) != ""
			if state.Href != lastHref || tokenPresent != lastTokenPresent || state.MerchantID != lastMerchantID {
				log.Infof(
					"sds login poll tenant=%s identifier=%s url=%s token_present=%t merchant_id=%d user_id=%d",
					account.TenantID,
					account.Identifier,
					state.Href,
					tokenPresent,
					state.MerchantID,
					state.UserID,
				)
				lastHref = state.Href
				lastTokenPresent = tokenPresent
				lastMerchantID = state.MerchantID
			}
		}
		if err == nil && hasUsableSessionMarkers(state) {
			browserState, stateErr := captureBrowserState(manager)
			if stateErr == nil {
				state.BrowserState = browserState
			}
		}
		if err == nil && hasUsableLoginState(state) {
			log.Infof("sds login completed tenant=%s identifier=%s url=%s merchant_id=%d user_id=%d elapsed=%s", account.TenantID, account.Identifier, state.Href, state.MerchantID, state.UserID, time.Since(startedAt))
			return buildPayload(account, state, "fresh_login"), false, nil
		}
	}

	log.Warnf(
		"sds login timed out tenant=%s identifier=%s final_url=%s title=%s body=%s",
		account.TenantID,
		account.Identifier,
		lastHref,
		readPageTitle(page),
		summarizeText(readPageText(page), 600),
	)
	waiting, classifyErr := classifyLoginFailure(page, lastState)
	return nil, waiting, classifyErr
}

func newBrowserEventTracer(account configuredAccount) *browserEventTracer {
	return &browserEventTracer{
		account: account,
		pageIDs: map[playwright.Page]int{},
		log:     logger.GetGlobalLogger("sdslogin/browser"),
	}
}

func (t *browserEventTracer) attachContext(context playwright.BrowserContext) {
	if context == nil {
		return
	}
	t.log.Infof("sds login context attached tenant=%s identifier=%s existing_pages=%d", t.account.TenantID, t.account.Identifier, len(context.Pages()))
	for _, page := range context.Pages() {
		t.attachPage(page, "existing")
	}
	context.OnPage(func(page playwright.Page) {
		t.attachPage(page, "context")
	})
	context.OnClose(func(playwright.BrowserContext) {
		t.log.Infof("sds login context closed tenant=%s identifier=%s", t.account.TenantID, t.account.Identifier)
	})
}

func (t *browserEventTracer) attachPage(page playwright.Page, source string) {
	if page == nil {
		return
	}
	id, isNew := t.pageID(page)
	if isNew {
		t.log.Infof("sds login page opened tenant=%s identifier=%s page_id=%d source=%s url=%s", t.account.TenantID, t.account.Identifier, id, source, page.URL())
	}
	page.OnPopup(func(popup playwright.Page) {
		popupID, _ := t.pageID(popup)
		t.log.Infof("sds login popup opened tenant=%s identifier=%s parent_page_id=%d popup_page_id=%d parent_url=%s popup_url=%s", t.account.TenantID, t.account.Identifier, id, popupID, page.URL(), popup.URL())
		t.attachPage(popup, "popup")
	})
	page.OnClose(func(closed playwright.Page) {
		title := ""
		if value, err := closed.Title(); err == nil {
			title = strings.TrimSpace(value)
		}
		t.log.Infof("sds login page closed tenant=%s identifier=%s page_id=%d url=%s title=%s", t.account.TenantID, t.account.Identifier, id, closed.URL(), title)
		t.removePage(closed)
	})
}

func (t *browserEventTracer) pageID(page playwright.Page) (int, bool) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if id, ok := t.pageIDs[page]; ok {
		return id, false
	}
	t.nextID++
	id := t.nextID
	t.pageIDs[page] = id
	return id, true
}

func (t *browserEventTracer) removePage(page playwright.Page) {
	t.mu.Lock()
	defer t.mu.Unlock()
	delete(t.pageIDs, page)
}

func resolveProfileDir(profileRoot string, account configuredAccount) (string, error) {
	root := strings.TrimSpace(profileRoot)
	if root == "" {
		root = filepath.Join(".", ".local", "tmp", "browser-profiles", "sds")
	}
	profileDir := filepath.Join(root, sanitizePathSegment(account.TenantID), sanitizePathSegment(account.Identifier))
	if !filepath.IsAbs(profileDir) {
		absDir, err := filepath.Abs(profileDir)
		if err != nil {
			return "", err
		}
		profileDir = absDir
	}
	return profileDir, nil
}

func acquireLoginPage(ctx context.Context, manager *sharedbrowser.Manager) (playwright.Page, error) {
	if manager == nil || manager.GetContext() == nil {
		return nil, fmt.Errorf("browser context is not initialized")
	}
	type newPageResult struct {
		page playwright.Page
		err  error
	}
	resultCh := make(chan newPageResult, 1)
	go func() {
		page, err := manager.NewPage()
		resultCh <- newPageResult{page: page, err: err}
	}()
	select {
	case result := <-resultCh:
		if result.err == nil && result.page != nil {
			return result.page, nil
		}
	case <-ctx.Done():
		return nil, fmt.Errorf("create SDS login page canceled: %w", ctx.Err())
	case <-time.After(15 * time.Second):
		return nil, fmt.Errorf("create SDS login page timed out, websocket may be stalled")
	}
	pages := manager.GetContext().Pages()
	if len(pages) > 0 && pages[0] != nil {
		return pages[0], nil
	}
	return nil, fmt.Errorf("browser context did not provide a usable page")
}

func gotoWithTimeout(ctx context.Context, page playwright.Page, url string, timeout time.Duration) error {
	type gotoResult struct {
		err error
	}
	resultCh := make(chan gotoResult, 1)
	go func() {
		_, err := page.Goto(url, playwright.PageGotoOptions{
			WaitUntil: playwright.WaitUntilStateDomcontentloaded,
			Timeout:   playwright.Float(float64(timeout.Milliseconds())),
		})
		resultCh <- gotoResult{err: err}
	}()
	select {
	case result := <-resultCh:
		return result.err
	case <-ctx.Done():
		return fmt.Errorf("navigate SDS login page canceled: %w", ctx.Err())
	case <-time.After(timeout + 5*time.Second):
		return fmt.Errorf("navigate SDS login page timed out, websocket may be stalled")
	}
}

func closePageWithTimeout(log interface{ Warnf(string, ...any) }, page playwright.Page) {
	if page == nil {
		return
	}
	done := make(chan error, 1)
	go func() {
		done <- page.Close()
	}()
	select {
	case err := <-done:
		if err != nil {
			log.Warnf("sds login page close failed: %v", err)
		}
	case <-time.After(5 * time.Second):
		log.Warnf("sds login page close timed out, websocket may be stalled")
	}
}

func closeManagerWithTimeout(log interface{ Warnf(string, ...any) }, manager *sharedbrowser.Manager, profileDir string) {
	if manager == nil {
		return
	}
	done := make(chan struct{}, 1)
	go func() {
		manager.Close()
		done <- struct{}{}
	}()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		log.Warnf("sds login browser manager close timed out, websocket may be stalled")
		terminateProfileBrowserProcesses(profileDir)
		clearProfileLockFiles(profileDir)
	}
}

type pageLoginState struct {
	Token        string
	OutToken     string
	Username     string
	MerchantID   int64
	UserID       int64
	Href         string
	BrowserState map[string]any
}

func readCurrentLoginState(manager *sharedbrowser.Manager, page playwright.Page) (*pageLoginState, error) {
	raw, err := page.Evaluate(`() => {
		const readNumber = (key) => {
			const raw = window.localStorage.getItem(key);
			if (!raw) return 0;
			const parsed = Number(raw);
			return Number.isFinite(parsed) ? parsed : 0;
		};
		return {
			token: window.localStorage.getItem("token") || "",
			outToken: window.localStorage.getItem("outToken") || "",
			username: window.localStorage.getItem("username") || "",
			merchantId: readNumber("merchant_id"),
			userId: readNumber("userid"),
			href: window.location.href,
		};
	}`, nil)
	if err != nil {
		return nil, err
	}
	values, _ := raw.(map[string]any)
	return &pageLoginState{
		Token:      stringValue(values["token"]),
		OutToken:   stringValue(values["outToken"]),
		Username:   stringValue(values["username"]),
		MerchantID: int64Value(values["merchantId"]),
		UserID:     int64Value(values["userId"]),
		Href:       stringValue(values["href"]),
	}, nil
}

func captureBrowserState(manager *sharedbrowser.Manager) (map[string]any, error) {
	if manager == nil || manager.GetContext() == nil {
		return nil, fmt.Errorf("browser context is not initialized")
	}
	storageState, err := manager.GetContext().StorageState()
	if err != nil {
		return nil, err
	}
	return map[string]any{"cookies": storageState.Cookies, "origins": storageState.Origins}, nil
}

func hasUsableSessionMarkers(state *pageLoginState) bool {
	if state == nil || strings.TrimSpace(state.Token) == "" || state.MerchantID <= 0 {
		return false
	}
	return true
}

func hasUsableLoginState(state *pageLoginState) bool {
	if !hasUsableSessionMarkers(state) {
		return false
	}
	if !isLoginPageURL(state.Href) && strings.TrimSpace(state.Href) != "" {
		return true
	}
	cookies, _ := state.BrowserState["cookies"].([]playwright.OptionalCookie)
	return len(cookies) > 0
}

func classifyLoginFailure(page playwright.Page, state *pageLoginState) (bool, error) {
	pageText := readPageText(page)
	currentURL := ""
	if state != nil {
		currentURL = state.Href
	}
	return classifyLoginFailureFromSignals(pageText, currentURL)
}

func readPageText(page playwright.Page) string {
	if page == nil {
		return ""
	}
	raw, err := page.Evaluate(`() => (document.body && document.body.innerText) ? document.body.innerText : ""`, nil)
	if err != nil {
		return ""
	}
	return stringValue(raw)
}

func stableFingerprintSeed(value string) int32 {
	if strings.TrimSpace(value) == "" {
		return int32(time.Now().Unix())
	}
	sum := sha256.Sum256([]byte(value))
	seedHex := hex.EncodeToString(sum[:])
	seed, err := strconv.ParseInt(seedHex[:8], 16, 32)
	if err != nil {
		return int32(time.Now().Unix())
	}
	return int32(seed)
}

func readPageTitle(page playwright.Page) string {
	if page == nil {
		return ""
	}
	title, err := page.Title()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(title)
}

func classifyLoginFailureFromSignals(pageText, currentURL string) (bool, error) {
	text := strings.ToLower(strings.TrimSpace(pageText))
	url := strings.ToLower(strings.TrimSpace(currentURL))
	if strings.Contains(text, "密码错误") ||
		strings.Contains(text, "账号或密码") ||
		strings.Contains(text, "用户名或密码") ||
		strings.Contains(text, "login failed") ||
		strings.Contains(text, "invalid") {
		return false, fmt.Errorf("SDS 登录失败，请检查账号密码")
	}
	if strings.Contains(text, "验证码") ||
		strings.Contains(text, "校验") ||
		strings.Contains(text, "人机") ||
		strings.Contains(text, "滑块") ||
		strings.Contains(text, "请勿频繁点击") ||
		strings.Contains(text, "稍后重试") {
		return true, fmt.Errorf("SDS 登录等待验证码或风控校验")
	}
	if isLoginPageURL(url) || strings.Contains(text, "登录") {
		return true, fmt.Errorf("SDS 登录等待验证码或风控校验")
	}
	return false, fmt.Errorf("SDS 登录未完成，请检查页面状态")
}

func isLoginPageURL(raw string) bool {
	value := strings.ToLower(strings.TrimSpace(raw))
	if value == "" {
		return false
	}
	return strings.Contains(value, "/user/login")
}

func buildPayload(account configuredAccount, state *pageLoginState, source string) *AuthPayload {
	payload := &AuthPayload{
		TenantID:     account.TenantID,
		ShopID:       account.Identifier,
		Identifier:   account.Identifier,
		Username:     coalesce(state.Username, account.Username),
		MerchantName: account.MerchantName,
		AccessToken:  state.Token,
		OutToken:     state.OutToken,
		MerchantID:   state.MerchantID,
		UserID:       state.UserID,
		BrowserState: state.BrowserState,
		IssuedAt:     time.Now(),
		Source:       source,
		CurrentURL:   state.Href,
	}
	return payload
}

func summarizeText(value string, maxChars int) string {
	normalized := strings.Join(strings.Fields(strings.TrimSpace(value)), " ")
	if maxChars <= 0 || len(normalized) <= maxChars {
		return normalized
	}
	return normalized[:maxChars] + "..."
}

func prefillLoginForm(page playwright.Page, account configuredAccount) error {
	return prefillLoginFormWithPage(playwrightLoginPage{page: page}, account)
}

type loginLocator interface {
	Count() (int, error)
	Click() error
	Press(key string) error
	Fill(value string) error
	Type(text string) error
	InputValue() (string, error)
	Evaluate(expression string, arg any) (any, error)
}

type loginPage interface {
	Locator(selector string) loginLocator
}

type playwrightLoginPage struct {
	page playwright.Page
}

func (p playwrightLoginPage) Locator(selector string) loginLocator {
	return playwrightLoginLocator{locator: p.page.Locator(selector).First()}
}

type playwrightLoginLocator struct {
	locator playwright.Locator
}

func (l playwrightLoginLocator) Count() (int, error) {
	return l.locator.Count()
}

func (l playwrightLoginLocator) Click() error {
	return l.locator.Click(playwright.LocatorClickOptions{Timeout: playwright.Float(3000)})
}

func (l playwrightLoginLocator) Press(key string) error {
	return l.locator.Press(key, playwright.LocatorPressOptions{Timeout: playwright.Float(2000)})
}

func (l playwrightLoginLocator) Fill(value string) error {
	return l.locator.Fill(value, playwright.LocatorFillOptions{Timeout: playwright.Float(5000)})
}

func (l playwrightLoginLocator) Type(text string) error {
	return l.locator.Type(text, playwright.LocatorTypeOptions{
		Delay:   playwright.Float(120),
		Timeout: playwright.Float(8000),
	})
}

func (l playwrightLoginLocator) InputValue() (string, error) {
	return l.locator.InputValue(playwright.LocatorInputValueOptions{Timeout: playwright.Float(2000)})
}

func (l playwrightLoginLocator) Evaluate(expression string, arg any) (any, error) {
	return l.locator.Evaluate(expression, arg, playwright.LocatorEvaluateOptions{Timeout: playwright.Float(2000)})
}

type loginField struct {
	name      string
	selectors []string
	value     string
}

func prefillLoginFormWithPage(page loginPage, account configuredAccount) error {
	fields := []loginField{
		{
			name: "merchant_name",
			selectors: []string{
				`#merchant_name`,
				`input[placeholder*="商户"]`,
				`input[name="merchant_name"]`,
				`input[id*="merchant"]`,
				`input[autocomplete="organization"]`,
			},
			value: account.MerchantName,
		},
		{
			name: "username",
			selectors: []string{
				`#username`,
				`input[placeholder*="手机"]`,
				`input[placeholder*="账号"]`,
				`input[placeholder*="用户名"]`,
				`input[name="username"]`,
				`input[name="account"]`,
				`input[type="text"]`,
				`input[type="tel"]`,
			},
			value: account.Username,
		},
		{
			name: "password",
			selectors: []string{
				`#password`,
				`input[type="password"]`,
				`input[placeholder*="密码"]`,
				`input[name="password"]`,
			},
			value: account.Password,
		},
	}
	for index, field := range fields {
		filled, err := typeCandidate(page, field.selectors, field.value)
		if err != nil {
			return fmt.Errorf("fill %s: %w", field.name, err)
		}
		if !filled {
			return fmt.Errorf("fill %s: no writable input matched", field.name)
		}
		if index < len(fields)-1 {
			time.Sleep(200 * time.Millisecond)
		}
	}
	time.Sleep(300 * time.Millisecond)
	return nil
}

func typeCandidate(page loginPage, selectors []string, value string) (bool, error) {
	if strings.TrimSpace(value) == "" {
		return false, nil
	}
	for _, selector := range selectors {
		loc := page.Locator(selector)
		count, err := loc.Count()
		if err != nil || count == 0 {
			continue
		}
		if err := loc.Click(); err != nil {
			continue
		}
		if err := loc.Press("Control+A"); err == nil {
			_ = loc.Press("Backspace")
		}
		if err := loc.Fill(value); err == nil {
			ok, verifyErr := locatorHasValue(loc, value)
			if verifyErr != nil {
				return false, verifyErr
			}
			if ok {
				return true, nil
			}
		}
		if err := loc.Type(value); err == nil {
			ok, verifyErr := locatorHasValue(loc, value)
			if verifyErr != nil {
				return false, verifyErr
			}
			if ok {
				return true, nil
			}
		}
		if _, err := loc.Evaluate(`(node, value) => {
			const input = node;
			if (!input) {
				return "";
			}
			input.focus();
			input.value = "";
			input.dispatchEvent(new Event("input", { bubbles: true }));
			input.value = value;
			input.dispatchEvent(new Event("input", { bubbles: true }));
			input.dispatchEvent(new Event("change", { bubbles: true }));
			return input.value;
		}`, value); err == nil {
			ok, verifyErr := locatorHasValue(loc, value)
			if verifyErr != nil {
				return false, verifyErr
			}
			if ok {
				return true, nil
			}
		}
		if actual, err := loc.InputValue(); err == nil && strings.TrimSpace(actual) != "" {
			return false, fmt.Errorf("input value %q did not match expected value", actual)
		}
	}
	return false, nil
}

func locatorHasValue(loc loginLocator, value string) (bool, error) {
	actual, err := loc.InputValue()
	if err != nil {
		return false, nil
	}
	if strings.TrimSpace(actual) == strings.TrimSpace(value) {
		return true, nil
	}
	return false, nil
}

var profileTrimDirs = []string{
	filepath.Join("Default", "Cache"),
	filepath.Join("Default", "Code Cache"),
	filepath.Join("Default", "GPUCache"),
	filepath.Join("Default", "ShaderCache"),
	filepath.Join("Default", "Service Worker", "CacheStorage"),
	filepath.Join("Default", "Session Storage"),
	"Crashpad",
	"GrShaderCache",
	"GraphiteDawnCache",
}

var profileLockFiles = []string{
	"SingletonLock",
	"SingletonCookie",
	"SingletonSocket",
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
		return fmt.Errorf("SDS 浏览器 profile 正在使用，请稍后重试或关闭当前登录窗口: %w", err)
	}
	if retryErr := manager.Launch(); retryErr != nil {
		if isProfileInUseError(retryErr) {
			return fmt.Errorf("SDS 浏览器 profile 正在使用，请稍后重试或关闭当前登录窗口: %w", retryErr)
		}
		return retryErr
	}
	return nil
}

func isProfileInUseError(err error) bool {
	if err == nil {
		return false
	}
	message := strings.ToLower(err.Error())
	return strings.Contains(message, "profile appears to be in use") ||
		strings.Contains(message, "processsingleton") ||
		strings.Contains(message, "profile directory") ||
		strings.Contains(message, "singletonlock")
}

func clearProfileLockFiles(profileDir string) bool {
	cleared := false
	for _, name := range profileLockFiles {
		path := filepath.Join(profileDir, name)
		if err := os.Remove(path); err == nil || os.IsNotExist(err) {
			if err == nil {
				cleared = true
			}
			continue
		}
	}
	return cleared
}

func terminateProfileBrowserProcesses(profileText string) int {
	profileText = strings.ToLower(strings.TrimSpace(profileText))
	if profileText == "" {
		return 0
	}
	if runtime.GOOS == "windows" {
		script := strings.ReplaceAll(`
$profile = '__PROFILE__'
$matches = Get-CimInstance Win32_Process |
  Where-Object {
    $_.CommandLine -and
    ($_.Name -match 'chrome|chromium') -and
    ($_.CommandLine.ToLowerInvariant().Contains($profile))
  }
$count = 0
foreach ($process in $matches) {
  try {
    Stop-Process -Id $process.ProcessId -Force -ErrorAction Stop
    $count += 1
  } catch {}
}
Write-Output $count
`, "__PROFILE__", strings.ReplaceAll(profileText, "'", "''"))
		cmd := exec.Command("powershell", "-NoProfile", "-Command", script)
		output, err := cmd.Output()
		if err != nil {
			return 0
		}
		lines := strings.Split(strings.TrimSpace(string(output)), "\n")
		if len(lines) == 0 {
			return 0
		}
		last := strings.TrimSpace(lines[len(lines)-1])
		var count int
		_, _ = fmt.Sscanf(last, "%d", &count)
		return count
	}
	return 0
}

func clickLoginIfPossible(page playwright.Page) (bool, error) {
	for _, selector := range []string{
		`button:has-text("登录")`,
		`button[type="submit"]`,
		`div[role="button"]:has-text("登录")`,
		`span:has-text("登录")`,
	} {
		loc := page.Locator(selector).First()
		count, err := loc.Count()
		if err != nil || count == 0 {
			continue
		}
		if err := loc.Click(playwright.LocatorClickOptions{Timeout: playwright.Float(3000)}); err == nil {
			return true, nil
		}
	}
	return false, nil
}

func defaultViewport(value, fallback int) int {
	if value > 0 {
		return value
	}
	return fallback
}

func sanitizePathSegment(value string) string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return "default"
	}
	replacer := strings.NewReplacer(":", "_", "/", "_", "\\", "_", " ", "_")
	return replacer.Replace(trimmed)
}

func stringValue(value any) string {
	switch typed := value.(type) {
	case string:
		return strings.TrimSpace(typed)
	default:
		return ""
	}
}

func int64Value(value any) int64 {
	switch typed := value.(type) {
	case float64:
		return int64(typed)
	case int64:
		return typed
	case int:
		return int64(typed)
	case string:
		parsed, _ := strconv.ParseInt(strings.TrimSpace(typed), 10, 64)
		return parsed
	default:
		return 0
	}
}

func optionalString(value *string) string {
	if value == nil {
		return ""
	}
	return strings.TrimSpace(*value)
}

func optionalBool(value *bool) bool {
	return value != nil && *value
}
