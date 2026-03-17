// package sync 提供TEMU产品处理工具函数
package sync

import (
	"fmt"
	managementapi "task-processor/internal/infra/clients/management/api"
	"time"

	"github.com/sirupsen/logrus"
)

// parseTime 解析时间字符串或毫秒时间戳
func (s *productSyncServiceImpl) parseTime(timeStr string) (*time.Time, error) {
	if timeStr == "" {
		return nil, fmt.Errorf("时间字符串为空")
	}

	// 首先尝试解析毫秒时间戳
	if len(timeStr) >= 10 && len(timeStr) <= 13 {
		// 检查是否为纯数字
		isNumeric := true
		for _, char := range timeStr {
			if char < '0' || char > '9' {
				isNumeric = false
				break
			}
		}

		if isNumeric {
			// 解析为毫秒时间戳
			var timestamp int64
			if _, err := fmt.Sscanf(timeStr, "%d", &timestamp); err == nil {
				// 如果是10位数字，认为是秒时间戳，转换为毫秒
				if len(timeStr) == 10 {
					timestamp = timestamp * 1000
				}

				// 转换毫秒时间戳为时间
				t := time.UnixMilli(timestamp)

				return &t, nil
			}
		}
	}

	// 如果不是时间戳，尝试解析字符串格式
	formats := []string{
		"2006-01-02 15:04:05",
		"2006-01-02T15:04:05Z",
		"2006-01-02T15:04:05.000Z",
		time.RFC3339,
		"2006-01-02T15:04:05-07:00",
		"2006-01-02 15:04:05.000",
		"2006-01-02T15:04:05+08:00", // 中国时区
		"2006-01-02T15:04:05.000+08:00",
	}

	for _, format := range formats {
		if t, err := time.Parse(format, timeStr); err == nil {
			return &t, nil
		}
	}

	return nil, fmt.Errorf("无法解析时间: %s", timeStr)
}

// calculateProgressInterval 计算进度输出间隔
func (s *productSyncServiceImpl) calculateProgressInterval(totalCount int) int {
	progressInterval := totalCount / 10
	if progressInterval < 10 {
		progressInterval = 10
	}
	if progressInterval > 100 {
		progressInterval = 100
	}
	return progressInterval
}

// logProgress 输出进度日志
func (s *productSyncServiceImpl) logProgress(current, total, success, interval int, operation string) {
	if current%interval == 0 || current == total {
		progress := float64(current) / float64(total) * 100
		s.logger.WithFields(logrus.Fields{
			"processed": current,
			"total":     total,
			"progress":  fmt.Sprintf("%.1f%%", progress),
			"success":   success,
		}).Infof("%s: %d/%d (%.1f%%)", operation, current, total, progress)
	}
}

// validateProductData 验证产品数据的完整性
func (s *productSyncServiceImpl) validateProductData(productData *managementapi.ProductDataDTO) error {
	if productData.PlatformProductID == "" {
		return fmt.Errorf("平台产品ID不能为空")
	}

	if productData.Title == "" {
		return fmt.Errorf("产品标题不能为空")
	}

	if productData.Platform == "" {
		return fmt.Errorf("平台信息不能为空")
	}

	if productData.TenantID <= 0 {
		return fmt.Errorf("租户ID无效")
	}

	if productData.StoreID <= 0 {
		return fmt.Errorf("店铺ID无效")
	}

	return nil
}

// logProductConversionSummary 记录产品转换摘要日志
func (s *productSyncServiceImpl) logProductConversionSummary(totalCount, successCount, failedCount int) {
	s.logger.WithFields(logrus.Fields{
		"total_products": totalCount,
		"success_count":  successCount,
		"failed_count":   failedCount,
		"success_rate":   fmt.Sprintf("%.2f%%", float64(successCount)/float64(totalCount)*100),
	}).Info("TEMU产品转换完成摘要")
}

// mapPlatformStatus 映射平台状态
func (s *productSyncServiceImpl) mapPlatformStatus(status4VO, subStatus4VO int) string {
	switch status4VO {
	case 1:
		return "草稿"
	case 2:
		return "审核中"
	case 3:
		return "已上架"
	case 4:
		return "已下架"
	case 5:
		return "已删除"
	default:
		return fmt.Sprintf("未知状态(%d-%d)", status4VO, subStatus4VO)
	}
}
