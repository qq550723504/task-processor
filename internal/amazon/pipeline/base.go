// package pipeline 提供基础Handler实现，减少重复代码
package pipeline

import (
	"fmt"
	"task-processor/internal/amazon/api"
	"task-processor/internal/amazon/model"

		"task-processor/internal/core/logger"
	"github.com/sirupsen/logrus"
)

// BaseHandler 基础处理器
// 提供通用功能，减少重复代码
type BaseHandler struct {
	name   string
	logger *logrus.Entry
}

// NewBaseHandler 创建基础处理器
func NewBaseHandler(name string) *BaseHandler {
	return &BaseHandler{
		name:   name,
		logger: logger.GetGlobalLogger("amazon/pipeline").WithField("handler", name),
	}
}

// Name 返回处理器名称
func (bh *BaseHandler) Name() string {
	return bh.name
}

// GetLogger 获取日志器
func (bh *BaseHandler) GetLogger() *logrus.Entry {
	return bh.logger
}

// ValidateServices 验证服务集合
func (bh *BaseHandler) ValidateServices(services *model.Services) error {
	if services == nil {
		return fmt.Errorf("服务集合为空")
	}
	return nil
}

// GetAPIClient 获取API客户端
func (bh *BaseHandler) GetAPIClient(services *model.Services) (*api.Client, error) {
	if services.APIClient == nil {
		return nil, fmt.Errorf("Amazon API客户端未初始化")
	}

	return services.APIClient, nil
}

// GetRequiredString 获取必需的字符串字段
func (bh *BaseHandler) GetRequiredString(data map[string]any, key string) (string, error) {
	value, exists := data[key]
	if !exists {
		return "", fmt.Errorf("缺少必需字段: %s", key)
	}

	str, ok := value.(string)
	if !ok {
		return "", fmt.Errorf("字段 %s 格式错误，期望字符串", key)
	}

	return str, nil
}

// GetRequiredInt64 获取必需的int64字段
func (bh *BaseHandler) GetRequiredInt64(data map[string]any, key string) (int64, error) {
	value, exists := data[key]
	if !exists {
		return 0, fmt.Errorf("缺少必需字段: %s", key)
	}

	switch v := value.(type) {
	case int64:
		return v, nil
	case int:
		return int64(v), nil
	case float64:
		return int64(v), nil
	default:
		return 0, fmt.Errorf("字段 %s 格式错误，期望数字", key)
	}
}

// GetOptionalString 获取可选的字符串字段
func (bh *BaseHandler) GetOptionalString(data map[string]any, key, defaultValue string) string {
	if value, exists := data[key]; exists {
		if str, ok := value.(string); ok {
			return str
		}
	}
	return defaultValue
}

// SetResult 设置处理结果
func (bh *BaseHandler) SetResult(data map[string]any, key string, value any) {
	data[key] = value
	bh.logger.Debugf("设置结果: %s = %v", key, value)
}
