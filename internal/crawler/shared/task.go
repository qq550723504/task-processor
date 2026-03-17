// Package shared 提供爬虫共享类型定义
package shared

import (
	"time"
)

// CrawlerTask 爬虫任务
type CrawlerTask struct {
	TaskID    string    // 任务唯一标识
	URL       string    // 目标 URL
	ASIN      string    // Amazon 产品 ASIN（可选）
	Region    string    // 地区代码（如 us, uk, jp）
	Zipcode   string    // 邮编（可选）
	Priority  int       // 优先级
	CreatedAt time.Time // 创建时间
}

// NewCrawlerTask 创建爬虫任务
func NewCrawlerTask(url string) *CrawlerTask {
	return &CrawlerTask{
		TaskID:    generateTaskID(),
		URL:       url,
		CreatedAt: time.Now(),
	}
}

// WithASIN 设置 ASIN
func (t *CrawlerTask) WithASIN(asin string) *CrawlerTask {
	t.ASIN = asin
	return t
}

// WithRegion 设置地区
func (t *CrawlerTask) WithRegion(region string) *CrawlerTask {
	t.Region = region
	return t
}

// WithZipcode 设置邮编
func (t *CrawlerTask) WithZipcode(zipcode string) *CrawlerTask {
	t.Zipcode = zipcode
	return t
}

// WithPriority 设置优先级
func (t *CrawlerTask) WithPriority(priority int) *CrawlerTask {
	t.Priority = priority
	return t
}

// URLBuilder 用于构造产品 URL 的接口
type URLBuilder interface {
	BuildAmazonProductURL(region, asin string) string
}

// BuildURLFromASIN 根据 ASIN 构造 URL
func (t *CrawlerTask) BuildURLFromASIN(builder URLBuilder) {
	if t.URL == "" && t.ASIN != "" {
		region := t.Region
		if region == "" {
			region = "us"
		}
		t.URL = builder.BuildAmazonProductURL(region, t.ASIN)
	}
}

// Validate 验证任务
func (t *CrawlerTask) Validate() error {
	if t.URL == "" && t.ASIN == "" {
		return ErrInvalidCrawlerTask
	}
	return nil
}

// generateTaskID 生成任务 ID
func generateTaskID() string {
	return time.Now().Format("20060102150405") + "-" + randomString(8)
}

func randomString(n int) string {
	const letters = "abcdefghijklmnopqrstuvwxyz0123456789"
	b := make([]byte, n)
	for i := range b {
		b[i] = letters[time.Now().UnixNano()%int64(len(letters))]
	}
	return string(b)
}
