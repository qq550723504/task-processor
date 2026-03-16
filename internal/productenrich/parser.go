// package productenrich 提供产品JSON生成的应用层实现
package productenrich

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"task-processor/internal/pkg/strx"

	"github.com/sirupsen/logrus"
)

// InputParser 输入解析器接口
type InputParser interface {
	ParseInput(ctx context.Context, req *GenerateRequest) (*ParsedInput, error)
	DownloadImages(ctx context.Context, urls []string) ([]string, error)
	CleanText(text string) string
	Scrape1688(ctx context.Context, url string) (*ScrapedData, error)
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

	if config.DownloadDir == "" {
		config.DownloadDir = "./downloads"
	}
	if err := os.MkdirAll(config.DownloadDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create download directory: %w", err)
	}

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
func (p *inputParser) ParseInput(ctx context.Context, req *GenerateRequest) (*ParsedInput, error) {
	if req == nil {
		return nil, fmt.Errorf("request cannot be nil")
	}

	if len(req.ImageURLs) == 0 && req.Text == "" && req.ProductURL == "" {
		return nil, fmt.Errorf("invalid input: at least one of image_urls, text, or product_url must be provided")
	}

	for i, url := range req.ImageURLs {
		if url == "" {
			return nil, fmt.Errorf("invalid input: image_urls[%d] is empty", i)
		}
		if !isValidURL(url) {
			return nil, fmt.Errorf("invalid input: image_urls[%d] is not a valid URL: %s", i, url)
		}
	}

	if req.ProductURL != "" && !isValidURL(req.ProductURL) {
		return nil, fmt.Errorf("invalid input: product_url is not a valid URL: %s", req.ProductURL)
	}

	result := &ParsedInput{
		Images: []string{},
		Text:   "",
	}

	if len(req.ImageURLs) > 0 {
		p.logger.WithField("count", len(req.ImageURLs)).Info("downloading images")
		downloadedPaths, err := p.DownloadImages(ctx, req.ImageURLs)
		if err != nil {
			return nil, fmt.Errorf("failed to download images: %w", err)
		}
		result.Images = downloadedPaths
	}

	if req.Text != "" {
		result.Text = p.CleanText(req.Text)
		if result.Text == "" {
			p.logger.Warn("text became empty after cleaning")
		}
	}

	if req.ProductURL != "" {
		p.logger.WithField("url", req.ProductURL).Info("scraping product URL")
		scrapedData, err := p.Scrape1688(ctx, req.ProductURL)
		if err != nil {
			return nil, fmt.Errorf("failed to scrape product URL: %w", err)
		}
		result.ScrapedData = scrapedData

		if len(scrapedData.Images) > 0 {
			result.Images = append(result.Images, scrapedData.Images...)
		}
		if scrapedData.Description != "" {
			if result.Text != "" {
				result.Text += "\n" + scrapedData.Description
			} else {
				result.Text = scrapedData.Description
			}
		}
	}

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

// CleanText 清洗文本，委托给 pkg/strutil
func (p *inputParser) CleanText(text string) string {
	return strx.CleanProductText(text)
}

// DownloadImages 并发下载多张图片
func (p *inputParser) DownloadImages(ctx context.Context, urls []string) ([]string, error) {
	if len(urls) == 0 {
		return []string{}, nil
	}

	semaphore := make(chan struct{}, p.maxConcurrent)
	var wg sync.WaitGroup
	var mu sync.Mutex

	downloadedPaths := make([]string, 0, len(urls))
	var errs []error

	for i, url := range urls {
		wg.Add(1)
		go func(index int, imageURL string) {
			defer wg.Done()
			semaphore <- struct{}{}
			defer func() { <-semaphore }()

			path, err := p.downloadSingleImage(ctx, imageURL, index)
			mu.Lock()
			defer mu.Unlock()
			if err != nil {
				p.logger.WithError(err).WithField("url", imageURL).Error("failed to download image")
				errs = append(errs, err)
			} else {
				downloadedPaths = append(downloadedPaths, path)
			}
		}(i, url)
	}

	wg.Wait()

	if len(errs) > 0 && len(downloadedPaths) == 0 {
		return nil, fmt.Errorf("all images failed to download: %d errors", len(errs))
	}

	p.logger.WithFields(logrus.Fields{
		"total":   len(urls),
		"success": len(downloadedPaths),
		"failed":  len(errs),
	}).Info("images downloaded")

	return downloadedPaths, nil
}

// downloadSingleImage 下载单张图片（带重试）
func (p *inputParser) downloadSingleImage(ctx context.Context, url string, index int) (string, error) {
	var lastErr error
	for attempt := 0; attempt < p.maxRetries; attempt++ {
		if attempt > 0 {
			select {
			case <-ctx.Done():
				return "", ctx.Err()
			case <-time.After(p.retryDelay * time.Duration(attempt)):
			}
			p.logger.WithFields(logrus.Fields{"url": url, "attempt": attempt + 1}).Warn("retrying image download")
		}
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
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36")

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to download: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	filename := fmt.Sprintf("image_%d_%d.jpg", time.Now().Unix(), index)
	fpath := filepath.Join(p.downloadDir, filename)

	file, err := os.Create(fpath)
	if err != nil {
		return "", fmt.Errorf("failed to create file: %w", err)
	}
	defer file.Close()

	if _, err = io.Copy(file, resp.Body); err != nil {
		os.Remove(fpath)
		return "", fmt.Errorf("failed to save file: %w", err)
	}

	return fpath, nil
}

// Scrape1688 抓取 1688 网页
func (p *inputParser) Scrape1688(ctx context.Context, url string) (*ScrapedData, error) {
	if url == "" {
		return nil, fmt.Errorf("url cannot be empty")
	}

	p.logger.WithField("url", url).Info("starting 1688 scrape")

	data, err := p.webScraper.Scrape(ctx, url)
	if err != nil {
		return nil, fmt.Errorf("web scraper failed: %w", err)
	}

	data.Title = p.CleanText(data.Title)
	data.Description = p.CleanText(data.Description)

	p.logger.WithFields(logrus.Fields{
		"url":    url,
		"title":  data.Title,
		"images": len(data.Images),
		"specs":  len(data.Specs),
	}).Info("1688 scrape completed")

	return data, nil
}

// isValidURL 验证 URL 格式是否有效
func isValidURL(urlStr string) bool {
	if len(urlStr) < 8 {
		return false
	}
	hasHTTP := strings.HasPrefix(urlStr, "http://")
	hasHTTPS := strings.HasPrefix(urlStr, "https://")
	if !hasHTTP && !hasHTTPS {
		return false
	}
	protocolEnd := 7
	if hasHTTPS {
		protocolEnd = 8
	}
	remaining := urlStr[protocolEnd:]
	return remaining != "" && remaining != "/"
}
