package sheinlogin

import "testing"

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
