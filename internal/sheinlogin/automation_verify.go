package sheinlogin

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	sharedbrowser "task-processor/internal/crawler/shared/browser"

	"github.com/mxschmitt/playwright-go"
)

type playwrightVerifySession struct {
	mu          sync.Mutex
	account     Account
	manager     *sharedbrowser.Manager
	page        playwright.Page
	artifactDir string
	profileDir  string
}

func (s *playwrightVerifySession) SubmitCode(ctx context.Context, code string) (*AutomationResult, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if err := submitVerifyCode(ctx, s.page, code); err != nil {
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
	return exportAuthenticatedBrowserState(s.manager, s.page, s.account, s.artifactDir, "export_state_after_verify")
}

func (s *playwrightVerifySession) WaitForLogin(ctx context.Context) (*AutomationResult, error) {
	for {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}

		s.mu.Lock()
		loggedIn, loginErr := isLoggedIn(s.page)
		if loginErr == nil && loggedIn {
			s.mu.Unlock()
			return exportAuthenticatedBrowserState(s.manager, s.page, s.account, s.artifactDir, "export_state_after_manual_verify")
		}
		_, _ = dismissRequestFailure(s.page)
		s.mu.Unlock()

		if err := sleepWithContext(ctx, time.Second); err != nil {
			return nil, err
		}
	}
}

func (s *playwrightVerifySession) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.manager != nil {
		closeManagerProfile(s.manager, s.profileDir)
	}
	return nil
}

func submitVerifyCode(ctx context.Context, page playwright.Page, code string) error {
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
		if err := ctx.Err(); err != nil {
			return err
		}
		disabled, disabledErr := button.IsDisabled()
		if disabledErr == nil && !disabled {
			break
		}
		if err := sleepWithContext(ctx, 200*time.Millisecond); err != nil {
			return err
		}
	}
	return clickWithFallback(page, button)
}
