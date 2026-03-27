package browser

import (
	"task-processor/internal/core/logger"
	"fmt"
	"regexp"

	"github.com/playwright-community/playwright-go"
)

// PlaywrightInstaller Playwright 驱动安装器
type PlaywrightInstaller struct{}

// NewPlaywrightInstaller 创建安装器实例
func NewPlaywrightInstaller() *PlaywrightInstaller {
	return &PlaywrightInstaller{}
}

// InstallIfNeeded 初始化Playwright（自动安装驱动）
func (pi *PlaywrightInstaller) InstallIfNeeded() (*playwright.Playwright, error) {
	// 先尝试运行 Playwright
	pw, err := playwright.Run()
	if err != nil {
		// 如果失败，检查是否是驱动未安装的错误
		if !pi.isDriverNotInstalledError(err) {
			return nil, fmt.Errorf("初始化playwright失败: %w", err)
		}

		logger.GetGlobalLogger("crawler/shared").Warn("检测到 Playwright 驱动未安装，开始自动安装...")

		// 使用 playwright-go 原生 API 安装驱动，无需 go 环境
		if installErr := playwright.Install(&playwright.RunOptions{
			Browsers: []string{"chromium"},
		}); installErr != nil {
			return nil, fmt.Errorf("自动安装 Playwright 驱动失败: %w", installErr)
		}

		logger.GetGlobalLogger("crawler/shared").Info("Playwright 驱动安装成功，重新初始化...")

		// 重新尝试运行
		pw, err = playwright.Run()
		if err != nil {
			return nil, fmt.Errorf("初始化playwright失败: %w", err)
		}
	}

	logger.GetGlobalLogger("crawler/shared").Info("Playwright 初始化成功")
	return pw, nil
}

// isDriverNotInstalledError 检查是否是驱动未安装的错误
func (pi *PlaywrightInstaller) isDriverNotInstalledError(err error) bool {
	if err == nil {
		return false
	}
	errMsg := err.Error()
	return regexp.MustCompile(`please install the driver`).MatchString(errMsg) ||
		regexp.MustCompile(`driver.*not.*found`).MatchString(errMsg)
}
