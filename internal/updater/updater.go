package updater

import (
	"crypto/sha256"
	"crypto/tls"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"time"

	"github.com/sirupsen/logrus"
)

// VersionInfo 版本信息
type VersionInfo struct {
	Version     string    `json:"version"`      // 版本号
	ReleaseDate time.Time `json:"release_date"` // 发布时间
	DownloadURL string    `json:"download_url"` // 下载地址
	SHA256      string    `json:"sha256"`       // 文件哈希
	Changelog   string    `json:"changelog"`    // 更新日志
	ForceUpdate bool      `json:"force_update"` // 是否强制更新
}

// Updater 自动更新器
type Updater struct {
	currentVersion     string
	updateURL          string        // 版本检查地址
	checkInterval      time.Duration // 检查间隔
	insecureSkipVerify bool          // 跳过TLS证书验证
}

// NewUpdater 创建更新器
func NewUpdater(currentVersion, updateURL string, checkInterval time.Duration, insecureSkipVerify bool) *Updater {
	if insecureSkipVerify {
		logrus.Info("自动更新: TLS证书验证已禁用（避免证书问题导致更新失败）")
	}
	return &Updater{
		currentVersion:     currentVersion,
		updateURL:          updateURL,
		checkInterval:      checkInterval,
		insecureSkipVerify: insecureSkipVerify,
	}
}

// Start 启动自动更新检查
func (u *Updater) Start() {
	logrus.Infof("自动更新器后台任务启动，当前版本: %s", u.currentVersion)

	// 检查是否刚刚更新过（避免更新循环）
	if u.isRecentlyUpdated() {
		logrus.Info("检测到最近刚更新过，跳过启动时的更新检查")
	} else {
		// 延迟30秒后再检查（给程序启动留出时间）
		logrus.Info("将在30秒后进行首次更新检查...")
		time.Sleep(30 * time.Second)
		u.checkAndUpdate()
	}

	ticker := time.NewTicker(u.checkInterval)
	defer ticker.Stop()

	for range ticker.C {
		u.checkAndUpdate()
	}
}

// checkAndUpdate 检查并执行更新
func (u *Updater) checkAndUpdate() {
	logrus.Infof("检查更新... (当前版本: %s)", u.currentVersion)

	// 获取最新版本信息
	latestVersion, err := u.fetchLatestVersion()
	if err != nil {
		logrus.Errorf("获取版本信息失败: %v", err)
		u.saveUpdateError("获取版本信息失败", err)
		return
	}

	logrus.Infof("远程版本: %s", latestVersion.Version)

	// 比较版本 - 使用语义化版本比较
	cmp := compareVersion(latestVersion.Version, u.currentVersion)
	if cmp <= 0 {
		logrus.Infof("当前已是最新版本 (本地: %s, 远程: %s)", u.currentVersion, latestVersion.Version)
		return
	}

	// 检查是否已经更新过这个版本（防止重复更新）
	if u.isAlreadyUpdated(latestVersion.Version) {
		logrus.Infof("版本 %s 已经更新过，跳过", latestVersion.Version)
		return
	}

	logrus.Infof("发现新版本: %s -> %s", u.currentVersion, latestVersion.Version)
	logrus.Infof("更新日志: %s", latestVersion.Changelog)

	// 下载新版本
	if err := u.downloadAndUpdate(latestVersion); err != nil {
		logrus.Errorf("更新失败: %v", err)
		u.saveUpdateError(fmt.Sprintf("更新到版本 %s 失败", latestVersion.Version), err)
		return
	}

	logrus.Info("更新成功，准备重启...")

	// 在重启前标记已更新（重要：必须在重启前创建标记文件）
	u.markAsUpdated(latestVersion.Version)

	// 等待1秒确保文件写入完成
	time.Sleep(1 * time.Second)

	u.restart()
}

// saveUpdateError 保存更新错误到文件
func (u *Updater) saveUpdateError(context string, err error) {
	errorLog := fmt.Sprintf(`========================================
Auto Update Error Log
========================================
Time: %s
Current Version: %s
Context: %s
Error: %v
========================================
`, time.Now().Format("2006-01-02 15:04:05"), u.currentVersion, context, err)

	// 保存到当前目录
	if err := os.WriteFile("update-error.log", []byte(errorLog), 0644); err != nil {
		logrus.Warnf("无法保存更新错误日志: %v", err)
	} else {
		logrus.Info("更新错误日志已保存到: update-error.log")
	}
}

// fetchLatestVersion 获取最新版本信息
func (u *Updater) fetchLatestVersion() (*VersionInfo, error) {
	resp, err := http.Get(u.updateURL)
	if err != nil {
		return nil, fmt.Errorf("请求失败: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP状态码: %d", resp.StatusCode)
	}

	var versionInfo VersionInfo
	if err := json.NewDecoder(resp.Body).Decode(&versionInfo); err != nil {
		return nil, fmt.Errorf("解析版本信息失败: %w", err)
	}

	return &versionInfo, nil
}

// downloadAndUpdate 下载并更新程序
func (u *Updater) downloadAndUpdate(version *VersionInfo) error {
	tmpFile := filepath.Join(os.TempDir(), "task-processor-new.exe")

	// 重试最多3次
	maxRetries := 3
	var lastErr error

	for attempt := 1; attempt <= maxRetries; attempt++ {
		if attempt > 1 {
			logrus.Infof("重试下载 (第 %d/%d 次)...", attempt, maxRetries)
			time.Sleep(time.Duration(attempt) * 2 * time.Second) // 递增延迟
		}

		err := u.downloadFile(version.DownloadURL, tmpFile, version.SHA256)
		if err == nil {
			logrus.Info("文件下载并校验成功")
			return u.replaceExecutable(tmpFile)
		}

		lastErr = err
		logrus.Errorf("下载失败 (第 %d/%d 次): %v", attempt, maxRetries, err)
	}

	return fmt.Errorf("下载失败，已重试 %d 次: %w", maxRetries, lastErr)
}

// downloadFile 下载文件并验证哈希
func (u *Updater) downloadFile(url, destPath, expectedHash string) error {
	logrus.Infof("开始下载: %s", url)

	// 创建带超时的HTTP客户端
	// 创建带超时的HTTP客户端（使用系统代理设置）
	transport := &http.Transport{
		Proxy: http.ProxyFromEnvironment, // 自动使用系统代理
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: u.insecureSkipVerify,
		},
	}

	client := &http.Client{
		Timeout:   10 * time.Minute, // 10分钟超时
		Transport: transport,
	}

	resp, err := client.Get(url)
	if err != nil {
		// 提供更详细的错误信息
		return u.diagnoseDownloadError(url, err)
	}
	defer resp.Body.Close()

	// 检查HTTP状态码
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("HTTP状态码错误: %d %s", resp.StatusCode, resp.Status)
	}

	// 获取文件大小
	contentLength := resp.ContentLength
	if contentLength > 0 {
		logrus.Infof("文件大小: %.2f MB", float64(contentLength)/(1024*1024))
	}

	// 创建临时文件
	out, err := os.Create(destPath)
	if err != nil {
		return fmt.Errorf("创建临时文件失败: %w", err)
	}
	defer out.Close()

	// 边下载边计算哈希
	hash := sha256.New()
	writer := io.MultiWriter(out, hash)

	// 使用带进度的复制
	written, err := u.copyWithProgress(writer, resp.Body, contentLength)
	if err != nil {
		os.Remove(destPath)
		return fmt.Errorf("写入文件失败: %w", err)
	}

	logrus.Infof("下载完成: %.2f MB", float64(written)/(1024*1024))

	// 验证哈希
	downloadedHash := hex.EncodeToString(hash.Sum(nil))
	if downloadedHash != expectedHash {
		os.Remove(destPath)
		return fmt.Errorf("文件哈希不匹配: 期望 %s, 实际 %s", expectedHash, downloadedHash)
	}

	logrus.Info("文件校验通过")
	return nil
}

// copyWithProgress 带进度显示的复制
func (u *Updater) copyWithProgress(dst io.Writer, src io.Reader, total int64) (int64, error) {
	var written int64
	buf := make([]byte, 32*1024) // 32KB buffer
	lastLog := time.Now()

	for {
		nr, er := src.Read(buf)
		if nr > 0 {
			nw, ew := dst.Write(buf[0:nr])
			if nw > 0 {
				written += int64(nw)
			}
			if ew != nil {
				return written, ew
			}
			if nr != nw {
				return written, io.ErrShortWrite
			}

			// 每5秒输出一次进度
			if total > 0 && time.Since(lastLog) > 5*time.Second {
				progress := float64(written) / float64(total) * 100
				logrus.Infof("下载进度: %.2f%% (%.2f/%.2f MB)",
					progress,
					float64(written)/(1024*1024),
					float64(total)/(1024*1024))
				lastLog = time.Now()
			}
		}
		if er != nil {
			if er != io.EOF {
				return written, er
			}
			break
		}
	}
	return written, nil
}

// replaceExecutable 替换可执行文件
func (u *Updater) replaceExecutable(tmpFile string) error {

	// 获取当前程序路径
	currentExe, err := os.Executable()
	if err != nil {
		return fmt.Errorf("获取当前程序路径失败: %w", err)
	}

	// 替换程序（Windows 需要特殊处理）
	if runtime.GOOS == "windows" {
		// Windows 下使用重命名方式：先将当前exe改名为.old作为备份
		oldExe := currentExe + ".old"

		// 删除旧的.old文件（如果存在）
		os.Remove(oldExe)

		// 将当前exe重命名为.old（Windows允许重命名正在运行的exe）
		if err := os.Rename(currentExe, oldExe); err != nil {
			return fmt.Errorf("重命名当前程序失败: %w", err)
		}

		logrus.Info("已备份当前版本为 .old")

		// 将新文件移动到正确位置
		if err := os.Rename(tmpFile, currentExe); err != nil {
			// 如果失败，尝试恢复
			os.Rename(oldExe, currentExe)
			return fmt.Errorf("移动新程序失败: %w", err)
		}

		logrus.Info("程序文件已更新")
		return nil
	}

	// Linux/Mac 先备份再替换
	backupFile := currentExe + ".old"
	os.Remove(backupFile) // 删除旧备份
	if err := copyFile(currentExe, backupFile); err != nil {
		return fmt.Errorf("备份失败: %w", err)
	}
	logrus.Info("已备份当前版本")

	if err := os.Rename(tmpFile, currentExe); err != nil {
		return fmt.Errorf("替换程序失败: %w", err)
	}

	return nil
}

// restart 重启程序
func (u *Updater) restart() {
	currentExe, err := os.Executable()
	if err != nil {
		logrus.Errorf("获取程序路径失败: %v", err)
		return
	}

	// 获取当前工作目录
	workDir, err := os.Getwd()
	if err != nil {
		logrus.Warnf("获取工作目录失败: %v", err)
		workDir = filepath.Dir(currentExe)
	}

	logrus.Info("准备启动新版本程序...")

	if runtime.GOOS == "windows" {
		// Windows: 使用 cmd /c start 在新窗口中启动程序
		// 构建参数
		args := []string{"/c", "start", "Task Processor", "/D", workDir, currentExe}
		args = append(args, os.Args[1:]...)

		cmd := exec.Command("cmd", args...)

		if err := cmd.Start(); err != nil {
			logrus.Errorf("启动新程序失败: %v", err)
			return
		}
	} else {
		// Linux/Mac: 直接启动
		cmd := exec.Command(currentExe, os.Args[1:]...)
		cmd.Dir = workDir

		if err := cmd.Start(); err != nil {
			logrus.Errorf("启动新程序失败: %v", err)
			return
		}
	}

	logrus.Info("新程序已启动，当前程序即将退出...")
	logrus.Info("提示: 旧版本已备份为 .old 文件，确认新版本正常后可手动删除")

	// 刷新日志缓冲区
	time.Sleep(500 * time.Millisecond)

	// 在 Windows 上使用 taskkill 强制终止当前进程
	if runtime.GOOS == "windows" {
		// 获取当前进程 PID
		pid := os.Getpid()
		logrus.Infof("强制终止当前进程 (PID: %d)...", pid)

		// 使用 taskkill 强制终止
		go func() {
			time.Sleep(1 * time.Second)
			exec.Command("taskkill", "/F", "/PID", fmt.Sprintf("%d", pid)).Run()
		}()
	}

	// 正常退出
	time.Sleep(1 * time.Second)
	os.Exit(0)
}

// copyFile 复制文件
func copyFile(src, dst string) error {
	source, err := os.Open(src)
	if err != nil {
		return err
	}
	defer source.Close()

	destination, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer destination.Close()

	_, err = io.Copy(destination, source)
	return err
}

// diagnoseDownloadError 诊断下载错误并提供详细信息
func (u *Updater) diagnoseDownloadError(url string, err error) error {
	errMsg := err.Error()

	// 检查是否是证书相关错误
	if contains(errMsg, "certificate") || contains(errMsg, "x509") || contains(errMsg, "tls") {
		return fmt.Errorf(`下载失败 - TLS/证书错误: %w

可能的原因：
1. 系统缺少根证书
2. 系统时间不正确
3. 防火墙或代理拦截了HTTPS连接

解决方案：
1. 更新Windows系统（安装最新的根证书）
2. 检查系统时间是否正确
3. 在配置文件中临时禁用证书验证（不安全）：
   updater:
     insecureSkipVerify: true

下载地址: %s`, err, url)
	}

	// 检查是否是网络连接错误
	if contains(errMsg, "connection") || contains(errMsg, "timeout") || contains(errMsg, "dial") {
		return fmt.Errorf(`下载失败 - 网络连接错误: %w

可能的原因：
1. 网络不通
2. 防火墙阻止
3. 需要配置代理

解决方案：
1. 检查网络连接: ping auto-update-1303159911.cos.ap-shanghai.myqcloud.com
2. 检查防火墙设置
3. 如果需要代理，设置环境变量:
   set HTTP_PROXY=http://proxy:port
   set HTTPS_PROXY=http://proxy:port

下载地址: %s`, err, url)
	}

	// 其他错误
	return fmt.Errorf("HTTP请求失败: %w\n下载地址: %s", err, url)
}

// contains 检查字符串是否包含子串（不区分大小写）
func contains(s, substr string) bool {
	return len(s) >= len(substr) &&
		(s == substr || len(s) > len(substr) &&
			(s[:len(substr)] == substr || s[len(s)-len(substr):] == substr ||
				len(s) > len(substr)*2 && findSubstring(s, substr)))
}

func findSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// compareVersion 比较两个版本号
// 返回值: 1 表示 v1 > v2, 0 表示 v1 == v2, -1 表示 v1 < v2
func compareVersion(v1, v2 string) int {
	// 移除 'v' 前缀（如果有）
	v1 = trimPrefix(v1, "v")
	v2 = trimPrefix(v2, "v")

	// 如果版本号完全相同，直接返回
	if v1 == v2 {
		return 0
	}

	// 分割版本号
	parts1 := splitVersion(v1)
	parts2 := splitVersion(v2)

	// 比较每个部分
	maxLen := len(parts1)
	if len(parts2) > maxLen {
		maxLen = len(parts2)
	}

	for i := 0; i < maxLen; i++ {
		var n1, n2 int
		if i < len(parts1) {
			n1 = parts1[i]
		}
		if i < len(parts2) {
			n2 = parts2[i]
		}

		if n1 > n2 {
			return 1
		}
		if n1 < n2 {
			return -1
		}
	}

	return 0
}

// splitVersion 分割版本号为数字数组
func splitVersion(version string) []int {
	parts := []int{}
	current := 0
	hasDigit := false

	for i := 0; i < len(version); i++ {
		c := version[i]
		if c >= '0' && c <= '9' {
			current = current*10 + int(c-'0')
			hasDigit = true
		} else if c == '.' || c == '-' {
			if hasDigit {
				parts = append(parts, current)
				current = 0
				hasDigit = false
			}
		}
	}

	if hasDigit {
		parts = append(parts, current)
	}

	return parts
}

// trimPrefix 移除字符串前缀
func trimPrefix(s, prefix string) string {
	if len(s) >= len(prefix) && s[:len(prefix)] == prefix {
		return s[len(prefix):]
	}
	return s
}

// isAlreadyUpdated 检查是否已经更新过指定版本
func (u *Updater) isAlreadyUpdated(version string) bool {
	markerFile := ".update-marker"
	data, err := os.ReadFile(markerFile)
	if err != nil {
		return false
	}
	return string(data) == version
}

// isRecentlyUpdated 检查是否最近刚更新过（1小时内）
func (u *Updater) isRecentlyUpdated() bool {
	markerFile := ".update-marker"
	info, err := os.Stat(markerFile)
	if err != nil {
		return false
	}

	// 如果标记文件是1小时内创建的，认为是最近更新过
	timeSinceUpdate := time.Since(info.ModTime())
	if timeSinceUpdate < 1*time.Hour {
		logrus.Infof("检测到 %v 前更新过", timeSinceUpdate.Round(time.Second))
		return true
	}

	return false
}

// markAsUpdated 标记已更新到指定版本
func (u *Updater) markAsUpdated(version string) {
	markerFile := ".update-marker"
	content := fmt.Sprintf("%s\n%s", version, time.Now().Format(time.RFC3339))
	if err := os.WriteFile(markerFile, []byte(content), 0644); err != nil {
		logrus.Warnf("无法创建更新标记文件: %v", err)
	} else {
		logrus.Infof("已标记更新到版本: %s", version)
	}
}
