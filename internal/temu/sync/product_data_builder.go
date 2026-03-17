// package sync 提供TEMU产品数据构建相关功能
package sync

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	managementapi "task-processor/internal/infra/clients/management/api"
	"task-processor/internal/pkg/types"
	temuproduct "task-processor/internal/temu/api/product"
)

// buildImageURLs 构建图片URL列表
func (s *productSyncServiceImpl) buildImageURLs(temuProduct *temuproduct.SkuResponse) []string {
	var imageURLs []string

	// 添加主图
	if temuProduct.ThumbURL != "" {
		imageURLs = append(imageURLs, temuProduct.ThumbURL)
	}

	return imageURLs
}

// fillPriceInfo 填充价格信息
func (s *productSyncServiceImpl) fillPriceInfo(productData *managementapi.ProductDataDTO, temuProduct *temuproduct.SkuResponse) {
	// 设置原价（使用 price 字段）
	if temuProduct.Price > 0 {
		productData.OriginalPrice = types.FlexibleString(fmt.Sprintf("%.2f", temuProduct.Price))
	} else if temuProduct.PriceVO.Amount != "" {
		productData.OriginalPrice = types.FlexibleString(temuProduct.PriceVO.Amount)
	}

	// 设置特价（列表价格）
	if temuProduct.ListPriceVO.Amount != "" {
		productData.SpecialPrice = types.FlexibleString(temuProduct.ListPriceVO.Amount)
	} else if temuProduct.ListPrice.Amount != "" {
		productData.SpecialPrice = types.FlexibleString(temuProduct.ListPrice.Amount)
	}

	// 设置货币
	if temuProduct.Currency != "" {
		productData.PriceCurrency = temuProduct.Currency
	} else if temuProduct.PriceVO.Currency != "" {
		productData.PriceCurrency = temuProduct.PriceVO.Currency
	} else if temuProduct.ListPriceVO.Currency != "" {
		productData.PriceCurrency = temuProduct.ListPriceVO.Currency
	}
}

// fillStockInfo 填充库存信息
func (s *productSyncServiceImpl) fillStockInfo(productData *managementapi.ProductDataDTO, temuProduct *temuproduct.SkuResponse) {
	// 使用普通库存
	if temuProduct.OrdinaryStock > 0 {
		productData.Stock = types.FlexibleString(strconv.Itoa(temuProduct.OrdinaryStock))
	} else if temuProduct.Stock > 0 {
		// 备用：使用 stock 字段
		productData.Stock = types.FlexibleString(strconv.Itoa(temuProduct.Stock))
	} else {
		// 默认库存为0
		productData.Stock = types.FlexibleString("0")
	}
}

// fillCategoryInfo 填充分类信息
func (s *productSyncServiceImpl) fillCategoryInfo(productData *managementapi.ProductDataDTO, temuProduct *temuproduct.SkuResponse) {
	// 设置分类ID
	if temuProduct.CatID > 0 {
		productData.CategoryID = int64(temuProduct.CatID)
	}

	// 设置分类名称（使用分类路径）
	if len(temuProduct.CatNameList) > 0 {
		productData.Category = strings.Join(temuProduct.CatNameList, " > ")
	}
}

// fillPlatformData 填充平台特有数据
func (s *productSyncServiceImpl) fillPlatformData(productData *managementapi.ProductDataDTO, temuProduct *temuproduct.SkuResponse) {
	// 构建平台状态信息
	platformStatus := map[string]any{
		"status4_vo":             temuProduct.Status4VO,
		"sub_status4_vo":         temuProduct.SubStatus4VO,
		"show_sub_status4_vo":    temuProduct.ShowSubStatus4VO,
		"personalization_status": temuProduct.PersonalizationStatus,
		"punish_tags":            temuProduct.PunishTags,
		"stock_display_tag":      temuProduct.StockDisplayTag,
		"low_traffic_tag":        temuProduct.LowTrafficTag,
		"restricted_traffic_tag": temuProduct.RestrictedTrafficTag,
		"easy_gains_tag":         temuProduct.EasyGainsTag,
		"shipping_mode":          temuProduct.ShippingMode,
		"is_books":               temuProduct.IsBooks,
	}
	platformStatusJSON, _ := json.Marshal(platformStatus)
	productData.PlatformStatus = string(platformStatusJSON)

	// 构建平台数据（包含更多详细信息）
	platformData := map[string]any{
		"listing_commit_id": temuProduct.ListingCommitID,
		"goods_commit_id":   temuProduct.GoodsCommitID,
		"mall_id":           temuProduct.MallID,
		"out_goods_sn":      temuProduct.OutGoodsSN,
		"sku_id":            temuProduct.SkuID,
		"sku_sn":            temuProduct.SkuSN,
		"supplier_price":    temuProduct.SupplierPrice,
		"cat_type":          temuProduct.CatType,
		"multi_site_goods":  temuProduct.MultiSiteGoods,
		"closed_type_list":  temuProduct.ClosedTypeList,
		"spec_name":         temuProduct.SpecName,
		"spec_list":         temuProduct.SpecList,
	}
	platformDataJSON, _ := json.Marshal(platformData)
	productData.PlatformData = string(platformDataJSON)

	// 构建属性信息
	attributes := map[string]any{
		"supplier_price":         temuProduct.SupplierPrice,
		"cat_type":               temuProduct.CatType,
		"multi_site_goods":       temuProduct.MultiSiteGoods,
		"personalization_status": temuProduct.PersonalizationStatus,
		"shipping_mode":          temuProduct.ShippingMode,
		"is_books":               temuProduct.IsBooks,
		"goods_is_onsale":        temuProduct.GoodsIsOnsale,
	}

	// 添加价格相关属性
	if temuProduct.PriceVO.Amount != "" {
		attributes["price_vo"] = temuProduct.PriceVO
	}
	if temuProduct.ListPriceVO.Amount != "" {
		attributes["list_price_vo"] = temuProduct.ListPriceVO
	}
	if temuProduct.ListPrice.Amount != "" {
		attributes["list_price"] = temuProduct.ListPrice
	}

	attributesJSON, _ := json.Marshal(attributes)
	productData.Attributes = string(attributesJSON)
}
