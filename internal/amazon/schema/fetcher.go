// Package schema 提供Amazon产品类型Schema获取和解析功能
package schema

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"

	"github.com/sirupsen/logrus"
)

// Fetcher Schema获取器
type Fetcher struct {
	httpClient *http.Client
	cache      sync.Map // 缓存已获取的Schema
	logger     *logrus.Entry
}

// NewFetcher 创建Schema获取器
func NewFetcher() *Fetcher {
	return &Fetcher{
		httpClient: &http.Client{Timeout: 60 * time.Second},
		logger:     logrus.WithField("component", "SchemaFetcher"),
	}
}

// FetchSchema 从URL获取完整Schema
func (f *Fetcher) FetchSchema(ctx context.Context, schemaURL string) (*ProductTypeSchema, error) {
	// 检查缓存
	if cached, ok := f.cache.Load(schemaURL); ok {
		return cached.(*ProductTypeSchema), nil
	}

	// 安全地截取URL用于日志显示
	displayURL := schemaURL
	if len(schemaURL) > 80 {
		displayURL = schemaURL[:80] + "..."
	}
	f.logger.WithField("url", displayURL).Info("下载产品类型Schema")

	req, err := http.NewRequestWithContext(ctx, "GET", schemaURL, nil)
	if err != nil {
		return nil, fmt.Errorf("创建请求失败: %w", err)
	}

	resp, err := f.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("下载Schema失败: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("下载Schema失败，状态码: %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("读取响应失败: %w", err)
	}

	var schema ProductTypeSchema
	if err := json.Unmarshal(body, &schema); err != nil {
		return nil, fmt.Errorf("解析Schema失败: %w", err)
	}

	// 缓存结果
	f.cache.Store(schemaURL, &schema)

	return &schema, nil
}

// ClearCache 清除缓存
func (f *Fetcher) ClearCache() {
	f.cache = sync.Map{}
}
