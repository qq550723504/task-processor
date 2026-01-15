package utils

import (
	"encoding/json"
	"task-processor/internal/pkg/management/api"
	"task-processor/internal/pkg/types"
	"task-processor/internal/platforms/shein/model"
	"time"

	"github.com/sirupsen/logrus"
)

// MapToProductData 将 SHEIN 产品数据映射为通用产品数据
func MapToProductData(sheinProduct *model.SheinProductResponse, tenantID, storeID int64) (*api.ProductDataDTO, error) {
	now := time.Now()

	// 解析发布时间，失败时设置为 nil
	var publishTime *time.Time
	if sheinProduct.PublishTime != "" {
		parsedTime, err := model.ParseTime(sheinProduct.PublishTime)
		if err != nil {
			logrus.WithFields(logrus.Fields{
				"spu_code":     sheinProduct.SpuCode,
				"publish_time": sheinProduct.PublishTime,
				"error":        err,
			}).Warn("解析发布时间失败")
			publishTime = nil
		} else {
			publishTime = parsedTime
		}
	} else {
		logrus.WithField("spu_code", sheinProduct.SpuCode).Debug("发布时间为空")
	}

	// 解析上架时间，失败时设置为 nil
	var shelfTime *time.Time
	if sheinProduct.FirstShelfTime != "" {
		parsedTime, err := model.ParseTime(sheinProduct.FirstShelfTime)
		if err != nil {
			logrus.WithFields(logrus.Fields{
				"spu_code":         sheinProduct.SpuCode,
				"first_shelf_time": sheinProduct.FirstShelfTime,
				"error":            err,
			}).Warn("解析上架时间失败")
			shelfTime = nil
		} else {
			shelfTime = parsedTime
		}
	} else {
		logrus.WithField("spu_code", sheinProduct.SpuCode).Debug("上架时间为空")
	}

	// 获取主图（第一个 SKC 的图片）
	mainImageURL := ""
	if len(sheinProduct.SkcInfoList) > 0 {
		mainImageURL = sheinProduct.SkcInfoList[0].MainImageThumbnailURL
	}

	// 收集所有 SKC 的图片
	var imageURLs []string
	for _, skc := range sheinProduct.SkcInfoList {
		if skc.MainImageThumbnailURL != "" {
			imageURLs = append(imageURLs, skc.MainImageThumbnailURL)
		}
	}
	imageURLsJSON, _ := json.Marshal(imageURLs)

	// 将完整的 SKC 信息存入 attributes
	attributesJSON, _ := json.Marshal(sheinProduct.SkcInfoList)

	// 平台状态
	platformStatusJSON, _ := json.Marshal(map[string]interface{}{
		"shelf_status": sheinProduct.ShelfStatus,
	})

	// SHEIN 产品列表 API 不返回价格和库存信息
	// 这些信息需要从产品详情 API 获取，暂时设置为默认值
	originalPrice := "0.00"
	stock := "0"

	return &api.ProductDataDTO{
		Platform:          "SHEIN",
		PlatformProductID: sheinProduct.SpuCode,
		TenantID:          tenantID,
		StoreID:           storeID,
		Title:             sheinProduct.ProductNameMulti,
		CategoryID:        sheinProduct.CategoryID,
		Brand:             sheinProduct.BrandName,
		OriginalPrice:     types.FlexibleString(originalPrice),
		Stock:             types.FlexibleString(stock),
		MainImageURL:      mainImageURL,
		ImageURLs:         string(imageURLsJSON),
		Attributes:        string(attributesJSON),
		PlatformStatus:    string(platformStatusJSON),
		ShelfStatus:       MapShelfStatus(sheinProduct.ShelfStatus),
		PublishTime:       types.ToFlexibleTime(publishTime),
		ShelfTime:         types.ToFlexibleTime(shelfTime),
		LastSyncTime:      types.ToFlexibleTime(&now),
		PlatformData:      "", // 留空，不存储完整原始数据
		Status:            2,  // 已上架
	}, nil
}

// MapShelfStatus 映射 SHEIN 上架状态到统一状态
func MapShelfStatus(shelfStatus string) int {
	switch shelfStatus {
	case "ON_SHELF":
		return api.ShelfStatusOnShelf // 2
	case "OFF_SHELF":
		return api.ShelfStatusOffShelf // 3
	case "PENDING":
		return api.ShelfStatusPending // 0
	default:
		return api.ShelfStatusPending
	}
}
