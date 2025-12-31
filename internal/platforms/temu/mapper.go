package temu

import (
	"encoding/json"
	"fmt"
	"task-processor/internal/pkg/management/api"
	"time"
)

// MapToProductData 将 TEMU 产品数据映射为通用产品数据
func MapToProductData(temuProduct *TemuProductResponse, tenantID, storeID int64) (*api.ProductDataDTO, error) {
	if temuProduct == nil {
		return nil, fmt.Errorf("TEMU 产品数据为空")
	}

	// 解析创建时间
	var createTime *time.Time
	if temuProduct.CrtTime != "" {
		t, err := ParseTime(temuProduct.CrtTime)
		if err == nil {
			createTime = t
		}
	}

	// 解析上架时间（使用状态更新时间）
	var shelfTime *time.Time
	if temuProduct.StatusUpdateTime != "" {
		t, err := ParseTime(temuProduct.StatusUpdateTime)
		if err == nil {
			shelfTime = t
		}
	}

	// 序列化图片URL列表
	imageURLs, _ := json.Marshal([]string{temuProduct.ThumbURL, temuProduct.SkuPreviewURL})

	// 序列化属性
	attributes := map[string]interface{}{
		"spec_name":              temuProduct.SpecName,
		"variations_count":       temuProduct.VariationsCount,
		"cat_name_list":          temuProduct.CatNameList,
		"personalization_status": temuProduct.PersonalizationStatus,
		"shipping_mode":          temuProduct.ShippingMode,
		"is_books":               temuProduct.IsBooks,
	}
	attributesJSON, _ := json.Marshal(attributes)

	return &api.ProductDataDTO{
		Platform:      "TEMU",
		TenantID:      tenantID,
		StoreID:       storeID,
		ProductID:     temuProduct.GoodsID,
		Title:         temuProduct.GoodsName,
		Description:   temuProduct.SpecName,
		OriginalPrice: api.FlexibleString(fmt.Sprintf("%.2f", temuProduct.SupplierPrice)),
		SpecialPrice:  api.FlexibleString(fmt.Sprintf("%.2f", temuProduct.Price)),
		PriceCurrency: temuProduct.Currency,
		Stock:         api.FlexibleString(fmt.Sprintf("%d", temuProduct.Quantity)),
		CategoryID:    temuProduct.CatID,
		Category:      getCategoryName(temuProduct.CatNameList),
		MainImageURL:  temuProduct.ThumbURL,
		ImageURLs:     string(imageURLs),
		Attributes:    string(attributesJSON),
		ShelfStatus:   MapShelfStatus(temuProduct.Status4Vo),
		CreateTime:    createTime,
		ShelfTime:     shelfTime,
	}, nil
}

// MapShelfStatus 映射 TEMU 上架状态
// TEMU 状态: 3=已上架
func MapShelfStatus(status int) int {
	switch status {
	case 3:
		return api.ShelfStatusOnShelf // 已上架
	default:
		return api.ShelfStatusPending // 待上架
	}
}

// getCategoryName 从分类名称列表中获取最后一级分类
func getCategoryName(catNameList []string) string {
	if len(catNameList) == 0 {
		return ""
	}
	return catNameList[len(catNameList)-1]
}
