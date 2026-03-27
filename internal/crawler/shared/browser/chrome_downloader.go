package browser

import (
	"archive/zip"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"task-processor/internal/pkg/httpclient"
	"task-processor/internal/pkg/timeout"

	logger "task-processor/internal/core/logger"
)

// ChromeDownloader fingerprint-chromium 下载器
type ChromeDownloader struct {
	version     string // Chrome 版本，如 "144"
	downloadDir string // 下载目录
	httpClient  *httpclient.Client
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
	httpConfig := httpclient.Config{
		Timeout:       10 * time.Minute,
		MaxRetries:    3,
		RetryDelay:    2 * time.Second,
		EnableLogging: true,
		SkipTLSVerify: false,
	}

	return &ChromeDownloader{
		version:     version,
		downloadDir: downloadDir,
		httpClient:  httpclient.New(httpConfig, logger.GetGlobalLogManager().GetRawLogger()),
	}
}

// GetDownloadURL 获取下载链接
// 实际文件名格式参考 https://github.com/adryfish/fingerprint-chromium/releases
func (cd *ChromeDownloader) GetDownloadURL() (string, error) {
	baseURL := "https://github.com/adryfish/fingerprint-chromium/releases/download"

	// 解析完整版本号（配置可能只有主版本号如 "144"，需要查找完整 tag）
	fullVersion, err := cd.resolveFullVersion()
	if err != nil {
		return "", fmt.Errorf("解析版本号失败: %w", err)
	}

	switch runtime.GOOS {
	case "windows":
		return fmt.Sprintf("%s/%s/ungoogled-chromium_%s-1.1_windows_x64.zip", baseURL, fullVersion, fullVersion), nil
	case "linux":
		return fmt.Sprintf("%s/%s/ungoogled-chromium-%s-1-x86_64_linux.tar.xz", baseURL, fullVersion, fullVersion), nil
	case "darwin":
		return fmt.Sprintf("%s/%s/ungoogled-chromium_%s-1.1_macos.dmg", baseURL, fullVersion, fullVersion), nil
	default:
		return "", fmt.Errorf("不支持的操作系统: %s", runtime.GOOS)
	}
}

// resolveFullVersion 将短版本号（如 "144"）解析为完整版本号（如 "144.0.7559.132"）
// 如果已经是完整版本号则直接返回
func (cd *ChromeDownloader) resolveFullVersion() (string, error) {
	// 已经是完整版本号（包含多个点）
	if strings.Count(cd.version, ".") >= 2 {
		return cd.version, nil
	}

	// 通过 GitHub API 查找匹配主版本号的最新 release
	logger.GetGlobalLogger("crawler/shared").Infof("查找 fingerprint-chromium 主版本 %s 对应的完整版本号...", cd.version)

	apiURL := "https://api.github.com/repos/adryfish/fingerprint-chromium/releases"
	req, err := http.NewRequest("GET", apiURL, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Accept", "application/vnd.github.v3+json")

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	resp, err := cd.httpClient.Do(ctx, req)
	if err != nil {
		return "", fmt.Errorf("查询 GitHub API 失败: %w", err)
	}
	defer resp.Body.Close()

	var releases []struct {
		TagName string `json:"tag_name"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&releases); err != nil {
		return "", fmt.Errorf("解析 GitHub API 响应失败: %w", err)
	}

	prefix := cd.version + "."
	for _, r := range releases {
		if strings.HasPrefix(r.TagName, prefix) {
			logger.GetGlobalLogger("crawler/shared").Infof("找到完整版本号: %s", r.TagName)
			return r.TagName, nil
		}
	}

	return "", fmt.Errorf("未找到主版本 %s 对应的 release", cd.version)
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

	logger.GetGlobalLogger("crawler/shared").Infof("开始下载 fingerprint-chromium v%s...", cd.version)
	logger.GetGlobalLogger("crawler/shared").Infof("下载地址: %s", downloadURL)

	// 下载文件
	downloadPath := filepath.Join(cd.downloadDir, filepath.Base(downloadURL))
	if downloadErr := cd.downloadFile(downloadURL, downloadPath); downloadErr != nil {
		return "", fmt.Errorf("下载失败: %w", downloadErr)
	}

	logger.GetGlobalLogger("crawler/shared").Info("下载完成，开始解压...")

	// 解压文件
	extractPath, err := cd.extractFile(downloadPath)
	if err != nil {
		return "", fmt.Errorf("解压失败: %w", err)
	}

	logger.GetGlobalLogger("crawler/shared").Infof("解压完成: %s", extractPath)

	// 查找 Chrome 可执行文件
	chromePath, err := cd.findChromeExecutable(extractPath)
	if err != nil {
		return "", fmt.Errorf("查找 Chrome 可执行文件失败: %w", err)
	}

	logger.GetGlobalLogger("crawler/shared").Infof("Chrome 可执行文件路径: %s", chromePath)
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
	ctx, cancel := timeout.WithDownloadTimeout(context.Background())
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
	logger.GetGlobalLogger("crawler/shared").Infof("开始写入文件: %s", filepath)
	written, err := io.Copy(out, resp.Body)
	if err != nil {
		return fmt.Errorf("写入文件失败: %w", err)
	}

	logger.GetGlobalLogger("crawler/shared").Infof("文件下载完成，大小: %.2f MB", float64(written)/(1024*1024))
	return nil
}

// extractFile 解压文件
func (cd *ChromeDownloader) extractFile(archivePath string) (string, error) {
	extractPath := filepath.Join(cd.downloadDir, fmt.Sprintf("chrome-%s", cd.version))

	// 根据文件扩展名选择解压方式
	if strings.HasSuffix(archivePath, ".zip") {
		return extractPath, cd.extractZip(archivePath, extractPath)
	} else if strings.HasSuffix(archivePath, ".tar.xz") {
		return extractPath, cd.extractTarXZ(archivePath, extractPath)
	} else if strings.HasSuffix(archivePath, ".dmg") {
		return "", fmt.Errorf("dmg 格式需要在 macOS 上手动挂载")
	}

	return "", fmt.Errorf("不支持的压缩格式: %s", archivePath)
}

// extractTarXZ 使用系统 tar 命令解压 .tar.xz 文件
func (cd *ChromeDownloader) extractTarXZ(archivePath, destPath string) error {
	if err := os.MkdirAll(destPath, 0755); err != nil {
		return fmt.Errorf("创建解压目录失败: %w", err)
	}

	cmd := exec.Command("tar", "-xJf", archivePath, "-C", destPath, "--strip-components=1")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("tar 解压失败: %w", err)
	}

	logger.GetGlobalLogger("crawler/shared").Infof("tar.xz 解压完成: %s", destPath)
	return nil
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

	// 保留原始权限（Linux 下 chrome 需要可执行权限）
	mode := f.Mode()
	if mode == 0 {
		mode = 0644
	}

	// 创建目标文件
	outFile, err := os.OpenFile(fpath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, mode)
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
	// 如果配置路径存在且有效，检查是否适用于当前平台
	if configPath != "" {
		// Windows exe 在非 Windows 系统上跳过，走自动下载
		if runtime.GOOS != "windows" && strings.HasSuffix(strings.ToLower(configPath), ".exe") {
			logger.GetGlobalLogger("crawler/shared").Infof("当前系统为 %s，跳过 Windows 浏览器路径: %s，将自动下载 Linux 版本", runtime.GOOS, configPath)
		} else if _, err := os.Stat(configPath); err == nil {
			logger.GetGlobalLogger("crawler/shared").Infof("使用配置的 Chrome 路径: %s", configPath)
			return configPath, nil
		} else {
			logger.GetGlobalLogger("crawler/shared").Warnf("配置的 Chrome 路径不存在: %s", configPath)
		}
	}

	// 检查默认下载位置是否已存在
	extractPath := filepath.Join(cd.downloadDir, fmt.Sprintf("chrome-%s", cd.version))
	chromePath, err := cd.findChromeExecutable(extractPath)
	if err == nil {
		logger.GetGlobalLogger("crawler/shared").Infof("找到已下载的 Chrome: %s", chromePath)
		return chromePath, nil
	}

	// 下载 Chrome
	logger.GetGlobalLogger("crawler/shared").Info("未找到 Chrome，开始自动下载...")
	return cd.Download()
}
