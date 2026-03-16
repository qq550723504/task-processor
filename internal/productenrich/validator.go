// package productenrich 提供产品JSON生成的应用层实现
package productenrich

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"


	"github.com/sirupsen/logrus"
)

// InputValidator 输入验证器接口
type InputValidator interface {
	// Validate 验证输入数据
	Validate(ctx context.Context, input *ParsedInput) (*ValidationResult, error)
	// ValidateImages 验证图片数据
	ValidateImages(ctx context.Context, imageURLs []string) (*ImageValidation, error)
	// ValidateText 验证文本数据
	ValidateText(ctx context.Context, text string) (*TextValidation, error)
	// ValidateScrapedData 验证抓取数据
	ValidateScrapedData(ctx context.Context, data *ScrapedData) (*ScrapedDataValidation, error)
}

// inputValidator 输入验证器实现
type inputValidator struct {
	httpClient *http.Client
	maxWorkers int
	metrics    MetricsCollector
	cache      ValidationCache
	cacheTTL   time.Duration
}

// InputValidatorConfig 输入验证器配置
type InputValidatorConfig struct {
	HTTPTimeout time.Duration // HTTP 请求超时时间
	MaxWorkers  int           // 最大并发工作数
	RedisClient RedisClient   // Redis 客户端（可选，用于缓存）
	CacheTTL    time.Duration // 缓存过期时间
	EnableCache bool          // 是否启用缓存
	Metrics     MetricsCollector
}

// NewInputValidator 创建新的输入验证器
func NewInputValidator(config *InputValidatorConfig) InputValidator {
	if config == nil {
		config = &InputValidatorConfig{
			HTTPTimeout: 5 * time.Second,
			MaxWorkers:  10,
			CacheTTL:    24 * time.Hour,
			EnableCache: false,
		}
	}

	if config.HTTPTimeout == 0 {
		config.HTTPTimeout = 5 * time.Second
	}

	if config.MaxWorkers == 0 {
		config.MaxWorkers = 10
	}

	if config.CacheTTL == 0 {
		config.CacheTTL = 24 * time.Hour
	}

	validator := &inputValidator{
		httpClient: &http.Client{
			Timeout: config.HTTPTimeout,
		},
		maxWorkers: config.MaxWorkers,
		metrics:    config.Metrics,
		cacheTTL:   config.CacheTTL,
	}

	// 如果启用缓存且提供了 Redis 客户端，则创建缓存
	if config.EnableCache && config.RedisClient != nil {
		validator.cache = NewValidationCache(config.RedisClient, config.Metrics)
		logrus.WithField("ttl", config.CacheTTL).Info("validation cache enabled")
	}

	return validator
}

// Validate 验证输入数据
func (v *inputValidator) Validate(ctx context.Context, input *ParsedInput) (*ValidationResult, error) {
	if input == nil {
		return nil, fmt.Errorf("input cannot be nil")
	}

	result := &ValidationResult{
		IsValid: true,
		Issues:  make([]ValidationIssue, 0),
	}

	// 验证图片
	imageValidation, err := v.ValidateImages(ctx, input.Images)
	if err != nil {
		logrus.WithError(err).Error("failed to validate images")
		result.Issues = append(result.Issues, ValidationIssue{
			Field:    "images",
			Severity: SeverityError,
			Message:  fmt.Sprintf("图片验证失败: %v", err),
			Code:     "IMAGE_VALIDATION_ERROR",
		})
		if v.metrics != nil {
			v.metrics.RecordCacheOperation("validation_issue", SeverityError)
		}
	} else {
		result.ImageScore = float64(imageValidation.ValidCount) * 20
	}

	// 验证文本
	textValidation, err := v.ValidateText(ctx, input.Text)
	if err != nil {
		logrus.WithError(err).Error("failed to validate text")
		result.Issues = append(result.Issues, ValidationIssue{
			Field:    "text",
			Severity: SeverityError,
			Message:  fmt.Sprintf("文本验证失败: %v", err),
			Code:     "TEXT_VALIDATION_ERROR",
		})
	} else {
		result.TextScore = float64(textValidation.Length) / 2
		if result.TextScore > 100 {
			result.TextScore = 100
		}
	}

	// 验证抓取数据
	if input.ScrapedData != nil {
		scrapedValidation, err := v.ValidateScrapedData(ctx, input.ScrapedData)
		if err != nil {
			logrus.WithError(err).Error("failed to validate scraped data")
		} else {
			score := 0.0
			if scrapedValidation.HasTitle {
				score += 20
			}
			if scrapedValidation.HasDescription {
				score += 20
			}
			if scrapedValidation.HasImages {
				score += 20
			}
			result.ScrapedScore = score
		}
	}

	// 计算总体质量评分
	result.QualityScore = (result.ImageScore + result.TextScore + result.ScrapedScore) / 3

	// 检查是否有严重错误
	for _, issue := range result.Issues {
		if issue.Severity == SeverityError {
			result.IsValid = false
			break
		}
	}

	return result, nil
}

// ValidateImages 验证图片数据
func (v *inputValidator) ValidateImages(ctx context.Context, imageURLs []string) (*ImageValidation, error) {
	validation := &ImageValidation{
		TotalCount:  len(imageURLs),
		ValidImages: make([]ImageInfo, 0),
	}

	if len(imageURLs) == 0 {
		return validation, nil
	}

	// 使用 goroutine pool 并发验证图片
	resultChan := make(chan ImageInfo, len(imageURLs))
	semaphore := make(chan struct{}, v.maxWorkers)
	var wg sync.WaitGroup

	for _, imageURL := range imageURLs {
		wg.Add(1)
		go func(url string) {
			defer wg.Done()

			// 获取信号量
			semaphore <- struct{}{}
			defer func() { <-semaphore }()

			info := v.validateSingleImage(ctx, url)
			resultChan <- info
		}(imageURL)
	}

	// 等待所有验证完成
	go func() {
		wg.Wait()
		close(resultChan)
	}()

	// 收集结果
	for info := range resultChan {
		if info.IsValid {
			validation.ValidImages = append(validation.ValidImages, info)
			validation.ValidCount++
		}
	}

	return validation, nil
}

// validateSingleImage 验证单个图片
func (v *inputValidator) validateSingleImage(ctx context.Context, imageURL string) ImageInfo {
	// 尝试从缓存获取
	if v.cache != nil {
		if cachedInfo, found := v.cache.GetImageValidation(ctx, imageURL); found {
			return *cachedInfo
		}
	}

	info := ImageInfo{
		URL:     imageURL,
		IsValid: false,
	}

	// 验证 URL 格式
	parsedURL, err := url.Parse(imageURL)
	if err != nil {
		info.Error = fmt.Sprintf("URL 格式无效: %v", err)
		v.cacheImageInfo(ctx, imageURL, &info)
		return info
	}

	if parsedURL.Scheme != "http" && parsedURL.Scheme != "https" {
		info.Error = "URL 必须使用 HTTP 或 HTTPS 协议"
		v.cacheImageInfo(ctx, imageURL, &info)
		return info
	}

	// 检查图片格式
	format := v.getImageFormat(imageURL)
	info.Format = format
	if format != "jpg" && format != "jpeg" && format != "png" && format != "webp" {
		info.Error = fmt.Sprintf("不支持的图片格式: %s", format)
		v.cacheImageInfo(ctx, imageURL, &info)
		return info
	}

	// 发送 HTTP HEAD 请求检查可访问性
	req, err := http.NewRequestWithContext(ctx, http.MethodHead, imageURL, nil)
	if err != nil {
		info.Error = fmt.Sprintf("创建请求失败: %v", err)
		v.cacheImageInfo(ctx, imageURL, &info)
		return info
	}

	resp, err := v.httpClient.Do(req)
	if err != nil {
		info.Error = fmt.Sprintf("无法访问图片: %v", err)
		v.cacheImageInfo(ctx, imageURL, &info)
		return info
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusOK {
		info.IsValid = true
	} else {
		info.Error = fmt.Sprintf("HTTP 错误: %d", resp.StatusCode)
	}

	// 缓存验证结果
	v.cacheImageInfo(ctx, imageURL, &info)

	return info
}

// cacheImageInfo 缓存图片验证信息
func (v *inputValidator) cacheImageInfo(ctx context.Context, imageURL string, info *ImageInfo) {
	if v.cache != nil {
		if err := v.cache.SetImageValidation(ctx, imageURL, info, v.cacheTTL); err != nil {
			logrus.WithFields(logrus.Fields{
				"url": imageURL,
			}).WithError(err).Warn("failed to cache image validation result")
		}
	}
}

// getImageFormat 从 URL 获取图片格式
func (v *inputValidator) getImageFormat(imageURL string) string {
	lower := strings.ToLower(imageURL)
	if strings.Contains(lower, ".jpg") || strings.Contains(lower, ".jpeg") {
		return "jpg"
	}
	if strings.Contains(lower, ".png") {
		return "png"
	}
	if strings.Contains(lower, ".webp") {
		return "webp"
	}
	return "unknown"
}

// ValidateText 验证文本数据
func (v *inputValidator) ValidateText(ctx context.Context, text string) (*TextValidation, error) {
	validation := &TextValidation{
		Length: len(text),
	}

	// 提取关键词（简单实现：按空格分割）
	if text != "" {
		words := strings.Fields(text)
		if len(words) > 0 {
			validation.HasKeywords = true
			// 取前 10 个词作为关键词
			maxKeywords := 10
			if len(words) < maxKeywords {
				maxKeywords = len(words)
			}
			validation.Keywords = words[:maxKeywords]
		}
	}

	return validation, nil
}

// ValidateScrapedData 验证抓取数据
func (v *inputValidator) ValidateScrapedData(ctx context.Context, data *ScrapedData) (*ScrapedDataValidation, error) {
	if data == nil {
		return nil, fmt.Errorf("scraped data cannot be nil")
	}

	validation := &ScrapedDataValidation{
		HasTitle:       data.Title != "",
		HasDescription: data.Description != "",
		HasImages:      len(data.Images) > 0,
		HasSpecs:       len(data.Specs) > 0,
		ImageCount:     len(data.Images),
	}

	return validation, nil
}
