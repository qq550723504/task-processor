// Package scheduler 提供TEMU产品转换相关功能
package scheduler

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	managementapi "task-processor/internal/pkg/management/api"
	"task-processor/internal/pkg/types"
	"task-processor/internal/platforms/temu/api/models"

	"github.com/sirupsen/logrus"
)

// convertSingleProduct 转换单个TEMU产品
func (s *productSyncServiceImpl) convertSingleProduct(ctx context.Context, temuProduct *models.TemuProductResponse, tenantID, storeID int64) (*managementapi.ProductDataDTO, error) {
	// 构建基础产品数据
	productData := s.buildBaseProductData(temuProduct, tenantID, storeID)

	// 获取店铺信息
	//storeInfo, err := s.storeAPI.GetStore(storeID)
	// if err != nil {
	// 	s.logger.WithError(err).WithField("store_id", storeID).Warn("获取店铺信息失败，使用默认处理")
	// }

	return productData, nil
}

// buildBaseProductData 构建基础产品数据
func (s *productSyncServiceImpl) buildBaseProductData(temuProduct *models.TemuProductResponse, tenantID, storeID int64) *managementapi.ProductDataDTO {
	var publishTime *time.Time
	if temuProduct.CrtTime != "" {
		if t, err := s.parseTime(temuProduct.CrtTime); err == nil {
			publishTime = t
		}
	}

	var shelfTime *time.Time
	if temuProduct.StatusUpdateTime != "" {
		if t, err := s.parseTime(temuProduct.StatusUpdateTime); err == nil {
			shelfTime = t
		}
	}

	// 构建图片URL列表
	var imageURLs []string
	if temuProduct.ThumbURL != "" {
		imageURLs = append(imageURLs, temuProduct.ThumbURL)
	}
	if temuProduct.SkuPreviewURL != "" && temuProduct.SkuPreviewURL != temuProduct.ThumbURL {
		imageURLs = append(imageURLs, temuProduct.SkuPreviewURL)
	}
	imageURLsJSON, _ := json.Marshal(imageURLs)

	// 构建平台状态信息
	platformStatusJSON, _ := json.Marshal(map[string]interface{}{
		"status4_vo":             temuProduct.Status4Vo,
		"sub_status4_vo":         temuProduct.SubStatus4Vo,
		"show_sub_status4_vo":    temuProduct.ShowSubStatus4Vo,
		"personalization_status": temuProduct.PersonalizationStatus,
		"punish_tags":            temuProduct.PunishTags,
		"stock_display_tag":      temuProduct.StockDisplayTag,
		"low_traffic_tag":        temuProduct.LowTrafficTag,
		"restricted_traffic_tag": temuProduct.RestrictedTrafficTag,
		"easy_gains_tag":         temuProduct.EasyGainsTag,
		"shipping_mode":          temuProduct.ShippingMode,
		"is_books":               temuProduct.IsBooks,
	})

	return &managementapi.ProductDataDTO{
		TenantID:          tenantID,
		StoreID:           storeID,
		Platform:          "TEMU",
		PlatformProductID: temuProduct.GoodsID,
		Title:             temuProduct.GoodsName,
		MainImageURL:      temuProduct.ThumbURL,
		ImageURLs:         string(imageURLsJSON),
		PlatformStatus:    string(platformStatusJSON),
		ShelfStatus:       s.mapShelfStatus(temuProduct.Status4Vo),
		PublishTime:       types.ToFlexibleTime(publishTime),
		ShelfTime:         types.ToFlexibleTime(shelfTime),
	}
}

// mapShelfStatus 映射上架状态
func (s *productSyncServiceImpl) mapShelfStatus(status4Vo int) int {
	switch status4Vo {
	case 3: // TEMU已上架状态
		return 2 // 管理系统上架状态
	case 1, 2, 4, 5: // TEMU其他状态（草稿、审核中、下架等）
		return 3 // 管理系统下架状态
	default:
		return 0 // 未知状态
	}
}

// parsePrice 解析价格字符串
func (s *productSyncServiceImpl) parsePrice(priceStr string) float64 {
	if priceStr == "" {
		return 0
	}

	price, err := strconv.ParseFloat(priceStr, 64)
	if err != nil {
		s.logger.WithError(err).WithField("price_str", priceStr).Warn("解析价格失败")
		return 0
	}
	return price
}

// parseTime 解析时间字符串
func (s *productSyncServiceImpl) parseTime(timeStr string) (*time.Time, error) {
	formats := []string{
		"2006-01-02 15:04:05",
		"2006-01-02T15:04:05Z",
		"2006-01-02T15:04:05.000Z",
		time.RFC3339,
		"2006-01-02T15:04:05-07:00",
		"2006-01-02 15:04:05.000",
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
