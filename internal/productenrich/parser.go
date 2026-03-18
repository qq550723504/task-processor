// package productenrich 提供产品JSON生成的应用层实现
package productenrich

import (
	"context"
	"fmt"
	"strings"

	"task-processor/internal/pkg/strx"

	"github.com/sirupsen/logrus"
)

// InputParser 输入解析器接口
type InputParser interface {
	ParseInput(ctx context.Context, req *GenerateRequest) (*ParsedInput, error)
	CollectImages(ctx context.Context, urls []string) ([]string, error)
	CleanText(text string) string
	Scrape1688(ctx context.Context, url string) (*ScrapedData, error)
}

// inputParser 输入解析器实现
type inputParser struct {
	logger     *logrus.Logger
	webScraper WebScraper
}

// InputParserConfig 输入解析器配置（当前保留为空结构体，供后续扩展）
type InputParserConfig struct{}

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

	return &inputParser{
		logger:     logger,
		webScraper: webScraper,
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
		p.logger.WithField("count", len(req.ImageURLs)).Info("collecting image URLs")
		result.Images = req.ImageURLs
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
			// 去重：跳过已存在的图片 URL
			existing := make(map[string]struct{}, len(result.Images))
			for _, u := range result.Images {
				existing[u] = struct{}{}
			}
			for _, u := range scrapedData.Images {
				if _, ok := existing[u]; !ok {
					result.Images = append(result.Images, u)
				}
			}
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

// CollectImages 返回原始图片 URL 列表（内存处理，不做本地下载）
func (p *inputParser) CollectImages(_ context.Context, urls []string) ([]string, error) {
	return urls, nil
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
