// Package shein 提供SHEIN平台产品获取功能
package shein

import (
	"fmt"
	"task-processor/internal/platforms/shein/api/product"

	"github.com/sirupsen/logrus"
)

// ProductFetcher SHEIN产品获取器
type ProductFetcher struct {
	logger *logrus.Entry
}

// NewProductFetcher 创建新的产品获取器
func NewProductFetcher() *ProductFetcher {
	return &ProductFetcher{
		logger: logrus.WithField("component", "ProductFetcher"),
	}
}

// FetchProductList 获取 SHEIN 产品列表
func (f *ProductFetcher) FetchProductList(apiClient *ShopAPIClient) ([]SheinProductResponse, error) {
	// 构建请求参数
	request := &product.ProductListRequest{
		Language:             "en",
		OnlyRecommendResell:  false,
		OnlySpmbCopyProduct:  false,
		SearchAbandonProduct: false,
		SearchIllegal:        false,
		SearchLessInventory:  false,
		//ShelfType:            "ON_SHELF", // 只获取已上架产品
		SortType: 1,
	}

	// 调用产品列表 API
	response, err := apiClient.ListProducts(1, 100, request)
	if err != nil {
		return nil, fmt.Errorf("调用产品列表 API 失败: %w", err)
	}

	// 转换为 SheinProductResponse 格式
	var products []SheinProductResponse
	for _, item := range response.Info.Data {
		// 转换 SKC 信息
		var skcInfoList []SkcInfo
		for _, skc := range item.SkcInfoList {
			var skuInfoList []SkuInfo
			for _, sku := range skc.SkuInfo {
				skuInfoList = append(skuInfoList, SkuInfo{
					SkuCode: sku.SkuCode,
				})
			}

			skcInfoList = append(skcInfoList, SkcInfo{
				SkcName:               skc.SkcName,
				SkcCode:               skc.SkcCode,
				SaleName:              skc.SaleName,
				MainImageThumbnailURL: skc.MainImageThumbnailURL,
				SupplierCode:          skc.SupplierCode,
				BusinessModel:         skc.BusinessModel,
				IsSaleAttribute:       skc.IsSaleAttribute,
				SupplierID:            skc.SupplierID,
				SkuInfo:               skuInfoList,
				MallSellStatus:        skc.MallSellStatus,
				Abandoned:             skc.Abandoned,
				TagInfoList:           skc.TagInfoList,
				ShelfFailReason:       skc.ShelfFailReason,
				HasOriginalImage:      skc.HasOriginalImage,
			})
		}

		product := SheinProductResponse{
			SpuName:          item.SpuName,
			SpuCode:          item.SpuCode,
			CategoryID:       item.CategoryID,
			BrandCode:        item.BrandCode,
			BrandName:        item.BrandName,
			ProductNameCh:    item.ProductNameCh,
			ProductNameEn:    item.ProductNameEn,
			ProductNameMulti: item.ProductNameMulti,
			SkcInfoList:      skcInfoList,
			ShelfStatus:      item.ShelfStatus,
			CreateTime:       item.CreateTime,
			PublishTime:      item.PublishTime,
			FirstShelfTime:   item.FirstShelfTime,
			ExpectShelfTime:  item.ExpectShelfTime,
			TagInfoList:      item.TagInfoList,
		}

		products = append(products, product)
	}

	f.logger.WithField("count", len(products)).Info("成功获取 SHEIN 产品列表")
	return products, nil
}
