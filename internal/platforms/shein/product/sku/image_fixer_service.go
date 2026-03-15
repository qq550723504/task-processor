// Package modules 提供SHEIN平台SKU图片自动修复功能
package sku

import (
	product "task-processor/internal/platforms/shein/api/product"
	"task-processor/internal/platforms/shein/model"

	"github.com/sirupsen/logrus"
)

// SKUImageAutoFixer SKU图片自动修复器
type SKUImageAutoFixer struct {
	utils *SKUUtils
}

// NewSKUImageAutoFixer 创建新的SKU图片自动修复器
func NewSKUImageAutoFixer() *SKUImageAutoFixer {
	return &SKUImageAutoFixer{
		utils: NewSKUUtils(),
	}
}

// AutoFixMultiPieceSKUImage 自动修复多件商品SKU图片
// 在SKU创建后调用，如果是多件商品但没有图片，从对应的SKC复制图片
func (f *SKUImageAutoFixer) AutoFixMultiPieceSKUImage(ctx *model.TaskContext, sku *product.SKU, skcImageInfo *product.ImageInfo, params model.SKUCreationParams) {
	// 检查是否为多件商品
	if !f.IsMultiPieceSKU(sku) {
		return
	}

	// 检查SKU是否已有图片
	if sku.ImageInfo != nil && len(sku.ImageInfo.ImageInfoList) > 0 {
		logrus.Debugf("多件商品SKU %s 已有图片，无需修复", sku.SupplierSKU)
		return
	}

	// 检查SKC是否有图片可以复制
	if skcImageInfo == nil || len(skcImageInfo.ImageInfoList) == 0 {
		logrus.Warnf("多件商品SKU %s 缺少图片，但SKC也没有图片可复制", sku.SupplierSKU)
		return
	}

	// 从SKC复制第一张图片到SKU
	firstImage := skcImageInfo.ImageInfoList[0]
	sku.ImageInfo = &product.ImageInfo{
		ImageGroupCode: nil,
		ImageInfoList: []product.ImageDetail{
			{
				ImageType:             firstImage.ImageType,
				ImageSort:             1, // SKU图片排序固定为1
				ImageURL:              firstImage.ImageURL,
				ImageItemID:           firstImage.ImageItemID,
				SizeImgFlag:           firstImage.SizeImgFlag,
				TransformCVSizeImage:  firstImage.TransformCVSizeImage,
				AISStatus:             firstImage.AISStatus,
				PSTypes:               firstImage.PSTypes,
				MarketingMainImage:    false, // SKU图片不是营销主图
				CommodityCategoryFlag: firstImage.CommodityCategoryFlag,
			},
		},
		OriginalImageInfoList: &[]interface{}{},
	}

	logrus.Infof("🔧 自动修复多件商品SKU图片: SKU %s 从SKC复制了图片", sku.SupplierSKU)
}

// isMultiPieceSKU 判断SKU是否为多件商品
func (f *SKUImageAutoFixer) IsMultiPieceSKU(sku *product.SKU) bool {
	return sku.QuantityInfo != nil &&
		sku.QuantityInfo.QuantityType != nil &&
		*sku.QuantityInfo.QuantityType == 2
}

// AutoFixSKUImageSorting 自动修复SKU图片排序
func (f *SKUImageAutoFixer) AutoFixSKUImageSorting(sku *product.SKU) {
	if sku.ImageInfo == nil || len(sku.ImageInfo.ImageInfoList) == 0 {
		return
	}

	// 多件商品SKU只能有一张图片
	if len(sku.ImageInfo.ImageInfoList) > 1 {
		logrus.Infof("🔧 修复多件商品SKU图片数量: SKU %s 从%d张减少到1张",
			sku.SupplierSKU, len(sku.ImageInfo.ImageInfoList))
		sku.ImageInfo.ImageInfoList = sku.ImageInfo.ImageInfoList[:1]
	}

	// 确保图片排序为1
	if len(sku.ImageInfo.ImageInfoList) > 0 {
		if sku.ImageInfo.ImageInfoList[0].ImageSort != 1 {
			logrus.Infof("🔧 修复多件商品SKU主图排序: SKU %s 从%d修复为1",
				sku.SupplierSKU, sku.ImageInfo.ImageInfoList[0].ImageSort)
			sku.ImageInfo.ImageInfoList[0].ImageSort = 1
		}
	}
}
