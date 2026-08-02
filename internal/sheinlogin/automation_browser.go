package sheinlogin

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	sharedbrowser "task-processor/internal/crawler/shared/browser"
)

func buildAutomationBrowserConfig(account Account, cfg AutomationConfig) *sharedbrowser.BrowserConfig {
	chromeVersion := strings.TrimSpace(cfg.ChromeVersion)
	if chromeVersion == "" {
		chromeVersion = "144"
	}
	chromeBrandVersion := chromeVersion
	if !strings.Contains(chromeBrandVersion, ".") {
		chromeBrandVersion += ".0.0.0"
	}
	managerCfg := &sharedbrowser.BrowserConfig{
		Headless:                       cfg.Headless,
		BrowserPath:                    strings.TrimSpace(cfg.BrowserPath),
		ChromeVersion:                  chromeVersion,
		ChromeDownloadDir:              strings.TrimSpace(cfg.ChromeDownloadDir),
		ProxyServer:                    automationProxyServer(account),
		ViewportWidth:                  defaultViewport(cfg.ViewportWidth, 1440),
		ViewportHeight:                 defaultViewport(cfg.ViewportHeight, 900),
		UserAgent:                      fmt.Sprintf("Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/%s Safari/537.36", chromeBrandVersion),
		FingerprintSeed:                int32(account.StoreID),
		FingerprintPlatform:            "windows",
		FingerprintPlatformVersion:     "10.0",
		FingerprintBrand:               "Chrome",
		FingerprintBrandVersion:        chromeBrandVersion,
		FingerprintHardwareConcurrency: 8,
		FingerprintGPUVendor:           "NVIDIA Corporation",
		FingerprintGPURenderer:         "ANGLE (NVIDIA, NVIDIA GeForce GTX 1060 Direct3D11 vs_5_0 ps_5_0, D3D11)",
		Language:                       "zh-CN",
		AcceptLanguage:                 "zh-CN,zh;q=0.9,en;q=0.8",
		Timezone:                       "Asia/Shanghai",
		SkipDefaultLaunchArgs:          true,
		UseMinimalFingerprintArgs:      true,
		// product-listing-api runs as root in Kubernetes. Keep the container-safe
		// Chromium flags here instead of relying on Playwright defaults, because
		// this login flow intentionally customizes its launch arguments.
		ExtraLaunchArgs: []string{
			"--no-sandbox",
			"--disable-dev-shm-usage",
			"--enable-unsafe-swiftshader",
		},
	}
	if isCloakBrowserPath(managerCfg.BrowserPath) {
		managerCfg.StealthProvider = sharedbrowser.StealthProviderCloakBrowser
	}
	return managerCfg
}

func automationProxyServer(account Account) string {
	if shouldIgnoreStoreProxy() {
		return ""
	}
	return strings.TrimSpace(account.Proxy)
}

func shouldIgnoreStoreProxy() bool {
	value := strings.TrimSpace(os.Getenv("TASK_PROCESSOR_SHEIN_IGNORE_STORE_PROXY"))
	switch strings.ToLower(value) {
	case "1", "true", "yes", "on":
		return true
	default:
		return false
	}
}

func buildAutomationFingerprint(account Account, cfg *sharedbrowser.BrowserConfig) *sharedbrowser.FingerprintConfig {
	_ = account
	if cfg == nil {
		return nil
	}
	return &sharedbrowser.FingerprintConfig{
		Enable: true,
		GPU: map[string]string{
			"vendor":      cfg.FingerprintGPUVendor,
			"renderer":    cfg.FingerprintGPURenderer,
			"description": cfg.FingerprintGPURenderer,
		},
		Languages: sharedbrowser.LanguageConfig{
			HTTP: cfg.AcceptLanguage,
			JS:   cfg.Language,
		},
	}
}

func isCloakBrowserPath(path string) bool {
	normalized := strings.ToLower(strings.TrimSpace(path))
	return strings.Contains(normalized, "cloakbrowser")
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

const sheinBrowserLaunchTimeout = time.Minute

type browserLaunchManager interface {
	Launch() error
	Close()
}

type blockingStage struct {
	result      chan error
	completed   chan struct{}
	onInterrupt func()
	cleanupOnce sync.Once
	interrupted atomic.Bool
}

func startBlockingStage(fn func() error, onInterrupt func()) *blockingStage {
	stage := &blockingStage{
		result:      make(chan error, 1),
		completed:   make(chan struct{}),
		onInterrupt: onInterrupt,
	}
	go func() {
		err := fn()
		close(stage.completed)
		if stage.interrupted.Load() {
			stage.runInterruptCleanup()
		}
		stage.result <- err
	}()
	return stage
}

func (stage *blockingStage) interrupt() {
	stage.interrupted.Store(true)
	select {
	case <-stage.completed:
		stage.runInterruptCleanup()
	default:
	}
}

func (stage *blockingStage) runInterruptCleanup() {
	if stage.onInterrupt == nil {
		return
	}
	stage.cleanupOnce.Do(stage.onInterrupt)
}

func newOnceCleanup(fn func()) func() {
	if fn == nil {
		return nil
	}
	var once sync.Once
	return func() {
		once.Do(fn)
	}
}

func launchManagerWithProfileRecovery(manager browserLaunchManager, profileDir string, cleanup func()) error {
	return launchManagerWithProfileRecoveryContext(context.Background(), manager, profileDir, cleanup)
}

func launchManagerWithProfileRecoveryContext(ctx context.Context, manager browserLaunchManager, profileDir string, cleanup func()) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	err := launchManagerWithTimeoutAndCleanupContext(ctx, manager, sheinBrowserLaunchTimeout, cleanup)
	if err == nil {
		return nil
	}
	if ctxErr := ctx.Err(); ctxErr != nil {
		return ctxErr
	}
	if !isProfileInUseError(err) {
		return err
	}
	terminateProfileBrowserProcesses(profileDir)
	cleared := clearProfileLockFiles(profileDir)
	if !cleared {
		return fmt.Errorf("SHEIN 浏览器 profile 正在使用，请稍后重试或关闭当前登录窗口: %w", err)
	}
	if ctxErr := ctx.Err(); ctxErr != nil {
		return ctxErr
	}
	if retryErr := launchManagerWithTimeoutAndCleanupContext(ctx, manager, sheinBrowserLaunchTimeout, cleanup); retryErr != nil {
		if ctxErr := ctx.Err(); ctxErr != nil {
			return ctxErr
		}
		if isProfileInUseError(retryErr) {
			return fmt.Errorf("SHEIN 浏览器 profile 正在使用，请稍后重试或关闭当前登录窗口: %w", retryErr)
		}
		return retryErr
	}
	return nil
}

func launchManagerWithTimeout(manager browserLaunchManager, timeout time.Duration) error {
	return launchManagerWithTimeoutAndCleanup(manager, timeout, manager.Close)
}

func launchManagerWithTimeoutAndCleanup(manager browserLaunchManager, timeout time.Duration, cleanup func()) error {
	return launchManagerWithTimeoutAndCleanupContext(context.Background(), manager, timeout, cleanup)
}

func launchManagerWithTimeoutAndCleanupContext(ctx context.Context, manager browserLaunchManager, timeout time.Duration, cleanup func()) error {
	if manager == nil {
		return fmt.Errorf("SHEIN browser manager is nil")
	}
	if timeout <= 0 {
		return manager.Launch()
	}
	if cleanup == nil {
		cleanup = manager.Close
	}

	stage := startBlockingStage(manager.Launch, cleanup)
	timer := time.NewTimer(timeout)
	defer timer.Stop()
	select {
	case err := <-stage.result:
		return err
	case <-ctx.Done():
		stage.interrupt()
		return ctx.Err()
	case <-timer.C:
		stage.interrupt()
		return fmt.Errorf("SHEIN browser launch timed out after %s: %w", timeout, context.DeadlineExceeded)
	}
}

func runBlockingStageWithContext(ctx context.Context, onCancel func(), fn func() error) error {
	if err := ctx.Err(); err != nil {
		if onCancel != nil {
			onCancel()
		}
		return err
	}
	stage := startBlockingStage(fn, onCancel)
	select {
	case err := <-stage.result:
		return err
	case <-ctx.Done():
		stage.interrupt()
		return ctx.Err()
	}
}

func shouldCloseManagerAfterStageError(err error) bool {
	return !errors.Is(err, context.Canceled) && !errors.Is(err, context.DeadlineExceeded)
}

func sleepWithContext(ctx context.Context, wait time.Duration) error {
	if wait <= 0 {
		return nil
	}
	timer := time.NewTimer(wait)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

func closeManagerProfile(manager *sharedbrowser.Manager, profileDir string) {
	if manager != nil {
		manager.Close()
	}
	trimProfileDir(profileDir)
}
