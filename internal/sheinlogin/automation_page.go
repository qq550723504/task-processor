package sheinlogin

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/mxschmitt/playwright-go"
)

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

	if code := classifyLoginFailureFromNetworkPayloads(metadata.NetworkPayloads); code != "LOGIN_FAILED" {
		return code
	}

	for _, candidate := range []string{metadata.LoginError, metadata.Error, metadata.BodyText, metadata.Title, metadata.URL} {
		if code := classifyLoginErrorText(candidate); code != "LOGIN_FAILED" {
			return code
		}
	}
	return "LOGIN_FAILED"
}

func classifyLoginFailureFromNetworkPayloads(payloads []map[string]any) string {
	for i := len(payloads) - 1; i >= 0; i-- {
		payload := payloads[i]
		url := strings.ToLower(strings.TrimSpace(fmt.Sprint(payload["url"])))
		if !strings.Contains(url, "/sso/authenticate/login") {
			continue
		}

		bodyPreview := strings.TrimSpace(fmt.Sprint(payload["bodyPreview"]))
		if bodyPreview == "" {
			bodyPreview = strings.TrimSpace(fmt.Sprint(payload["body_preview"]))
		}
		if bodyPreview == "" {
			continue
		}

		if code := classifyLoginFailureFromLoginResponseBody(bodyPreview); code != "LOGIN_FAILED" {
			return code
		}
	}
	return "LOGIN_FAILED"
}

func classifyLoginFailureFromLoginResponseBody(body string) string {
	body = strings.TrimSpace(body)
	if body == "" {
		return "LOGIN_FAILED"
	}

	var payload struct {
		Code string `json:"code"`
		Msg  string `json:"msg"`
		Info struct {
			NeedValidCode bool `json:"needValidCode"`
		} `json:"info"`
	}
	if err := json.Unmarshal([]byte(body), &payload); err == nil {
		switch {
		case payload.Info.NeedValidCode:
			return "VERIFY_CODE_REQUIRED"
		case strings.TrimSpace(payload.Code) == "022008":
			return "VERIFY_CODE_REQUIRED"
		case classifyLoginErrorText(payload.Msg) != "LOGIN_FAILED":
			return classifyLoginErrorText(payload.Msg)
		}
	}

	return classifyLoginErrorText(body)
}

func networkPayloadsRequireVerifyCode(payloads []map[string]any) bool {
	return classifyLoginFailureFromNetworkPayloads(payloads) == "VERIFY_CODE_REQUIRED"
}
