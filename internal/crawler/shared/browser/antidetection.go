package browser

import (
	"github.com/playwright-community/playwright-go"
	"github.com/sirupsen/logrus"
)

// InjectAntiDetectionScripts 注入反检测脚本
// 注意：Python版本不注入任何JS脚本，只依赖--kfingerprint参数
// 过多的JS注入反而容易被检测到
func InjectAntiDetectionScripts(page playwright.Page) error {
	// Python版本不注入任何脚本，所以这里也不注入
	// 指纹伪装完全依赖Chrome的--kfingerprint参数
	return nil
}

// ApplyAntiDetectionToPage 为页面应用反检测措施
func ApplyAntiDetectionToPage(page playwright.Page, fingerprint *FingerprintConfig) error {
	// 指纹已通过--kfingerprint参数在Chrome启动时注入，无需额外处理
	if fingerprint != nil && fingerprint.Enable {
		logrus.Debug("指纹已通过--kfingerprint参数注入到Chrome")
	}

	// 不注入任何JS脚本，与Python版本保持一致
	return nil
}
