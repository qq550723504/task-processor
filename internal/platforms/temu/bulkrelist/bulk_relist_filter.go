package bulkrelist

import (
	"fmt"
	"slices"
	"task-processor/internal/pkg/strx"
	"task-processor/internal/platforms/temu/api/inventory"

	"github.com/sirupsen/logrus"
)

// ProductFilter 产品过滤器
type ProductFilter struct {
	logger *logrus.Entry
}

// NewProductFilter 创建产品过滤器
func NewProductFilter(logger *logrus.Entry) *ProductFilter {
	return &ProductFilter{
		logger: logger,
	}
}

// ShouldSkipProduct 判断是否应该跳过某个产品
func (f *ProductFilter) ShouldSkipProduct(product *inventory.Item, conditions *SkipConditions) bool {
	if conditions == nil {
		return false
	}

	// 基于日志分析的核心跳过条件 - 这些是导致上架失败的关键标识

	// 1. 检查惩罚标签 - PunishTags = 1 的商品通常上架失败，PunishTags = 0 的商品通常成功
	if product.PunishTags == 1 {
		f.logger.Debugf("跳过惩罚商品: %s (PunishTags=1)", product.GoodsName)
		return true
	}

	// 2. 检查商品状态异常 - ShowSubStatus4VO = 3001 的商品通常失败，ShowSubStatus4VO = 3002 的商品通常成功
	if product.ShowSubStatus4VO == 3001 {
		f.logger.Debugf("跳过状态异常商品: %s (ShowSubStatus4VO=3001)", product.GoodsName)
		return true
	}

	// 原有的跳过条件保持不变

	// 检查是否被严重惩罚 (PunishTags > 1)
	if conditions.SkipSeverelyPunished && product.PunishTags > 1 {
		return true
	}

	// 检查锁定状态 - 修正锁定状态判断逻辑
	if conditions.SkipLocked {
		// 检查是否允许上架操作
		if product.LockInfo.CloseListingMMS.AllowOperate {
			f.logger.Debugf("商品 %s 被锁定: AllowOperate=false", product.GoodsName)
			return true
		}
	}

	// 检查库存
	if conditions.SkipNoStock && product.Stock <= 0 {
		return true
	}

	// 检查最小库存
	if conditions.MinStock > 0 && product.Stock < conditions.MinStock {
		return true
	}

	return false
}

// GetSkipReason 获取跳过原因
func (f *ProductFilter) GetSkipReason(product *inventory.Item, conditions *SkipConditions) string {
	if conditions == nil {
		return "未知原因"
	}

	// 基于日志分析的核心跳过原因 - 优先检查这些导致上架失败的关键标识

	// 1. 惩罚标签检查 - PunishTags = 1 的商品通常失败
	if product.PunishTags == 1 {
		return "商品存在惩罚标签 (PunishTags=1)"
	}

	// 2. 状态异常检查 - ShowSubStatus4VO = 3001 的商品通常失败
	if product.ShowSubStatus4VO == 3001 {
		return "商品状态异常 (ShowSubStatus4VO=3001)"
	}

	// 原有的跳过原因检查

	if conditions.SkipNeedRectification && product.CategoryRectificationInfo.NeedRectification {
		return "商品需要分类整改"
	}

	if conditions.SkipSeverelyPunished && product.PunishTags > 1 {
		return "商品被严重惩罚"
	}

	if conditions.SkipLocked && !product.LockInfo.CloseListingMMS.AllowOperate {
		return "商品被锁定，不允许上架操作"
	}

	if conditions.SkipNoStock && product.Stock <= 0 {
		return "商品无库存"
	}

	if conditions.MinStock > 0 && product.Stock < conditions.MinStock {
		return fmt.Sprintf("商品库存(%d)低于最小要求(%d)", product.Stock, conditions.MinStock)
	}

	return "未知原因"
}

// MatchesFilter 检查产品是否匹配过滤条件
func (f *ProductFilter) MatchesFilter(product *inventory.Item, filter *ProductFilterOptions) bool {
	if filter == nil {
		return true
	}

	// 检查分类
	if len(filter.IncludeCategories) > 0 {
		found := false
		for _, category := range filter.IncludeCategories {
			if slices.Contains(product.CatNameList, category) {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}

	// 检查排除分类
	if len(filter.ExcludeCategories) > 0 {
		for _, category := range filter.ExcludeCategories {
			if slices.Contains(product.CatNameList, category) {
				return false
			}
		}
	}

	// 检查商品名称关键词
	if len(filter.NameKeywords) > 0 {
		found := false
		for _, keyword := range filter.NameKeywords {
			if len(keyword) > 0 && contains(product.GoodsName, keyword) {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}

	// 检查库存范围
	if filter.MinStock > 0 && product.Stock < filter.MinStock {
		return false
	}

	if filter.MaxStock > 0 && product.Stock > filter.MaxStock {
		return false
	}

	// 检查价格范围
	if filter.MinPrice > 0 && product.Price < filter.MinPrice {
		return false
	}

	if filter.MaxPrice > 0 && product.Price > filter.MaxPrice {
		return false
	}

	return true
}

// contains 检查字符串是否包含子字符串（简单实现）
func contains(s, substr string) bool {
	return strx.Contains(s, substr)
}
