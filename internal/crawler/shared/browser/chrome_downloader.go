package browser

import (
	"archive/zip"
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"task-processor/internal/pkg/contextutil"
	"task-processor/internal/pkg/utils"

	"github.com/sirupsen/logrus"
)

// ChromeDownloader fingerprint-chromium 下载器
type ChromeDownloader struct {
	version     string // Chrome 版本，如 "144"
	downloadDir string // 下载目录
	httpClient  *utils.HTTPClient
}

// NewChromeDownloader 创建 Chrome 下载器
func NewChromeDownloader(version, downloadDir string) *ChromeDownloader {
	if version == "" {
		version = "144" // 默认使用最新版本
	}
	if downloadDir == "" {
		downloadDir = filepath.Join(".", "chrome")
	}

	// 创建 HTTP 客户端配置（下载大文件需要更长的超时时间）
	httpConfig := utils.HTTPClientConfig{
		Timeout:       10 * time.Minute, // 下载大文件需要更长超时
		MaxRetries:    3,
		RetryDelay:    2 * time.Second,
		EnableLogging: true,
		SkipTLSVerify: false,
	}

	return &ChromeDownloader{
		version:     version,
		downloadDir: downloadDir,
		httpClient:  utils.NewHTTPClient(httpConfig, logrus.StandardLogger()),
	}
}

// GetDownloadURL 获取下载链接
func (cd *ChromeDownloader) GetDownloadURL() (string, error) {
	// 根据操作系统选择下载链接
	// 注意：这里使用的是 GitHub Release 的直接下载链接格式
	baseURL := "https://github.com/adryfish/fingerprint-chromium/releases/download"

	switch runtime.GOOS {
	case "windows":
		// Windows 使用 ZIP 包
		return fmt.Sprintf("%s/v%s/fingerprint-chromium-%s_windows.zip", baseURL, cd.version, cd.version), nil
	case "linux":
		// Linux 使用 tar.xz 包
		return fmt.Sprintf("%s/v%s/fingerprint-chromium-%s_linux.tar.xz", baseURL, cd.version, cd.version), nil
	case "darwin":
		// macOS 使用 dmg
		return fmt.Sprintf("%s/v%s/fingerprint-chromium-%s_macos.dmg", baseURL, cd.version, cd.version), nil
	default:
		return "", fmt.Errorf("不支持的操作系统: %s", runtime.GOOS)
	}
}

// Download 下载并解压 Chrome
func (cd *ChromeDownloader) Download() (string, error) {
	// 确保下载目录存在
	if err := os.MkdirAll(cd.downloadDir, 0755); err != nil {
		return "", fmt.Errorf("创建下载目录失败: %w", err)
	}

	// 获取下载链接
	downloadURL, err := cd.GetDownloadURL()
	if err != nil {
		return "", err
	}

	logrus.Infof("开始下载 fingerprint-chromium v%s...", cd.version)
	logrus.Infof("下载地址: %s", downloadURL)

	// 下载文件
	downloadPath := filepath.Join(cd.downloadDir, filepath.Base(downloadURL))
	if err := cd.downloadFile(downloadURL, downloadPath); err != nil {
		return "", fmt.Errorf("下载失败: %w", err)
	}

	logrus.Info("下载完成，开始解压...")

	// 解压文件
	extractPath, err := cd.extractFile(downloadPath)
	if err != nil {
		return "", fmt.Errorf("解压失败: %w", err)
	}

	logrus.Infof("解压完成: %s", extractPath)

	// 查找 Chrome 可执行文件
	chromePath, err := cd.findChromeExecutable(extractPath)
	if err != nil {
		return "", fmt.Errorf("查找 Chrome 可执行文件失败: %w", err)
	}

	logrus.Infof("Chrome 可执行文件路径: %s", chromePath)
	return chromePath, nil
}

// downloadFile 下载文件（使用项目的 HTTP 客户端）
func (cd *ChromeDownloader) downloadFile(url, filepath string) error {
	// 创建请求
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return fmt.Errorf("创建请求失败: %w", err)
	}

	// 使用项目的 HTTP 客户端发送请求（带重试机制）
	ctx, cancel := contextutil.WithDownloadTimeout(context.Background())
	defer cancel()

	resp, err := cd.httpClient.Do(ctx, req)
	if err != nil {
		return fmt.Errorf("下载请求失败: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("下载失败，HTTP 状态码: %d", resp.StatusCode)
	}

	// 创建文件
	out, err := os.Create(filepath)
	if err != nil {
		return fmt.Errorf("创建文件失败: %w", err)
	}
	defer out.Close()

	// 写入文件（显示下载进度）
	logrus.Infof("开始写入文件: %s", filepath)
	written, err := io.Copy(out, resp.Body)
	if err != nil {
		return fmt.Errorf("写入文件失败: %w", err)
	}

	logrus.Infof("文件下载完成，大小: %.2f MB", float64(written)/(1024*1024))
	return nil
}

// extractFile 解压文件
func (cd *ChromeDownloader) extractFile(archivePath string) (string, error) {
	extractPath := filepath.Join(cd.downloadDir, fmt.Sprintf("chrome-%s", cd.version))

	// 根据文件扩展名选择解压方式
	if strings.HasSuffix(archivePath, ".zip") {
		return extractPath, cd.extractZip(archivePath, extractPath)
	} else if strings.HasSuffix(archivePath, ".tar.xz") {
		return "", fmt.Errorf("tar.xz 格式需要外部工具解压，请手动解压或使用系统命令")
	} else if strings.HasSuffix(archivePath, ".dmg") {
		return "", fmt.Errorf("dmg 格式需要在 macOS 上手动挂载")
	}

	return "", fmt.Errorf("不支持的压缩格式: %s", archivePath)
}

// extractZip 解压 ZIP 文件
func (cd *ChromeDownloader) extractZip(zipPath, destPath string) error {
	// 打开 ZIP 文件
	r, err := zip.OpenReader(zipPath)
	if err != nil {
		return err
	}
	defer r.Close()

	// 确保目标目录存在
	if err := os.MkdirAll(destPath, 0755); err != nil {
		return err
	}

	// 解压每个文件
	for _, f := range r.File {
		if err := cd.extractZipFile(f, destPath); err != nil {
			return err
		}
	}

	return nil
}

// extractZipFile 解压单个 ZIP 文件
func (cd *ChromeDownloader) extractZipFile(f *zip.File, destPath string) error {
	// 构建目标路径
	fpath := filepath.Join(destPath, f.Name)

	// 检查路径安全性（防止 Zip Slip 攻击）
	if !strings.HasPrefix(fpath, filepath.Clean(destPath)+string(os.PathSeparator)) {
		return fmt.Errorf("非法文件路径: %s", fpath)
	}

	// 创建目录
	if f.FileInfo().IsDir() {
		return os.MkdirAll(fpath, os.ModePerm)
	}

	// 确保父目录存在
	if err := os.MkdirAll(filepath.Dir(fpath), os.ModePerm); err != nil {
		return err
	}

	// 打开源文件
	rc, err := f.Open()
	if err != nil {
		return err
	}
	defer rc.Close()

	// 创建目标文件
	outFile, err := os.OpenFile(fpath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, f.Mode())
	if err != nil {
		return err
	}
	defer outFile.Close()

	// 复制内容
	_, err = io.Copy(outFile, rc)
	return err
}

// findChromeExecutable 查找 Chrome 可执行文件
func (cd *ChromeDownloader) findChromeExecutable(extractPath string) (string, error) {
	var exeName string

	switch runtime.GOOS {
	case "windows":
		exeName = "chrome.exe"
	case "linux":
		exeName = "chrome"
	case "darwin":
		exeName = "Chromium.app/Contents/MacOS/Chromium"
	default:
		return "", fmt.Errorf("不支持的操作系统: %s", runtime.GOOS)
	}

	// 在解压目录中查找可执行文件
	var chromePath string
	err := filepath.Walk(extractPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if strings.HasSuffix(path, exeName) {
			chromePath = path
			return filepath.SkipDir
		}
		return nil
	})

	if err != nil {
		return "", err
	}

	if chromePath == "" {
		return "", fmt.Errorf("未找到 Chrome 可执行文件: %s", exeName)
	}

	return chromePath, nil
}

// CheckAndDownload 检查 Chrome 是否存在，不存在则下载
func (cd *ChromeDownloader) CheckAndDownload(configPath string) (string, error) {
	// 如果配置路径存在且有效，直接返回
	if configPath != "" {
		if _, err := os.Stat(configPath); err == nil {
			logrus.Infof("使用配置的 Chrome 路径: %s", configPath)
			return configPath, nil
		}
		logrus.Warnf("配置的 Chrome 路径不存在: %s", configPath)
	}

	// 检查默认下载位置是否已存在
	extractPath := filepath.Join(cd.downloadDir, fmt.Sprintf("chrome-%s", cd.version))
	chromePath, err := cd.findChromeExecutable(extractPath)
	if err == nil {
		logrus.Infof("找到已下载的 Chrome: %s", chromePath)
		return chromePath, nil
	}

	// 下载 Chrome
	logrus.Info("未找到 Chrome，开始自动下载...")
	return cd.Download()
}
