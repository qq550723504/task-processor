package sdslogin

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"task-processor/internal/core/logger"
	sharedbrowser "task-processor/internal/crawler/shared/browser"

	"github.com/playwright-community/playwright-go"
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
	profileDir, cleanupProfile, err := createEphemeralProfileDir(cfg.ProfileRoot, account)
	if err != nil {
		return nil, false, err
	}
	defer cleanupProfile()

	managerCfg := &sharedbrowser.BrowserConfig{
		Headless:          cfg.Headless,
		BrowserPath:       strings.TrimSpace(cfg.BrowserPath),
		ChromeVersion:     strings.TrimSpace(cfg.ChromeVersion),
		ChromeDownloadDir: strings.TrimSpace(cfg.ChromeDownloadDir),
		ViewportWidth:     defaultViewport(cfg.ViewportWidth, 1440),
		ViewportHeight:    defaultViewport(cfg.ViewportHeight, 960),
		Language:          "en-US",
		AcceptLanguage:    "en-US,en;q=0.9",
	}
	manager := sharedbrowser.NewManager(managerCfg)
	manager.SetUserDataDir(profileDir)
	manager.SetFingerprint(manager.GenerateStableFingerprint(account.Identifier))
	if err := manager.Install(); err != nil {
		return nil, false, err
	}
	if err := manager.Launch(); err != nil {
		manager.Close()
		return nil, false, err
	}
	defer manager.Close()
	tracer := newBrowserEventTracer(account)
	tracer.attachContext(manager.GetContext())

	page, err := acquireLoginPage(manager)
	if err != nil {
		return nil, false, err
	}
	tracer.attachPage(page, "login-root")

	loginURL := strings.TrimSpace(cfg.LoginURL)
	if loginURL == "" {
		loginURL = "https://www.sdsdiy.com/user/login?redirect=%2Fadmin%2Fmaterial"
	}
	if _, err = page.Goto(loginURL, playwright.PageGotoOptions{
		WaitUntil: playwright.WaitUntilStateDomcontentloaded,
		Timeout:   playwright.Float(90000),
	}); err != nil {
		return nil, false, err
	}

	if err := prefillLoginForm(page, account); err != nil {
		return nil, false, err
	}
	_, _ = clickLoginIfPossible(page)

	deadline := time.Now().Add(30 * time.Second)
	var lastState *pageLoginState
	for time.Now().Before(deadline) {
		select {
		case <-ctx.Done():
			return nil, false, ctx.Err()
		default:
		}
		state, err := readCurrentLoginState(manager, page)
		if err == nil {
			lastState = state
		}
		if err == nil && hasUsableSessionMarkers(state) {
			browserState, stateErr := captureBrowserState(manager)
			if stateErr == nil {
				state.BrowserState = browserState
			}
		}
		if err == nil && hasUsableLoginState(state) {
			return buildPayload(account, state, "fresh_login"), false, nil
		}
		time.Sleep(time.Second)
	}

	return nil, false, classifyLoginFailure(page, lastState)
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

func createEphemeralProfileDir(profileRoot string, account configuredAccount) (string, func(), error) {
	root := strings.TrimSpace(profileRoot)
	if root == "" {
		root = filepath.Join(os.TempDir(), "task-processor-sds-login")
	}
	if !filepath.IsAbs(root) {
		absRoot, err := filepath.Abs(root)
		if err != nil {
			return "", nil, err
		}
		root = absRoot
	}
	attemptRoot := filepath.Join(root, "attempts")
	if err := os.MkdirAll(attemptRoot, 0o755); err != nil {
		return "", nil, err
	}
	prefix := sanitizePathSegment(account.TenantID) + "-" + sanitizePathSegment(account.Identifier) + "-"
	dir, err := os.MkdirTemp(attemptRoot, prefix)
	if err != nil {
		return "", nil, err
	}
	cleanup := func() {
		_ = os.RemoveAll(dir)
	}
	return dir, cleanup, nil
}

func acquireLoginPage(manager *sharedbrowser.Manager) (playwright.Page, error) {
	if manager == nil || manager.GetContext() == nil {
		return nil, fmt.Errorf("browser context is not initialized")
	}
	pages := manager.GetContext().Pages()
	if len(pages) > 0 && pages[0] != nil {
		return pages[0], nil
	}
	return manager.NewPage()
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
	cookies, _ := state.BrowserState["cookies"].([]playwright.OptionalCookie)
	return len(cookies) > 0
}

func classifyLoginFailure(page playwright.Page, state *pageLoginState) error {
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

func classifyLoginFailureFromSignals(pageText, currentURL string) error {
	text := strings.ToLower(strings.TrimSpace(pageText))
	url := strings.ToLower(strings.TrimSpace(currentURL))
	if strings.Contains(text, "密码错误") ||
		strings.Contains(text, "账号或密码") ||
		strings.Contains(text, "用户名或密码") ||
		strings.Contains(text, "login failed") ||
		strings.Contains(text, "invalid") {
		return fmt.Errorf("SDS 登录失败，请检查账号密码")
	}
	if isLoginPageURL(url) || strings.Contains(text, "登录") {
		return fmt.Errorf("SDS 登录失败，请检查账号密码")
	}
	return fmt.Errorf("SDS 登录未完成，请检查页面状态")
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
	if cookies, ok := state.BrowserState["cookies"].([]playwright.OptionalCookie); ok {
		for _, item := range cookies {
			record := CookieRecord{
				Name:     item.Name,
				Value:    item.Value,
				Domain:   optionalString(item.Domain),
				Path:     optionalString(item.Path),
				Secure:   optionalBool(item.Secure),
				HTTPOnly: optionalBool(item.HttpOnly),
			}
			if item.Expires != nil && *item.Expires > 0 {
				record.Expires = time.Unix(int64(*item.Expires), 0)
			}
			payload.Cookies = append(payload.Cookies, record)
		}
	}
	return payload
}

func prefillLoginForm(page playwright.Page, account configuredAccount) error {
	if _, err := fillCandidate(page, []string{
		`input[placeholder*="商户"]`,
		`input[name="merchant_name"]`,
		`input[id*="merchant"]`,
		`input[autocomplete="organization"]`,
	}, account.MerchantName); err != nil {
		return err
	}
	if _, err := fillCandidate(page, []string{
		`input[placeholder*="手机"]`,
		`input[placeholder*="账号"]`,
		`input[placeholder*="用户名"]`,
		`input[name="username"]`,
		`input[name="account"]`,
		`input[type="text"]`,
		`input[type="tel"]`,
	}, account.Username); err != nil {
		return err
	}
	if _, err := fillCandidate(page, []string{
		`input[type="password"]`,
		`input[placeholder*="密码"]`,
		`input[name="password"]`,
	}, account.Password); err != nil {
		return err
	}
	return nil
}

func fillCandidate(page playwright.Page, selectors []string, value string) (bool, error) {
	if strings.TrimSpace(value) == "" {
		return false, nil
	}
	for _, selector := range selectors {
		loc := page.Locator(selector).First()
		count, err := loc.Count()
		if err != nil || count == 0 {
			continue
		}
		if err := loc.Fill(value, playwright.LocatorFillOptions{Timeout: playwright.Float(3000)}); err == nil {
			return true, nil
		}
	}
	return false, nil
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
