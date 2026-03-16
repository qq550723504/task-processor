// Package syncsvc 提供TEMU产品转换相关功能
package syncsvc

import (
	"encoding/json"
	"fmt"
	"time"

	"task-processor/internal/infra/clients/management/api"
	managementapi "task-processor/internal/infra/clients/management/api"
	"task-processor/internal/pkg/types"
	temuproduct "task-processor/internal/platforms/temu/api/product"

	"github.com/sirupsen/logrus"
)

// convertSingleProduct 转换单个TEMU产品
func (s *productSyncServiceImpl) convertSingleProduct(temuProduct *temuproduct.SkuResponse, tenantID, storeID int64) (*managementapi.ProductDataDTO, error) {
	// 检查必要字段
	if temuProduct.GoodsID == "" {
		return nil, fmt.Errorf("商品ID为空，跳过转换")
	}

	// 构建基础产品数据
	productData := s.buildBaseProductData(temuProduct, tenantID, storeID)

	// 填充价格信息
	s.fillPriceInfo(productData, temuProduct)

	// 填充库存信息
	s.fillStockInfo(productData, temuProduct)

	// 填充分类信息
	s.fillCategoryInfo(productData, temuProduct)

	// 填充平台特有数据
	s.fillPlatformData(productData, temuProduct)

	// 验证产品数据
	if err := s.validateProductData(productData); err != nil {
		return nil, fmt.Errorf("产品数据验证失败: %w", err)
	}

	return productData, nil
}

// buildBaseProductData 构建基础产品数据
func (s *productSyncServiceImpl) buildBaseProductData(temuProduct *temuproduct.SkuResponse, tenantID, storeID int64) *managementapi.ProductDataDTO {
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
	imageURLs := s.buildImageURLs(temuProduct)
	imageURLsJSON, _ := json.Marshal(imageURLs)

	// 构建商品描述（使用规格名称作为描述的一部分）
	description := temuProduct.GoodsName
	if temuProduct.SpecName != "" {
		description = fmt.Sprintf("%s - %s", description, temuProduct.SpecName)
	}

	return &managementapi.ProductDataDTO{
		TenantID:          tenantID,
		StoreID:           storeID,
		Platform:          "TEMU",
		PlatformProductID: temuProduct.GoodsID,
		ProductID:         temuProduct.OutGoodsSN, // 使用外部商品编号作为产品ID
		Title:             temuProduct.GoodsName,
		Description:       description,
		MainImageURL:      temuProduct.ThumbURL,
		ImageURLs:         string(imageURLsJSON),
		ShelfStatus:       2,
		PublishTime:       types.ToFlexibleTime(publishTime),
		ShelfTime:         types.ToFlexibleTime(shelfTime),
		PriceCurrency:     temuProduct.Currency,
	}
}

// convertSingleGoodsProduct 转换单个TEMU商品（来自商品搜索）
func (s *productSyncServiceImpl) convertSingleGoodsProduct(temuProduct *temuproduct.GoodsSearchItem, tenantID, storeID int64) (*managementapi.ProductDataDTO, error) {
	// 获取店铺信息
	storeInfo, err := s.storeAPI.GetStore(storeID)
	if err != nil {
		s.logger.WithError(err).WithField("store_id", storeID).Warn("获取店铺信息失败，使用默认处理")
	}
	// 构建基础产品数据
	productData := s.buildBaseGoodsProductData(temuProduct, storeInfo)

	// 通过SKU查询接口获取详细的价格和库存信息
	skuDetails, err := s.fetchSkuDetails(temuProduct)
	if err != nil {
		s.logger.WithError(err).WithField("goods_id", temuProduct.GoodsID).Warn("获取SKU详细信息失败")
	} else {
		// 使用SKU查询结果填充详细的价格和库存信息
		s.fillSkuBasedPriceAndStockInfo(productData, skuDetails)
	}

	// 填充分类信息
	s.fillGoodsCategoryInfo(productData, temuProduct)

	// 填充平台特有数据
	s.fillGoodsPlatformData(productData, temuProduct)

	// 生成映射数据并填充到Attributes字段
	if err := s.fillMappingAttributesWithSkuDetails(productData, temuProduct, skuDetails, storeID); err != nil {
		s.logger.WithError(err).Warn("生成映射数据失败，继续处理")
	}

	// 验证产品数据
	if err := s.validateProductData(productData); err != nil {
		return nil, fmt.Errorf("产品数据验证失败: %w", err)
	}

	s.logger.WithFields(logrus.Fields{
		"goods_id":   temuProduct.GoodsID,
		"goods_name": temuProduct.GoodsName,
		"status":     temuProduct.Status4VO,
		"price":      productData.OriginalPrice,
		"currency":   temuProduct.Currency,
		"variations": temuProduct.VariationsCount,
		"sku_count": func() int {
			if skuDetails != nil {
				return len(skuDetails.Result.SkuList)
			}
			return 0
		}(),
	}).Debug("TEMU商品转换完成")

	return productData, nil
}

// buildBaseGoodsProductData 构建基础商品数据（来自商品搜索）
func (s *productSyncServiceImpl) buildBaseGoodsProductData(temuProduct *temuproduct.GoodsSearchItem, storeInfo *api.StoreRespDTO) *managementapi.ProductDataDTO {
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
	imageURLs := s.buildGoodsImageURLs(temuProduct)
	imageURLsJSON, _ := json.Marshal(imageURLs)

	// 构建商品描述
	description := temuProduct.GoodsName
	if temuProduct.SpecName != "" {
		description = fmt.Sprintf("%s - %s", description, temuProduct.SpecName)
	}

	return &managementapi.ProductDataDTO{
		TenantID:          storeInfo.TenantID,
		StoreID:           storeInfo.ID,
		Region:            storeInfo.Region,
		Platform:          "TEMU",
		PlatformProductID: temuProduct.GoodsID,
		ProductID:         temuProduct.OutGoodsSN, // 使用外部商品编号作为产品ID
		Title:             temuProduct.GoodsName,
		Description:       description,
		MainImageURL:      temuProduct.ThumbURL,
		ImageURLs:         string(imageURLsJSON),
		ShelfStatus:       2,
		PublishTime:       types.ToFlexibleTime(publishTime),
		ShelfTime:         types.ToFlexibleTime(shelfTime),
		PriceCurrency:     temuProduct.Currency,
	}
}

// fillGoodsPriceInfo 填充商品价格信息
func (s *productSyncServiceImpl) fillGoodsPriceInfo(productData *managementapi.ProductDataDTO, temuProduct *temuproduct.GoodsSearchItem) {
	productData.OriginalPrice = types.FlexibleString(fmt.Sprintf("%.2f", temuProduct.Price))
	productData.SpecialPrice = types.FlexibleString(fmt.Sprintf("%.2f", temuProduct.Price)) // 商品搜索中没有特价信息，使用原价
}

// fillGoodsStockInfo 填充商品库存信息
func (s *productSyncServiceImpl) fillGoodsStockInfo(productData *managementapi.ProductDataDTO, temuProduct *temuproduct.GoodsSearchItem) {
	productData.Stock = types.FlexibleString(fmt.Sprintf("%d", temuProduct.Quantity))
}

// fillGoodsCategoryInfo 填充商品分类信息
func (s *productSyncServiceImpl) fillGoodsCategoryInfo(productData *managementapi.ProductDataDTO, temuProduct *temuproduct.GoodsSearchItem) {
	if len(temuProduct.CatNameList) > 0 {
		productData.Category = temuProduct.CatNameList[len(temuProduct.CatNameList)-1] // 使用最后一级分类
	}
	productData.CategoryID = int64(temuProduct.CatID)
}

// fillGoodsPlatformData 填充商品平台特有数据
func (s *productSyncServiceImpl) fillGoodsPlatformData(productData *managementapi.ProductDataDTO, temuProduct *temuproduct.GoodsSearchItem) {
	platformData := map[string]interface{}{
		"listing_commit_id":       temuProduct.ListingCommitID,
		"goods_commit_id":         temuProduct.GoodsCommitID,
		"mall_id":                 temuProduct.MallID,
		"status4_vo":              temuProduct.Status4VO,
		"sub_status4_vo":          temuProduct.SubStatus4VO,
		"variations_count":        temuProduct.VariationsCount,
		"cat_type":                temuProduct.CatType,
		"multi_site_goods":        temuProduct.MultiSiteGoods,
		"personalization_status":  temuProduct.PersonalizationStatus,
		"punish_tags":             temuProduct.PunishTags,
		"high_price_spread_ratio": temuProduct.HighPriceSpreadRatio,
		"low_traffic_tag":         temuProduct.LowTrafficTag,
		"restricted_traffic_tag":  temuProduct.RestrictedTrafficTag,
		"is_books":                temuProduct.IsBooks,
		"checked_time":            temuProduct.CheckedTime,
	}

	if len(temuProduct.OutSkuSNList) > 0 {
		platformData["out_sku_sn_list"] = temuProduct.OutSkuSNList
	}

	if len(temuProduct.SkuIDList) > 0 {
		platformData["sku_id_list"] = temuProduct.SkuIDList
	}

	platformDataJSON, _ := json.Marshal(platformData)
	productData.PlatformData = string(platformDataJSON)

	// 设置平台状态
	productData.PlatformStatus = s.mapPlatformStatus(temuProduct.Status4VO, temuProduct.SubStatus4VO)
}

// buildGoodsImageURLs 构建商品图片URL列表
func (s *productSyncServiceImpl) buildGoodsImageURLs(temuProduct *temuproduct.GoodsSearchItem) []string {
	var imageURLs []string

	// 添加主图
	if temuProduct.ThumbURL != "" {
		imageURLs = append(imageURLs, temuProduct.ThumbURL)
	}

	// 添加预览图（如果与主图不同）
	if temuProduct.SkuPreviewURL != "" && temuProduct.SkuPreviewURL != temuProduct.ThumbURL {
		imageURLs = append(imageURLs, temuProduct.SkuPreviewURL)
	}

	return imageURLs
}
