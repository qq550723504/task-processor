package product

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"task-processor/internal/pkg/jsonx"
	models "task-processor/internal/temu/api/product"

	"github.com/sirupsen/logrus"
)

// ProductSubmitUtils 产品提交工具类
type ProductSubmitUtils struct {
	logger *logrus.Entry
}

// NewProductSubmitUtils 创建新的工具类
func NewProductSubmitUtils(logger *logrus.Entry) *ProductSubmitUtils {
	return &ProductSubmitUtils{
		logger: logger,
	}
}

// IsNonRetryableError 判断错误是否不可重试
func (u *ProductSubmitUtils) IsNonRetryableError(errorCode int, errorMessage string) bool {
	// 定义不可重试的错误码
	nonRetryableErrorCodes := map[int]string{
		10000103: "SKU重复错误 - Contribution SKU duplicated",
		10000104: "商品已存在",
		10000105: "商品ID重复",
		// 可以根据实际情况添加更多不可重试的错误码
	}

	// 定义可重试的错误码（即使看起来像配置错误，但我们可以自动修复）
	retryableErrorCodes := map[int]string{
		999999999: "规格模板错误 - 可通过规格完整性验证修复",
		10000414:  "多件套包装配置错误 - 可通过包装验证修复",
	}

	// 检查是否为可重试的错误码
	if reason, exists := retryableErrorCodes[errorCode]; exists {
		u.logger.Infof("识别到可重试错误: %s (error_code=%d)", reason, errorCode)
		return false // 返回false表示可重试
	}

	// 检查错误码
	if reason, exists := nonRetryableErrorCodes[errorCode]; exists {
		u.logger.Infof("识别到不可重试错误: %s (error_code=%d)", reason, errorCode)
		return true
	}

	// 检查错误消息中的关键词
	nonRetryableKeywords := []string{
		"duplicated",
		"duplicate",
		"already exists",
		"重复",
		"已存在",
	}

	for _, keyword := range nonRetryableKeywords {
		if strings.Contains(strings.ToLower(errorMessage), strings.ToLower(keyword)) {
			u.logger.Infof("错误消息包含不可重试关键词: %s", keyword)
			return true
		}
	}

	return false
}

// MarshalWithoutHTMLEscape 序列化JSON但不转义HTML字符
func (u *ProductSubmitUtils) MarshalWithoutHTMLEscape(v any) ([]byte, error) {
	return jsonx.MarshalWithoutHTMLEscape(v)
}

// SaveJSONToFile 保存JSON数据到文件
func (u *ProductSubmitUtils) SaveJSONToFile(taskID string, jsonData []byte, prefix string) error {
	// 创建文件名
	filename := fmt.Sprintf("%s_%s.json", prefix, taskID)

	// 确保目录存在
	if err := os.MkdirAll("logs", 0755); err != nil {
		return fmt.Errorf("创建日志目录失败: %w", err)
	}

	// 写入文件
	filePath := filepath.Join("logs", filename)
	if err := os.WriteFile(filePath, jsonData, 0644); err != nil {
		return fmt.Errorf("写入文件失败: %w", err)
	}

	u.logger.Infof("JSON数据已保存到文件: %s", filePath)
	return nil
}

// GetTotalSkuCount 获取总SKU数量
func (u *ProductSubmitUtils) GetTotalSkuCount(skcList []models.Skc) int {
	total := 0
	for _, skc := range skcList {
		total += len(skc.SkuList)
	}
	return total
}
