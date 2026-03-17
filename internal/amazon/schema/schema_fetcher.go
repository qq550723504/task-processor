package schema

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"sync"
	"task-processor/internal/pkg/httpclient"
	"task-processor/internal/pkg/jsonx"
	"task-processor/internal/amazon/model"

	"github.com/sirupsen/logrus"
)

// SchemaFetcher Schema获取器
type SchemaFetcher struct {
	httpClient *http.Client
	cache      sync.Map
	logger     *logrus.Entry
}

// NewSchemaFetcher 创建Schema获取器
func NewSchemaFetcher() *SchemaFetcher {
	return &SchemaFetcher{
		httpClient: httpclient.NewSimple(),
		logger:     logrus.WithField("component", "SchemaFetcher"),
	}
}

// FetchSchema 获取Schema
func (f *SchemaFetcher) FetchSchema(ctx context.Context, schemaURL string) (*model.ProductTypeSchema, error) {
	f.logger.WithField("url", schemaURL).Info("开始获取Schema")

	// 检查缓存
	if cached, ok := f.cache.Load(schemaURL); ok {
		if schema, ok := cached.(*model.ProductTypeSchema); ok {
			f.logger.Info("使用缓存的Schema")
			return schema, nil
		}
	}

	// 从URL下载Schema
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

	// 读取响应体
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("读取Schema内容失败: %w", err)
	}

	// 解析JSON
	var schema model.ProductTypeSchema
	if err := jsonx.UnmarshalBytes(body, &schema, "解析Schema JSON失败"); err != nil {
		return nil, err
	}

	// 缓存Schema
	f.cacheSchema(schemaURL, &schema)

	f.logger.WithField("properties_count", len(schema.Properties)).Info("Schema获取成功")
	return &schema, nil
}

// ValidateSchema 验证Schema
func (f *SchemaFetcher) ValidateSchema(schema *model.ProductTypeSchema) error {
	if schema == nil {
		return fmt.Errorf("Schema不能为空")
	}

	if schema.Properties == nil {
		return fmt.Errorf("Schema属性不能为空")
	}

	if len(schema.Properties) == 0 {
		return fmt.Errorf("Schema必须包含至少一个属性")
	}

	f.logger.WithField("properties_count", len(schema.Properties)).Info("Schema验证通过")
	return nil
}

// cacheSchema 缓存Schema
func (f *SchemaFetcher) cacheSchema(url string, schema *model.ProductTypeSchema) {
	f.cache.Store(url, schema)
	f.logger.WithField("url", url).Info("Schema已缓存")
}

// ClearCache 清理缓存
func (f *SchemaFetcher) ClearCache() {
	f.cache = sync.Map{}
	f.logger.Info("Schema获取器缓存已清理")
}

// GetCacheStats 获取缓存统计
func (f *SchemaFetcher) GetCacheStats() map[string]any {
	count := 0
	f.cache.Range(func(key, value any) bool {
		count++
		return true
	})

	return map[string]any{
		"cached_schemas": count,
	}
}

