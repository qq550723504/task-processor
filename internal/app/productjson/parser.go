// Package productjson 提供产品JSON生成的应用层实现
package productjson

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"time"

	"task-processor/internal/domain/productjson"

	"github.com/sirupsen/logrus"
)

// InputParser 输入解析器接口
type InputParser interface {
	// ParseInput 解析输入请求
	ParseInput(ctx context.Context, req *productjson.GenerateRequest) (*productjson.ParsedInput, error)
	// DownloadImages 下载图片
	DownloadImages(ctx context.Context, urls []string) ([]string, error)
	// CleanText 清洗文本
	CleanText(text string) string
	// Scrape1688 抓取 1688 网页
	Scrape1688(ctx context.Context, url string) (*productjson.ScrapedData, error)
}

// inputParser 输入解析器实现
type inputParser struct {
	logger        *logrus.Logger
	httpClient    *http.Client
	webScraper    WebScraper
	downloadDir   string
	maxRetries    int
	retryDelay    time.Duration
	maxConcurrent int
}

// InputParserConfig 输入解析器配置
type InputParserConfig struct {
	DownloadDir      string
	MaxRetries       int
	RetryDelay       time.Duration
	Timeout          time.Duration
	MaxConcurrent    int
	ScraperTimeout   time.Duration
	ScraperRetries   int
	ScraperUserAgent string
}

// NewInputParser 创建新的输入解析器
func NewInputParser(logger *logrus.Logger, config *InputParserConfig, webScraper WebScraper) (InputParser, error) {
	if logger == nil {
		return nil, fmt.Errorf("logger cannot be nil")
	}
	if config == nil {
		return nil, fmt.Errorf("config cannot be nil")
	}
	if webScraper == nil {
		return nil, fmt.Errorf("webScraper cannot be nil")
	}

	// 创建下载目录
	if config.DownloadDir == "" {
		config.DownloadDir = "./downloads"
	}
	if err := os.MkdirAll(config.DownloadDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create download directory: %w", err)
	}

	// 设置默认值
	if config.MaxRetries == 0 {
		config.MaxRetries = 3
	}
	if config.RetryDelay == 0 {
		config.RetryDelay = 1 * time.Second
	}
	if config.Timeout == 0 {
		config.Timeout = 30 * time.Second
	}
	if config.MaxConcurrent == 0 {
		config.MaxConcurrent = 5
	}

	// 创建 HTTP 客户端
	httpClient := &http.Client{
		Timeout: config.Timeout,
		Transport: &http.Transport{
			MaxIdleConns:        100,
			MaxIdleConnsPerHost: 10,
			IdleConnTimeout:     90 * time.Second,
		},
	}

	return &inputParser{
		logger:        logger,
		httpClient:    httpClient,
		webScraper:    webScraper,
		downloadDir:   config.DownloadDir,
		maxRetries:    config.MaxRetries,
		retryDelay:    config.RetryDelay,
		maxConcurrent: config.MaxConcurrent,
	}, nil
}

// ParseInput 解析输入请求
// 根据输入类型调用相应的处理方法，支持混合输入处理
// Requirements: 1.3, 1.4, 1.5
func (p *inputParser) ParseInput(ctx context.Context, req *productjson.GenerateRequest) (*productjson.ParsedInput, error) {
	if req == nil {
		return nil, fmt.Errorf("request cannot be nil")
	}

	// 验证输入：至少需要提供一种输入类型
	if len(req.ImageURLs) == 0 && req.Text == "" && req.ProductURL == "" {
		return nil, fmt.Errorf("invalid input: at least one of image_urls, text, or product_url must be provided")
	}

	// 验证图片 URL 格式
	if len(req.ImageURLs) > 0 {
		for i, url := range req.ImageURLs {
			if url == "" {
				return nil, fmt.Errorf("invalid input: image_urls[%d] is empty", i)
			}
			if !isValidURL(url) {
				return nil, fmt.Errorf("invalid input: image_urls[%d] is not a valid URL: %s", i, url)
			}
		}
	}

	// 验证产品 URL 格式
	if req.ProductURL != "" && !isValidURL(req.ProductURL) {
		return nil, fmt.Errorf("invalid input: product_url is not a valid URL: %s", req.ProductURL)
	}

	result := &productjson.ParsedInput{
		Images: []string{},
		Text:   "",
	}

	// 处理图片下载（需求 1.1）
	if len(req.ImageURLs) > 0 {
		p.logger.WithField("count", len(req.ImageURLs)).Info("downloading images")

		downloadedPaths, err := p.DownloadImages(ctx, req.ImageURLs)
		if err != nil {
			return nil, fmt.Errorf("failed to download images: %w", err)
		}
		result.Images = downloadedPaths
	}

	// 处理文本清洗（需求 1.2）
	if req.Text != "" {
		result.Text = p.CleanText(req.Text)
		// 验证清洗后的文本不为空
		if result.Text == "" {
			p.logger.Warn("text became empty after cleaning")
		}
	}

	// 处理 1688 网页抓取（需求 1.3）
	if req.ProductURL != "" {
		p.logger.WithField("url", req.ProductURL).Info("scraping product URL")

		scrapedData, err := p.Scrape1688(ctx, req.ProductURL)
		if err != nil {
			return nil, fmt.Errorf("failed to scrape product URL: %w", err)
		}
		result.ScrapedData = scrapedData

		// 合并抓取的图片到结果中（混合输入处理 - 需求 1.4）
		if len(scrapedData.Images) > 0 {
			result.Images = append(result.Images, scrapedData.Images...)
		}

		// 合并抓取的文本到结果中（混合输入处理 - 需求 1.4）
		if scrapedData.Description != "" {
			if result.Text != "" {
				result.Text += "\n" + scrapedData.Description
			} else {
				result.Text = scrapedData.Description
			}
		}
	}

	// 最终验证：确保至少有一些有效数据
	if len(result.Images) == 0 && result.Text == "" && result.ScrapedData == nil {
		return nil, fmt.Errorf("no valid data extracted from input")
	}

	p.logger.WithFields(logrus.Fields{
		"images":           len(result.Images),
		"has_text":         result.Text != "",
		"has_scraped_data": result.ScrapedData != nil,
	}).Info("input parsing completed")

	return result, nil
}

// DownloadImages 并发下载多张图片
func (p *inputParser) DownloadImages(ctx context.Context, urls []string) ([]string, error) {
	if len(urls) == 0 {
		return []string{}, nil
	}

	// 使用信号量控制并发数
	semaphore := make(chan struct{}, p.maxConcurrent)
	var wg sync.WaitGroup
	var mu sync.Mutex

	downloadedPaths := make([]string, 0, len(urls))
	errors := make([]error, 0)

	for i, url := range urls {
		wg.Add(1)
		go func(index int, imageURL string) {
			defer wg.Done()

			// 获取信号量
			semaphore <- struct{}{}
			defer func() { <-semaphore }()

			// 下载单张图片
			path, err := p.downloadSingleImage(ctx, imageURL, index)

			mu.Lock()
			defer mu.Unlock()

			if err != nil {
				p.logger.WithError(err).WithField("url", imageURL).Error("failed to download image")
				errors = append(errors, err)
			} else {
				downloadedPaths = append(downloadedPaths, path)
			}
		}(i, url)
	}

	wg.Wait()

	// 如果所有图片都下载失败，返回错误
	if len(errors) > 0 && len(downloadedPaths) == 0 {
		return nil, fmt.Errorf("all images failed to download: %d errors", len(errors))
	}

	p.logger.WithFields(logrus.Fields{
		"total":   len(urls),
		"success": len(downloadedPaths),
		"failed":  len(errors),
	}).Info("images downloaded")

	return downloadedPaths, nil
}

// downloadSingleImage 下载单张图片（带重试）
func (p *inputParser) downloadSingleImage(ctx context.Context, url string, index int) (string, error) {
	var lastErr error

	for attempt := 0; attempt < p.maxRetries; attempt++ {
		if attempt > 0 {
			// 重试前等待
			select {
			case <-ctx.Done():
				return "", ctx.Err()
			case <-time.After(p.retryDelay * time.Duration(attempt)):
			}

			p.logger.WithFields(logrus.Fields{
				"url":     url,
				"attempt": attempt + 1,
			}).Warn("retrying image download")
		}

		// 尝试下载
		path, err := p.downloadImageAttempt(ctx, url, index)
		if err == nil {
			return path, nil
		}

		lastErr = err
	}

	return "", fmt.Errorf("failed after %d attempts: %w", p.maxRetries, lastErr)
}

// downloadImageAttempt 单次下载尝试
func (p *inputParser) downloadImageAttempt(ctx context.Context, url string, index int) (string, error) {
	// 创建请求
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	// 设置 User-Agent
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36")

	// 发送请求
	resp, err := p.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to download: %w", err)
	}
	defer resp.Body.Close()

	// 检查状态码
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	// 生成文件名
	filename := fmt.Sprintf("image_%d_%d.jpg", time.Now().Unix(), index)
	filepath := filepath.Join(p.downloadDir, filename)

	// 创建文件
	file, err := os.Create(filepath)
	if err != nil {
		return "", fmt.Errorf("failed to create file: %w", err)
	}
	defer file.Close()

	// 复制内容
	_, err = io.Copy(file, resp.Body)
	if err != nil {
		os.Remove(filepath) // 清理失败的文件
		return "", fmt.Errorf("failed to save file: %w", err)
	}

	return filepath, nil
}

// Scrape1688 抓取 1688 网页
func (p *inputParser) Scrape1688(ctx context.Context, url string) (*productjson.ScrapedData, error) {
	if url == "" {
		return nil, fmt.Errorf("url cannot be empty")
	}

	p.logger.WithField("url", url).Info("starting 1688 scrape")

	// 调用网页抓取器
	data, err := p.webScraper.Scrape(ctx, url)
	if err != nil {
		return nil, fmt.Errorf("web scraper failed: %w", err)
	}

	// 清洗抓取的文本数据
	if data.Title != "" {
		data.Title = p.CleanText(data.Title)
	}
	if data.Description != "" {
		data.Description = p.CleanText(data.Description)
	}

	p.logger.WithFields(logrus.Fields{
		"url":    url,
		"title":  data.Title,
		"images": len(data.Images),
		"specs":  len(data.Specs),
	}).Info("1688 scrape completed")

	return data, nil
}

// CleanText 清洗文本（移除特殊字符、多余空格和换行符）
func (p *inputParser) CleanText(text string) string {
	if text == "" {
		return ""
	}

	// 移除特殊字符（保留中文、英文、数字、基本标点）
	cleaned := removeSpecialChars(text)

	// 移除多余的空格
	cleaned = normalizeSpaces(cleaned)

	// 移除多余的换行符
	cleaned = normalizeNewlines(cleaned)

	// 去除首尾空白
	cleaned = trimWhitespace(cleaned)

	return cleaned
}

// 文本清洗辅助函数

// removeSpecialChars 移除特殊字符
func removeSpecialChars(text string) string {
	// 定义允许的字符：中文、英文、数字、基本标点
	// 移除控制字符和其他特殊符号
	result := make([]rune, 0, len(text))

	for _, r := range text {
		// 保留：
		// - 中文字符 (U+4E00 到 U+9FFF)
		// - 英文字母和数字
		// - 基本标点符号
		// - 空格、换行、制表符
		if (r >= 0x4E00 && r <= 0x9FFF) || // 中文
			(r >= 'a' && r <= 'z') || // 小写字母
			(r >= 'A' && r <= 'Z') || // 大写字母
			(r >= '0' && r <= '9') || // 数字
			r == ' ' || r == '\n' || r == '\r' || r == '\t' || // 空白字符
			r == '.' || r == ',' || r == '!' || r == '?' || // 基本标点
			r == ':' || r == ';' || r == '-' || r == '_' ||
			r == '(' || r == ')' || r == '[' || r == ']' ||
			r == '"' || r == '\'' || r == '/' || r == '\\' {
			result = append(result, r)
		}
	}

	return string(result)
}

// normalizeSpaces 规范化空格（将多个连续空格替换为单个空格）
func normalizeSpaces(text string) string {
	result := make([]rune, 0, len(text))
	lastWasSpace := false

	for _, r := range text {
		isSpace := r == ' ' || r == '\t'

		if isSpace {
			if !lastWasSpace {
				result = append(result, ' ')
				lastWasSpace = true
			}
		} else {
			result = append(result, r)
			lastWasSpace = false
		}
	}

	return string(result)
}

// normalizeNewlines 规范化换行符（将多个连续换行替换为最多两个换行）
func normalizeNewlines(text string) string {
	result := make([]rune, 0, len(text))
	newlineCount := 0

	for _, r := range text {
		if r == '\n' || r == '\r' {
			newlineCount++
			if newlineCount <= 2 {
				result = append(result, '\n')
			}
		} else {
			newlineCount = 0
			result = append(result, r)
		}
	}

	return string(result)
}

// trimWhitespace 去除首尾空白字符
func trimWhitespace(text string) string {
	start := 0
	end := len(text)

	// 找到第一个非空白字符
	for start < end {
		r := rune(text[start])
		if r != ' ' && r != '\t' && r != '\n' && r != '\r' {
			break
		}
		start++
	}

	// 找到最后一个非空白字符
	for end > start {
		r := rune(text[end-1])
		if r != ' ' && r != '\t' && r != '\n' && r != '\r' {
			break
		}
		end--
	}

	return text[start:end]
}

// isValidURL 验证 URL 格式是否有效
func isValidURL(urlStr string) bool {
	if urlStr == "" {
		return false
	}

	// 简单验证：必须以 http:// 或 https:// 开头
	if len(urlStr) < 8 {
		return false
	}

	hasHTTP := len(urlStr) >= 7 && urlStr[:7] == "http://"
	hasHTTPS := len(urlStr) >= 8 && urlStr[:8] == "https://"

	if !hasHTTP && !hasHTTPS {
		return false
	}

	// 验证至少有域名部分
	protocolEnd := 7
	if hasHTTPS {
		protocolEnd = 8
	}

	if len(urlStr) <= protocolEnd {
		return false
	}

	// 检查域名部分不为空
	remaining := urlStr[protocolEnd:]
	if remaining == "" || remaining == "/" {
		return false
	}

	return true
}
