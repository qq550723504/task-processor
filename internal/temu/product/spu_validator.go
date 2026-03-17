package product

import (
	"fmt"
	"strconv"

	"task-processor/internal/pipeline"
	models "task-processor/internal/temu/api/product"

	"github.com/sirupsen/logrus"
)

// SpuValidator SPU验证器
type SpuValidator struct {
	logger *logrus.Entry
}

// NewSpuValidator 创建新的SPU验证器
func NewSpuValidator(logger *logrus.Entry) *SpuValidator {
	return &SpuValidator{
		logger: logger,
	}
}

// ValidateProductData 验证产品数据
func (sv *SpuValidator) ValidateProductData(ctx pipeline.TaskContext, temuProduct *models.Product) error {
	sv.logger.Info("验证产品数据")

	// 验证基本信息
	if temuProduct.GoodsBasic.GoodsName == "" {
		return fmt.Errorf("商品名称不能为空")
	}

	if len(temuProduct.SkcList) == 0 {
		return fmt.Errorf("SKC列表不能为空")
	}

	// 验证每个SKC
	for i, skc := range temuProduct.SkcList {

		if len(skc.SkuList) == 0 {
			return fmt.Errorf("SKC[%d]的SKU列表不能为空", i)
		}

		// 验证每个SKU
		for j, sku := range skc.SkuList {
			// 验证规格不能为空
			if len(sku.Spec) == 0 {
				sv.logger.Errorf("❌ SKC[%d]的SKU[%d]的规格列表为空", i, j)
				return fmt.Errorf("SKC[%d]的SKU[%d]的规格列表不能为空，TEMU要求每个SKU必须有规格", i, j)
			}

			// 验证规格的必要字段
			for k, spec := range sku.Spec {
				if spec.SpecID == "" {
					sv.logger.Errorf("❌ SKC[%d]的SKU[%d]的规格[%d]的SpecID为空", i, j, k)
					return fmt.Errorf("SKC[%d]的SKU[%d]的规格[%d]的SpecID不能为空", i, j, k)
				}
				if spec.ParentSpecID == "" {
					sv.logger.Errorf("❌ SKC[%d]的SKU[%d]的规格[%d]的ParentSpecID为空", i, j, k)
					return fmt.Errorf("SKC[%d]的SKU[%d]的规格[%d]的ParentSpecID不能为空", i, j, k)
				}
				if spec.SpecName == "" {
					sv.logger.Errorf("❌ SKC[%d]的SKU[%d]的规格[%d]的SpecName为空", i, j, k)
					return fmt.Errorf("SKC[%d]的SKU[%d]的规格[%d]的SpecName不能为空", i, j, k)
				}
			}

			// if sku.Price <= 0 {
			// 	return fmt.Errorf("SKC[%d]的SKU[%d]的价格必须大于0", i, j)
			// }

			// 验证库存（现在是字符串类型）
			// if sku.Quantity == "" || sku.Quantity == "0" {
			// 	return fmt.Errorf("SKC[%d]的SKU[%d]的库存不能为空或0", i, j)
			// }
		}
	}

	sv.logger.Info("产品数据验证通过")
	return nil
}

// LogProductSummary 记录产品摘要
func (sv *SpuValidator) LogProductSummary(ctx pipeline.TaskContext, temuProduct *models.Product) {
	sv.logger.Info("=== 产品构建摘要 ===")
	sv.logger.Infof("商品名称: %s", temuProduct.GoodsBasic.GoodsName)
	sv.logger.Infof("外部商品编号: %s", temuProduct.GoodsBasic.OutGoodsSN)
	sv.logger.Infof("语言: %s", temuProduct.GoodsBasic.Lang)
	sv.logger.Infof("允许站点: %v", temuProduct.GoodsBasic.AllowSite)
	sv.logger.Infof("SKC数量: %d", len(temuProduct.SkcList))

	// 计算总SKU数量
	totalSkuCount := 0
	for _, skc := range temuProduct.SkcList {
		totalSkuCount += len(skc.SkuList)
	}
	sv.logger.Infof("总SKU数量: %d", totalSkuCount)

	// 记录价格范围
	minPrice, maxPrice := sv.getPriceRange(temuProduct.SkcList)
	if minPrice == maxPrice {
		sv.logger.Infof("价格: %.2f", float64(minPrice)/100)
	} else {
		sv.logger.Infof("价格范围: %.2f - %.2f", float64(minPrice)/100, float64(maxPrice)/100)
	}

	// 记录库存总量
	totalStock := sv.getTotalStock(temuProduct.SkcList)
	sv.logger.Infof("总库存: %d", totalStock)

	sv.logger.Info("==================")
}

// getPriceRange 获取价格范围
func (sv *SpuValidator) getPriceRange(skcList []models.Skc) (int, int) {
	if len(skcList) == 0 {
		return 0, 0
	}

	var minPrice, maxPrice int
	first := true

	for _, skc := range skcList {
		for _, sku := range skc.SkuList {
			if first {
				minPrice = sku.Price
				maxPrice = sku.Price
				first = false
			} else {
				if sku.Price < minPrice {
					minPrice = sku.Price
				}
				if sku.Price > maxPrice {
					maxPrice = sku.Price
				}
			}
		}
	}

	return minPrice, maxPrice
}

// getTotalStock 获取总库存
func (sv *SpuValidator) getTotalStock(skcList []models.Skc) int {
	total := 0
	for _, skc := range skcList {
		for _, sku := range skc.SkuList {
			// 将字符串类型的库存转换为整数
			if quantity, err := strconv.Atoi(sku.Quantity); err == nil {
				total += quantity
			}
		}
	}
	return total
}
