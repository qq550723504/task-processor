// Package updater 提供自动更新器的文件下载功能
package updater

import (
	"crypto/sha256"
	"crypto/tls"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"task-processor/internal/pkg/httpclient"
	"task-processor/internal/pkg/strutil"
	"time"

	"github.com/sirupsen/logrus"
)

// FileDownloader 文件下载器
type FileDownloader struct {
	insecureSkipVerify bool
}

// NewFileDownloader 创建文件下载器
func NewFileDownloader(insecureSkipVerify bool) *FileDownloader {
	return &FileDownloader{
		insecureSkipVerify: insecureSkipVerify,
	}
}

// DownloadFile 下载文件并验证哈希
func (fd *FileDownloader) DownloadFile(url, destPath, expectedHash string) error {
	logrus.Infof("开始下载: %s", url)

	// 创建带超时的HTTP客户端（使用系统代理设置）
	transport := &http.Transport{
		Proxy: http.ProxyFromEnvironment, // 自动使用系统代理
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: fd.insecureSkipVerify,
		},
	}

	client := httpclient.NewWithTransport(10*time.Minute, transport)

	resp, err := client.Get(url)
	if err != nil {
		// 提供更详细的错误信息
		return fd.diagnoseDownloadError(url, err)
	}
	defer func() {
		_ = resp.Body.Close()
	}()

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
	defer func() {
		_ = out.Close()
	}()

	// 边下载边计算哈希
	hash := sha256.New()
	writer := io.MultiWriter(out, hash)

	// 使用带进度的复制
	written, err := fd.copyWithProgress(writer, resp.Body, contentLength)
	if err != nil {
		_ = os.Remove(destPath)
		return fmt.Errorf("写入文件失败: %w", err)
	}

	logrus.Infof("下载完成: %.2f MB", float64(written)/(1024*1024))

	// 验证哈希
	downloadedHash := hex.EncodeToString(hash.Sum(nil))
	if downloadedHash != expectedHash {
		_ = os.Remove(destPath)
		return fmt.Errorf("文件哈希不匹配: 期望 %s, 实际 %s", expectedHash, downloadedHash)
	}

	logrus.Info("文件校验通过")
	return nil
}

// DownloadWithRetry 带重试的下载
func (fd *FileDownloader) DownloadWithRetry(url, destPath, expectedHash string, maxRetries int) error {
	var lastErr error

	for attempt := 1; attempt <= maxRetries; attempt++ {
		if attempt > 1 {
			logrus.Infof("重试下载 (第 %d/%d 次)...", attempt, maxRetries)
			time.Sleep(time.Duration(attempt) * 2 * time.Second) // 递增延迟
		}

		err := fd.DownloadFile(url, destPath, expectedHash)
		if err == nil {
			logrus.Info("文件下载并校验成功")
			return nil
		}

		lastErr = err
		logrus.Errorf("下载失败 (第 %d/%d 次): %v", attempt, maxRetries, err)
	}

	return fmt.Errorf("下载失败，已重试 %d 次: %w", maxRetries, lastErr)
}

// copyWithProgress 带进度显示的复制
func (fd *FileDownloader) copyWithProgress(dst io.Writer, src io.Reader, total int64) (int64, error) {
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

// diagnoseDownloadError 诊断下载错误
func (fd *FileDownloader) diagnoseDownloadError(_ string, err error) error {
	errMsg := err.Error()

	// 常见错误诊断
	if strutil.Contains(errMsg, "no such host") || strutil.Contains(errMsg, "cannot resolve") {
		return fmt.Errorf("DNS解析失败，请检查网络连接或DNS设置: %w", err)
	}

	if strutil.Contains(errMsg, "connection refused") {
		return fmt.Errorf("连接被拒绝，服务器可能不可用: %w", err)
	}

	if strutil.Contains(errMsg, "timeout") || strutil.Contains(errMsg, "deadline exceeded") {
		return fmt.Errorf("连接超时，请检查网络连接或稍后重试: %w", err)
	}

	if strutil.Contains(errMsg, "certificate") || strutil.Contains(errMsg, "tls") {
		return fmt.Errorf("TLS证书验证失败: %w (提示: 可以在配置中设置 insecure_skip_verify: true 跳过证书验证)", err)
	}

	if strutil.Contains(errMsg, "proxy") {
		return fmt.Errorf("代理连接失败，请检查代理设置: %w", err)
	}

	// 默认错误信息
	return fmt.Errorf("下载失败: %w", err)
}

// GetTempFilePath 获取临时文件路径
func (fd *FileDownloader) GetTempFilePath() string {
	return filepath.Join(os.TempDir(), "task-processor-new.exe")
}
