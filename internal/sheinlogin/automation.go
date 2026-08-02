package sheinlogin

import (
	"context"
	"os"
	"strings"
	"sync"
	"time"

	sharedbrowser "task-processor/internal/crawler/shared/browser"

	"github.com/mxschmitt/playwright-go"
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

type loginSurface int

const (
	loginSurfaceUnknown loginSurface = iota
	loginSurfaceForm
	loginSurfaceVerifyCode
	loginSurfaceAuthenticated
)

func resolveLoginSurface(loginFormVisible, verifyCodeVisible, loggedIn bool) loginSurface {
	switch {
	case loginFormVisible:
		return loginSurfaceForm
	case verifyCodeVisible:
		return loginSurfaceVerifyCode
	case loggedIn:
		return loginSurfaceAuthenticated
	default:
		return loginSurfaceUnknown
	}
}

func postLoginTargetURL(account Account) string {
	loginURL := strings.ToLower(strings.TrimSpace(account.LoginURL))
	if loginURL == "" || strings.Contains(loginURL, "sellerhub.shein.com") {
		return "https://sellerhub.shein.com/#/spmp/commdities/list"
	}
	return "https://sso.geiwohuo.com/#/spmp/commdities/list"
}

func validatedCookieOnlyBrowserState(cookies any) (map[string]any, error) {
	state := cookieOnlyBrowserState(map[string]any{"cookies": cookies})
	if cookieCount(state) == 0 {
		return nil, ErrNoUsableCookie
	}
	return state, nil
}

type artifactMetadata struct {
	TenantID               int64            `json:"tenant_id"`
	StoreID                int64            `json:"store_id"`
	Username               string           `json:"username"`
	Stage                  string           `json:"stage"`
	Error                  string           `json:"error"`
	ErrorCode              string           `json:"error_code"`
	PageState              string           `json:"page_state,omitempty"`
	ActionKey              string           `json:"action_key,omitempty"`
	ActionMessage          string           `json:"action_message,omitempty"`
	CapturedAt             string           `json:"captured_at"`
	URL                    string           `json:"url,omitempty"`
	Title                  string           `json:"title,omitempty"`
	LoggedIn               *bool            `json:"logged_in,omitempty"`
	VerifyCodeVisible      *bool            `json:"verify_code_visible,omitempty"`
	OnLoginPage            *bool            `json:"on_login_page,omitempty"`
	RequestFailureModal    *bool            `json:"request_failure_modal,omitempty"`
	LoginFormVisible       *bool            `json:"login_form_visible,omitempty"`
	SellerHubVisible       *bool            `json:"seller_hub_visible,omitempty"`
	VerificationVisible    *bool            `json:"verification_visible,omitempty"`
	PermissionVisible      *bool            `json:"permission_visible,omitempty"`
	AgreementVisible       *bool            `json:"agreement_visible,omitempty"`
	CredentialErrorVisible *bool            `json:"credential_error_visible,omitempty"`
	LoginError             string           `json:"login_error,omitempty"`
	BodyText               string           `json:"body_text,omitempty"`
	SelectorStates         map[string]bool  `json:"selector_states,omitempty"`
	NetworkPayloads        []map[string]any `json:"network_payloads,omitempty"`
	PageEvents             []map[string]any `json:"page_events,omitempty"`
	ResourceRequests       []map[string]any `json:"resource_requests,omitempty"`
	DeviceSnapshot         map[string]any   `json:"device_snapshot,omitempty"`
	FormSnapshot           map[string]any   `json:"form_snapshot,omitempty"`
}

type Automation interface {
	Login(ctx context.Context, account Account, cfg AutomationConfig, store *RedisStore) (*AutomationResult, error)
	StartLogin(ctx context.Context, account Account, cfg AutomationConfig) (*AutomationResult, VerifySession, error)
}

type VerifySession interface {
	SubmitCode(ctx context.Context, code string) (*AutomationResult, error)
	Close() error
}

// VerifySessionLoginWatcher is implemented by verification sessions that can
// observe a user completing the remaining SHEIN checks in the browser window.
// The service uses it to persist the browser state even when the final step is
// completed manually after the verify-code request has returned.
type VerifySessionLoginWatcher interface {
	WaitForLogin(ctx context.Context) (*AutomationResult, error)
}

type PlaywrightAutomation struct{}

func NewPlaywrightAutomation() *PlaywrightAutomation { return &PlaywrightAutomation{} }

type pageNetworkCapture struct {
	mu        sync.Mutex
	items     []map[string]any
	installed bool
}

type pageEventCapture struct {
	mu        sync.Mutex
	items     []map[string]any
	installed bool
}

var pageNetworkCaptures sync.Map
var pageEventCaptures sync.Map

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
	managerCfg := buildAutomationBrowserConfig(account, cfg)
	manager := sharedbrowser.NewManager(managerCfg)
	manager.SetUserDataDir(profileDir)
	manager.SetFingerprint(buildAutomationFingerprint(account, managerCfg))
	launchCleanup := newOnceCleanup(manager.Close)
	if err := ctx.Err(); err != nil {
		closeManagerProfile(manager, profileDir)
		return nil, nil, err
	}
	if err := runBlockingStageWithContext(ctx, launchCleanup, manager.Install); err != nil {
		if shouldCloseManagerAfterStageError(err) {
			closeManagerProfile(manager, profileDir)
		}
		return nil, nil, err
	}
	if err := runBlockingStageWithContext(ctx, nil, func() error {
		return launchManagerWithProfileRecoveryContext(ctx, manager, profileDir, launchCleanup)
	}); err != nil {
		if shouldCloseManagerAfterStageError(err) {
			closeManagerProfile(manager, profileDir)
		}
		return nil, nil, err
	}

	page, err := manager.NewPage()
	if err != nil {
		closeManagerProfile(manager, profileDir)
		return nil, nil, err
	}
	_ = installPageNetworkCapture(page)
	if err := runBlockingStageWithContext(ctx, func() {
		closeManagerProfile(manager, profileDir)
	}, func() error {
		_, gotoErr := page.Goto(loginURLForAccount(account), playwright.PageGotoOptions{
			WaitUntil: playwright.WaitUntilStateDomcontentloaded,
			Timeout:   playwright.Float(60000),
		})
		return gotoErr
	}); err != nil {
		if shouldCloseManagerAfterStageError(err) {
			closeManagerProfile(manager, profileDir)
		}
		return nil, nil, err
	}
	surface, err := waitForLoginSurface(ctx, page)
	if err != nil {
		if ctx.Err() != nil {
			closeManagerProfile(manager, profileDir)
			return nil, nil, ctx.Err()
		}
		result, resultErr := artifactResult(page, cfg.ArtifactDir, account, "wait_login_surface", err)
		closeManagerProfile(manager, profileDir)
		return result, nil, resultErr
	}
	if surface == loginSurfaceAuthenticated {
		result, resultErr := exportAuthenticatedBrowserState(manager, page, account, cfg.ArtifactDir, "export_state_already_logged_in")
		closeManagerProfile(manager, profileDir)
		return result, nil, resultErr
	}
	if surface == loginSurfaceVerifyCode {
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
	if err := fillLogin(ctx, page, account); err != nil {
		if ctx.Err() != nil {
			closeManagerProfile(manager, profileDir)
			return nil, nil, ctx.Err()
		}
		result, resultErr := artifactResult(page, cfg.ArtifactDir, account, "fill_login", err)
		closeManagerProfile(manager, profileDir)
		return result, nil, resultErr
	}
	waitForDeviceContextReady(ctx, page, 12*time.Second)
	if err := submitLogin(ctx, page); err != nil {
		if ctx.Err() != nil {
			closeManagerProfile(manager, profileDir)
			return nil, nil, ctx.Err()
		}
		result, resultErr := artifactResult(page, cfg.ArtifactDir, account, "submit_login", err)
		closeManagerProfile(manager, profileDir)
		return result, nil, resultErr
	}
	if waiting, err := waitForLoginOutcome(ctx, page); err != nil {
		if ctx.Err() != nil {
			closeManagerProfile(manager, profileDir)
			return nil, nil, ctx.Err()
		}
		_ = settleAfterSubmit(ctx, page, 8*time.Second)
		if loggedIn, loginErr := isLoggedIn(page); loginErr == nil && loggedIn {
			result, resultErr := exportAuthenticatedBrowserState(manager, page, account, cfg.ArtifactDir, "export_state_after_recover")
			closeManagerProfile(manager, profileDir)
			return result, nil, resultErr
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

	result, resultErr := exportAuthenticatedBrowserState(manager, page, account, cfg.ArtifactDir, "export_state")
	closeManagerProfile(manager, profileDir)
	return result, nil, resultErr
}
