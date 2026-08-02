package sheinlogin

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	sharedbrowser "task-processor/internal/crawler/shared/browser"

	"github.com/mxschmitt/playwright-go"
)

func cookieDiagnosticSummary(cookies []playwright.Cookie) map[string]any {
	domainSet := make(map[string]struct{}, len(cookies))
	for _, cookie := range cookies {
		domain := strings.TrimSpace(cookie.Domain)
		if domain == "" {
			domain = "<empty>"
		}
		domainSet[domain] = struct{}{}
	}
	domains := make([]string, 0, len(domainSet))
	for domain := range domainSet {
		domains = append(domains, domain)
	}
	sort.Strings(domains)
	return map[string]any{
		"count":   len(cookies),
		"domains": domains,
	}
}

func waitForLoginSurface(ctx context.Context, page playwright.Page) (loginSurface, error) {
	deadline := time.Now().Add(45 * time.Second)
	for time.Now().Before(deadline) {
		select {
		case <-ctx.Done():
			return loginSurfaceUnknown, ctx.Err()
		default:
		}
		loginFormVisible := isLoginFormVisible(page)
		verifyRequired, _ := isVerifyCodeRequired(page)
		loggedIn, _ := isLoggedIn(page)
		if surface := resolveLoginSurface(loginFormVisible, verifyRequired, loggedIn); surface != loginSurfaceUnknown {
			return surface, nil
		}
		if err := sleepWithContext(ctx, time.Second); err != nil {
			return loginSurfaceUnknown, err
		}
	}
	return loginSurfaceUnknown, fmt.Errorf("login surface not ready")
}

func isLoginFormVisible(page playwright.Page) bool {
	_, usernameErr := firstVisible(page, []string{
		"input.soui-input-input:first-of-type",
		`input.soui-input-input:not([type="password"])`,
		`input[type="text"].soui-input-input`,
		`input[type="text"]`,
	})
	if usernameErr != nil {
		return false
	}
	_, passwordErr := firstVisible(page, []string{
		`input[type="password"].soui-input-input`,
		`input[type="password"]`,
	})
	return passwordErr == nil
}

func exportAuthenticatedBrowserState(ctx context.Context, manager *sharedbrowser.Manager, page playwright.Page, account Account, artifactDir, profileDir, stage string) (*AutomationResult, error) {
	targetURL := postLoginTargetURL(account)
	if err := runBlockingStageWithContext(ctx, func() {
		closeManagerProfile(manager, profileDir)
	}, func() error {
		_, err := page.Goto(targetURL, playwright.PageGotoOptions{
			WaitUntil: playwright.WaitUntilStateDomcontentloaded,
			Timeout:   playwright.Float(30000),
		})
		return err
	}); err != nil {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		sheinLoginServiceLogger.WithError(err).WithFields(map[string]any{
			"tenant_id":  account.TenantID,
			"store_id":   account.StoreID,
			"target_url": targetURL,
		}).Warn("open SHEIN post-login target before exporting cookies failed")
	}
	storageState, err := manager.GetContext().StorageState()
	if err != nil {
		return artifactResult(page, artifactDir, account, stage, err)
	}
	contextCookies, contextCookieErr := manager.GetContext().Cookies()
	storageDiagnosticSummary := cookieDiagnosticSummary(storageState.Cookies)
	diagnosticFields := map[string]any{
		"tenant_id":                    account.TenantID,
		"store_id":                     account.StoreID,
		"target_url":                   targetURL,
		"stage":                        stage,
		"storage_state_cookie_count":   storageDiagnosticSummary["count"],
		"storage_state_cookie_domains": storageDiagnosticSummary["domains"],
		"context_cookie_read_error":    contextCookieErr != nil,
	}
	if contextCookieErr == nil {
		diagnosticSummary := cookieDiagnosticSummary(contextCookies)
		diagnosticFields["context_cookie_count"] = diagnosticSummary["count"]
		diagnosticFields["context_cookie_domains"] = diagnosticSummary["domains"]
	}
	sheinLoginServiceLogger.WithFields(diagnosticFields).Info("SHEIN cookie export diagnostic")
	state, err := validatedCookieOnlyBrowserState(storageState.Cookies)
	if err != nil {
		sheinLoginServiceLogger.WithFields(map[string]any{
			"tenant_id":    account.TenantID,
			"store_id":     account.StoreID,
			"target_url":   targetURL,
			"cookie_count": cookieCount(cookieOnlyBrowserState(map[string]any{"cookies": storageState.Cookies})),
		}).Warn("SHEIN login completed without exportable cookies")
		return artifactResult(page, artifactDir, account, stage, err)
	}
	sheinLoginServiceLogger.WithFields(map[string]any{
		"tenant_id":    account.TenantID,
		"store_id":     account.StoreID,
		"target_url":   targetURL,
		"cookie_count": cookieCount(state),
	}).Info("exported SHEIN cookies after authenticated login")
	return &AutomationResult{BrowserState: state}, nil
}

func fillLogin(ctx context.Context, page playwright.Page, account Account) error {
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
	if err := sleepWithContext(ctx, 300*time.Millisecond); err != nil {
		return err
	}
	if err := username.Fill(account.Username); err != nil {
		return err
	}
	if err := password.Click(); err != nil {
		return err
	}
	if err := sleepWithContext(ctx, 300*time.Millisecond); err != nil {
		return err
	}
	if err := password.Fill(account.Password); err != nil {
		return err
	}
	if err := sleepWithContext(ctx, 500*time.Millisecond); err != nil {
		return err
	}
	return nil
}

func submitLogin(ctx context.Context, page playwright.Page) error {
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
		if err := sleepWithContext(ctx, 300*time.Millisecond); err != nil {
			return err
		}
		if err := password.Press("Enter"); err == nil {
			if err := sleepWithContext(ctx, time.Second); err != nil {
				return err
			}
			if advanced, waitErr := loginOutcomeAdvanced(ctx, page, 2*time.Second); waitErr == nil && advanced {
				return nil
			} else if waitErr != nil {
				return waitErr
			}
		}
	}
	if err := button.Click(playwright.LocatorClickOptions{Timeout: playwright.Float(5000)}); err == nil {
		if advanced, waitErr := loginOutcomeAdvanced(ctx, page, 2*time.Second); waitErr == nil && advanced {
			return nil
		} else if waitErr != nil {
			return waitErr
		}
	}
	if dismissed, dismissErr := dismissRequestFailure(page); dismissErr == nil && dismissed {
		if advanced, waitErr := loginOutcomeAdvanced(ctx, page, 2*time.Second); waitErr == nil && advanced {
			return nil
		} else if waitErr != nil {
			return waitErr
		}
	}
	if err := button.Click(playwright.LocatorClickOptions{Timeout: playwright.Float(5000), Force: playwright.Bool(true)}); err == nil {
		if advanced, waitErr := loginOutcomeAdvanced(ctx, page, 2*time.Second); waitErr == nil && advanced {
			return nil
		} else if waitErr != nil {
			return waitErr
		}
	}
	if dismissed, dismissErr := dismissRequestFailure(page); dismissErr == nil && dismissed {
		if advanced, waitErr := loginOutcomeAdvanced(ctx, page, 2*time.Second); waitErr == nil && advanced {
			return nil
		} else if waitErr != nil {
			return waitErr
		}
	}
	if _, evalErr := button.Evaluate(`(el) => el.click()`, nil); evalErr == nil {
		if advanced, waitErr := loginOutcomeAdvanced(ctx, page, 2*time.Second); waitErr == nil && advanced {
			return nil
		} else if waitErr != nil {
			return waitErr
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
		if advanced, waitErr := loginOutcomeAdvanced(ctx, page, 2*time.Second); waitErr == nil && advanced {
			return nil
		} else if waitErr != nil {
			return waitErr
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
		payloads := getCapturedNetworkPayloads(page)
		if verifyRequired, err := isVerifyCodeRequired(page); err == nil && verifyRequired {
			return true, nil
		}
		if networkPayloadsRequireVerifyCode(payloads) {
			return true, nil
		}
		if networkPayloadsConfirmSHEINLogin(payloads) {
			return false, nil
		}
		if loginError, err := extractLoginError(page); err == nil && loginError != "" {
			return false, fmt.Errorf("%s", loginError)
		}
		if dismissed, _ := dismissRequestFailure(page); dismissed {
			continue
		}
		if err := sleepWithContext(ctx, time.Second); err != nil {
			return false, err
		}
	}
	if loginError, err := extractLoginError(page); err == nil && loginError != "" {
		return false, fmt.Errorf("%s", loginError)
	}
	payloads := getCapturedNetworkPayloads(page)
	if networkPayloadsRequireVerifyCode(payloads) {
		return true, nil
	}
	if networkPayloadsConfirmSHEINLogin(payloads) {
		return false, nil
	}
	if shouldWaitForCaptcha(page) {
		return true, nil
	}
	return false, fmt.Errorf("login outcome timeout")
}

func loginOutcomeAdvanced(ctx context.Context, page playwright.Page, wait time.Duration) (bool, error) {
	deadline := time.Now().Add(wait)
	for time.Now().Before(deadline) {
		if err := ctx.Err(); err != nil {
			return false, err
		}
		if loggedIn, err := isLoggedIn(page); err == nil && loggedIn {
			return true, nil
		}
		if verifyRequired, err := isVerifyCodeRequired(page); err == nil && verifyRequired {
			return true, nil
		}
		if networkPayloadsRequireVerifyCode(getCapturedNetworkPayloads(page)) {
			return true, nil
		}
		if loginError, err := extractLoginError(page); err == nil && loginError != "" {
			return true, nil
		}
		if dismissed, _ := dismissRequestFailure(page); dismissed {
			continue
		}
		if err := sleepWithContext(ctx, 200*time.Millisecond); err != nil {
			return false, err
		}
	}
	return false, nil
}

func settleAfterSubmit(ctx context.Context, page playwright.Page, wait time.Duration) error {
	deadline := time.Now().Add(wait)
	for time.Now().Before(deadline) {
		if err := ctx.Err(); err != nil {
			return err
		}
		if loggedIn, err := isLoggedIn(page); err == nil && loggedIn {
			return nil
		}
		if verifyRequired, err := isVerifyCodeRequired(page); err == nil && verifyRequired {
			return nil
		}
		if dismissed, _ := dismissRequestFailure(page); dismissed {
			continue
		}
		if err := sleepWithContext(ctx, 200*time.Millisecond); err != nil {
			return err
		}
	}
	return nil
}
