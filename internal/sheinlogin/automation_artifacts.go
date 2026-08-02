package sheinlogin

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/mxschmitt/playwright-go"
)

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
	if errors.Is(cause, ErrNoUsableCookie) {
		errorCode = "COOKIE_EXPORT_EMPTY"
	}
	actionKey, actionMessage := deriveFailureAction(metadata.PageState, metadata.VerifyCodeVisible != nil && *metadata.VerifyCodeVisible, errorCode)
	return &AutomationResult{
		ErrorCode:           errorCode,
		ErrorMessage:        cause.Error(),
		FailureArtifactPath: dir,
		FailureSummary: &FailureSummary{
			ErrorCode:            errorCode,
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
	debugEnabled := sheinLoginDebugEnabled()
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
	if debugEnabled {
		if events := getCapturedPageEvents(page); len(events) > 0 {
			metadata.PageEvents = events
		}
		if requests := collectResourceRequests(page); len(requests) > 0 {
			metadata.ResourceRequests = requests
		}
		if snapshot := collectDeviceSnapshot(page); len(snapshot) > 0 {
			metadata.DeviceSnapshot = snapshot
		}
		if snapshot := collectFormSnapshot(page); len(snapshot) > 0 {
			metadata.FormSnapshot = snapshot
		}
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

func sheinLoginDebugEnabled() bool {
	return strings.TrimSpace(os.Getenv("TASK_PROCESSOR_SHEIN_LOGIN_DEBUG")) == "1"
}

func collectFormSnapshot(page playwright.Page) map[string]any {
	if page == nil {
		return nil
	}
	value, err := page.Evaluate(`() => {
		const readValue = (selector) => {
			const el = document.querySelector(selector);
			if (!el) return null;
			return typeof el.value === 'string' ? el.value : null;
		};
		const readDisabled = (selector) => {
			const el = document.querySelector(selector);
			if (!el) return null;
			return !!el.disabled;
		};
		const active = document.activeElement;
		return {
			usernameValue: readValue('input.soui-input-input:not([type="password"]), input[type="text"].soui-input-input, input[type="text"]'),
			passwordLength: (() => {
				const value = readValue('input[type="password"].soui-input-input, input[type="password"]');
				return typeof value === 'string' ? value.length : null;
			})(),
			loginButtonDisabled: readDisabled('button.soui-button-primary, button[type="submit"], button'),
			activeTag: active ? active.tagName : null,
			activeType: active && active.getAttribute ? active.getAttribute('type') : null,
		};
	}`, nil)
	if err != nil || value == nil {
		return nil
	}
	snapshot, ok := value.(map[string]interface{})
	if !ok || len(snapshot) == 0 {
		return nil
	}
	result := make(map[string]any, len(snapshot))
	for k, v := range snapshot {
		result[k] = v
	}
	return result
}

func collectResourceRequests(page playwright.Page) []map[string]any {
	if page == nil {
		return nil
	}
	requests, err := page.Requests()
	if err != nil || len(requests) == 0 {
		return nil
	}
	results := make([]map[string]any, 0, len(requests))
	for _, request := range requests {
		if request == nil {
			continue
		}
		resourceType := strings.TrimSpace(request.ResourceType())
		if resourceType != "document" && resourceType != "script" && resourceType != "xhr" && resourceType != "fetch" {
			continue
		}
		url := strings.TrimSpace(request.URL())
		if url == "" {
			continue
		}
		item := map[string]any{
			"resourceType": resourceType,
			"method":       strings.TrimSpace(request.Method()),
			"url":          url,
		}
		if failure := request.Failure(); failure != nil {
			item["failure"] = summarizeBodyText(failure.Error(), 300)
		}
		if response, respErr := request.Response(); respErr == nil && response != nil {
			item["status"] = response.Status()
		}
		if strings.Contains(strings.ToLower(url), "geetest") ||
			strings.Contains(strings.ToLower(url), "zpnv") ||
			strings.Contains(strings.ToLower(url), "us-fp") ||
			strings.Contains(strings.ToLower(url), "us-behavior") ||
			strings.Contains(strings.ToLower(url), "antiin") ||
			strings.Contains(strings.ToLower(url), "infp") ||
			strings.Contains(strings.ToLower(url), "fpv2") ||
			strings.Contains(strings.ToLower(url), "fm.") ||
			strings.Contains(strings.ToLower(url), "gt.js") {
			item["important"] = true
		}
		results = append(results, item)
	}
	sort.SliceStable(results, func(i, j int) bool {
		leftPriority := resourceRequestPriority(results[i])
		rightPriority := resourceRequestPriority(results[j])
		if leftPriority != rightPriority {
			return leftPriority < rightPriority
		}
		return anyString(results[i]["url"]) < anyString(results[j]["url"])
	})
	if len(results) > 40 {
		results = results[:40]
	}
	return results
}

func collectDeviceSnapshot(page playwright.Page) map[string]any {
	if page == nil {
		return nil
	}
	value, err := page.Evaluate(`async () => {
		const safeLength = (value) => typeof value === 'string' ? value.length : null;
		const safeKeys = (value) => value && typeof value === 'object' ? Object.keys(value).slice(0, 12) : [];
		const safeAwait = async (getter) => {
			try {
				return await getter();
			} catch (e) {
				return null;
			}
		};
		const antiInResolved = await safeAwait(async () => {
			if (!(window.AntiIn && typeof window.AntiIn.getAllEncrypted === 'function')) return null;
			return await window.AntiIn.getAllEncrypted();
		});
		const armorTokenResolved = await safeAwait(async () => {
			if (!(window.AntiDevices && typeof window.AntiDevices.getArmorToken === 'function')) return null;
			return await window.AntiDevices.getArmorToken();
		});
		const smDeviceIdResolved = await safeAwait(async () => {
			if (!(window.SMSdk && typeof window.SMSdk.getDeviceId === 'function')) return null;
			return await window.SMSdk.getDeviceId();
		});
		const fmInfoResolved = await safeAwait(async () => {
			if (!(window._fmOpt && typeof window._fmOpt.getinfo === 'function')) return null;
			return await window._fmOpt.getinfo();
		});
		return {
			blackboxLength: safeLength(window.blackbox),
			antiInLength: safeLength(window._AntiInVal),
			armorTokenLength: safeLength(window._armorToken),
			smDeviceIdLength: safeLength(window._fpvSmDeviceId),
			antiInResolvedLength: safeLength(antiInResolved),
			armorTokenResolvedLength: safeLength(armorTokenResolved),
			smDeviceIdResolvedLength: safeLength(smDeviceIdResolved),
			fmInfoResolvedLength: safeLength(fmInfoResolved),
			fmOptHasGetInfo: !!(window._fmOpt && typeof window._fmOpt.getinfo === 'function'),
			fmOptKeys: safeKeys(window._fmOpt),
			antiInHasGetAllEncrypted: !!(window.AntiIn && typeof window.AntiIn.getAllEncrypted === 'function'),
			antiInKeys: safeKeys(window.AntiIn),
			smSdkHasGetDeviceId: !!(window.SMSdk && typeof window.SMSdk.getDeviceId === 'function'),
			smSdkKeys: safeKeys(window.SMSdk),
			antiDevicesHasGetArmorToken: !!(window.AntiDevices && typeof window.AntiDevices.getArmorToken === 'function'),
			antiDevicesKeys: safeKeys(window.AntiDevices),
			inconfApiHost: window._INCONF && window._INCONF.apiHost ? window._INCONF.apiHost : null,
			smConfApiHost: window._smConf && window._smConf.apiHost ? window._smConf.apiHost : null,
		};
	}`, nil)
	if err != nil || value == nil {
		return nil
	}
	snapshot, ok := value.(map[string]interface{})
	if !ok || len(snapshot) == 0 {
		return nil
	}
	result := make(map[string]any, len(snapshot))
	for k, v := range snapshot {
		result[k] = v
	}
	return result
}

func waitForDeviceContextReady(ctx context.Context, page playwright.Page, timeout time.Duration) bool {
	if page == nil || timeout <= 0 {
		return false
	}
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		select {
		case <-ctx.Done():
			return false
		default:
		}
		primeDeviceContext(page)
		snapshot := collectDeviceSnapshot(page)
		if isDeviceContextReadySnapshot(snapshot) {
			return true
		}
		if err := sleepWithContext(ctx, 300*time.Millisecond); err != nil {
			return false
		}
	}
	return false
}

func isDeviceContextReadySnapshot(snapshot map[string]any) bool {
	if len(snapshot) == 0 {
		return false
	}
	return anyInt64Default(snapshot["blackboxLength"]) > 0 &&
		anyInt64Default(snapshot["antiInResolvedLength"]) > 0 &&
		anyInt64Default(snapshot["armorTokenResolvedLength"]) > 0 &&
		anyInt64Default(snapshot["smDeviceIdResolvedLength"]) > 0
}

func primeDeviceContext(page playwright.Page) {
	if page == nil {
		return
	}
	_, _ = page.Evaluate(`async () => {
		const results = {};
		const safe = async (key, runner) => {
			try {
				const value = await runner();
				if (typeof value === 'string' && value) {
					results[key] = value.length;
				}
				return value;
			} catch (e) {
				return '';
			}
		};

		if (!window.blackbox && window._fmOpt && typeof window._fmOpt.getinfo === 'function') {
			const blackbox = await safe('blackbox', async () => window._fmOpt.getinfo());
			if (typeof blackbox === 'string' && blackbox) {
				window.blackbox = blackbox;
			}
		}

		if ((!window._AntiInVal || !String(window._AntiInVal).trim()) && window.AntiIn && typeof window.AntiIn.getAllEncrypted === 'function') {
			const channel = (window.AntiIn && window.AntiIn.Channel) || {};
			const antiInValue = await safe('antiIn', async () => window.AntiIn.getAllEncrypted(channel.PC || channel.M));
			if (typeof antiInValue === 'string' && antiInValue) {
				window._AntiInVal = antiInValue;
			}
		}

		if ((!window._fpvSmDeviceId || !String(window._fpvSmDeviceId).trim()) && window.SMSdk && typeof window.SMSdk.getDeviceId === 'function') {
			const smDeviceId = await safe('smDeviceId', async () => window.SMSdk.getDeviceId());
			if (typeof smDeviceId === 'string' && smDeviceId) {
				window._fpvSmDeviceId = smDeviceId;
			}
		}

		if ((!window._armorToken || !String(window._armorToken).trim()) && window.AntiDevices && typeof window.AntiDevices.getArmorToken === 'function') {
			const armorToken = await safe('armorToken', async () => window.AntiDevices.getArmorToken());
			if (typeof armorToken === 'string' && armorToken) {
				window._armorToken = armorToken;
			}
		}

		return results;
	}`, nil)
}

func resourceRequestPriority(item map[string]any) int {
	if important, ok := item["important"].(bool); ok && important {
		return 0
	}
	resourceType := anyString(item["resourceType"])
	switch resourceType {
	case "xhr", "fetch":
		return 1
	case "document":
		return 2
	case "script":
		return 3
	default:
		return 4
	}
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
