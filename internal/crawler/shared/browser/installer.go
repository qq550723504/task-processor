package browser

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"

	"github.com/playwright-community/playwright-go"
	"github.com/sirupsen/logrus"
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

		logrus.Warn("检测到 Playwright 驱动未安装，开始自动安装...")

		// 自动安装驱动
		if installErr := pi.installDriver(); installErr != nil {
			return nil, fmt.Errorf("自动安装 Playwright 驱动失败: %w", installErr)
		}

		logrus.Info("Playwright 驱动安装成功，重新初始化...")

		// 重新尝试运行
		pw, err = playwright.Run()
		if err != nil {
			return nil, fmt.Errorf("初始化playwright失败: %w", err)
		}
	}

	logrus.Info("Playwright 初始化成功")
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

// installDriver 安装 Playwright 驱动
func (pi *PlaywrightInstaller) installDriver() error {
	logrus.Info("开始安装 Playwright 驱动（Chromium）...")

	// 获取 go.mod 中的 playwright 版本
	version, err := pi.getPlaywrightVersion()
	if err != nil {
		logrus.Warnf("无法读取 playwright 版本，使用默认安装方式: %v", err)
		version = ""
	} else {
		logrus.Infof("检测到 playwright-go 版本: %s", version)
	}

	// 构建安装命令
	cmd := pi.buildInstallCommand(version)

	// 设置输出
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	// 执行命令
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("执行安装命令失败: %w", err)
	}

	logrus.Info("Playwright 驱动安装完成")
	return nil
}

// buildInstallCommand 构建安装命令
func (pi *PlaywrightInstaller) buildInstallCommand(version string) *exec.Cmd {
	if version != "" {
		// 使用指定版本安装
		return exec.Command("go", "run",
			fmt.Sprintf("github.com/playwright-community/playwright-go/cmd/playwright@%s", version),
			"install", "chromium")
	}

	// 使用当前项目依赖的版本安装
	return exec.Command("go", "run",
		"github.com/playwright-community/playwright-go/cmd/playwright",
		"install", "chromium")
}

// getPlaywrightVersion 从 go.mod 读取 playwright-go 的版本
func (pi *PlaywrightInstaller) getPlaywrightVersion() (string, error) {
	// 查找 go.mod 文件
	goModPath, err := pi.findGoMod()
	if err != nil {
		return "", err
	}

	// 打开文件
	file, err := os.Open(goModPath)
	if err != nil {
		return "", fmt.Errorf("打开 go.mod 失败: %w", err)
	}
	defer file.Close()

	// 正则匹配 playwright-go 版本
	re := regexp.MustCompile(`github\.com/playwright-community/playwright-go\s+v(\d+\.\d+\.\d+)`)

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		if matches := re.FindStringSubmatch(line); len(matches) > 1 {
			return "v" + matches[1], nil
		}
	}

	if err := scanner.Err(); err != nil {
		return "", fmt.Errorf("读取 go.mod 失败: %w", err)
	}

	return "", fmt.Errorf("未找到 playwright-go 依赖")
}

// findGoMod 查找 go.mod 文件路径
func (pi *PlaywrightInstaller) findGoMod() (string, error) {
	// 从当前工作目录开始向上查找
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}

	for {
		goModPath := filepath.Join(dir, "go.mod")
		if _, err := os.Stat(goModPath); err == nil {
			return goModPath, nil
		}

		// 到达根目录
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}

	return "", fmt.Errorf("未找到 go.mod 文件")
}
