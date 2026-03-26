package enrich

import (
	"context"
	"fmt"
	"net/url"
	"strings"

	"task-processor/internal/pkg/strx"
	productenrich "task-processor/internal/productenrich"

	"github.com/sirupsen/logrus"
)

type inputParser struct {
	logger     *logrus.Logger
	webScraper productenrich.WebScraper
}

func NewInputParser(logger *logrus.Logger, config *productenrich.InputParserConfig, webScraper productenrich.WebScraper) (productenrich.InputParser, error) {
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

func (p *inputParser) ParseInput(ctx context.Context, req *productenrich.GenerateRequest) (*productenrich.ParsedInput, error) {
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
	if req.ProductURL != "" && !is1688ProductDetailURL(req.ProductURL) {
		return nil, fmt.Errorf("invalid input: product_url must be a valid 1688 product detail URL: %s", req.ProductURL)
	}

	result := &productenrich.ParsedInput{
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
		if scrapedData == nil {
			return nil, fmt.Errorf("failed to scrape product URL: scraper returned no data")
		}
		result.ScrapedData = scrapedData

		if len(scrapedData.Images) > 0 {
			existing := make(map[string]struct{}, len(result.Images))
			for _, u := range result.Images {
				existing[u] = struct{}{}
			}
			for _, u := range scrapedData.Images {
				if _, ok := existing[u]; !ok {
					result.Images = append(result.Images, u)
					existing[u] = struct{}{}
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

func (p *inputParser) CleanText(text string) string {
	return strx.CleanProductText(text)
}

func (p *inputParser) CollectImages(_ context.Context, urls []string) ([]string, error) {
	return urls, nil
}

func (p *inputParser) Scrape1688(ctx context.Context, url string) (*productenrich.ScrapedData, error) {
	if url == "" {
		return nil, fmt.Errorf("url cannot be empty")
	}

	p.logger.WithField("url", url).Info("starting 1688 scrape")

	data, err := p.webScraper.Scrape(ctx, url)
	if err != nil {
		return nil, fmt.Errorf("web scraper failed: %w", err)
	}
	if data == nil {
		return nil, fmt.Errorf("web scraper returned no data")
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

func is1688ProductDetailURL(rawURL string) bool {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return false
	}
	host := strings.ToLower(parsed.Hostname())
	if host != "detail.1688.com" {
		return false
	}
	path := strings.ToLower(strings.TrimSpace(parsed.EscapedPath()))
	return strings.HasPrefix(path, "/offer/") && len(strings.TrimPrefix(path, "/offer/")) > 0
}
