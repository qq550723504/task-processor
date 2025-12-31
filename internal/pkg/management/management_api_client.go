package management

import (
	"task-processor/internal/pkg/management/api"
	"task-processor/internal/pkg/management/impl"
)

// RawJsonDataAPIClient 原始JSON数据API客户端接口
// 该接口与api.RawJsonDataAPI保持一致
type RawJsonDataAPIClient = api.RawJsonDataAPI

// ImageDownloaderAPIClient 图片下载API客户端接口
// 该接口与api.ImageDownloader保持一致
type ImageDownloaderAPIClient = api.ImageDownloader

// FilterRuleAPIClient 筛选规则API客户端接口
// 该接口与api.FilterRuleInterface保持一致
type FilterRuleAPIClient = api.FilterRuleInterface

// ProfitRuleAPIClient 利润规则API客户端接口
// 该接口与api.ProfitRuleAPI保持一致
type ProfitRuleAPIClient = api.ProfitRuleAPI

// PricingRuleAPIClient 自动核价规则API客户端接口
// 该接口与api.PricingRuleAPI保持一致
type PricingRuleAPIClient = api.PricingRuleAPI

// SensitiveWordAPIClient 敏感词API客户端接口
// 该接口与api.SensitiveWordInterface保持一致
type SensitiveWordAPIClient = api.SensitiveWordInterface

// ImportTaskAPIClient 导入任务API客户端接口
// 该接口与api.ImportTaskAPI保持一致
type ImportTaskAPIClient = api.ImportTaskAPI

// DailyListingCountAPIClient 每日上架数量API客户端接口
// 该接口与api.DailyListingCountAPI保持一致
type DailyListingCountAPIClient = api.DailyListingCountAPI

// ProductImportMappingAPIClient 产品导入映射API客户端接口
// 该接口与api.ProductImportMappingAPI保持一致
type ProductImportMappingAPIClient = api.ProductImportMappingAPI

// 确保ProductImportMappingAPIClientImpl实现了ProductImportMappingAPIClient接口
var _ ProductImportMappingAPIClient = (*impl.ProductImportMappingAPIClientImpl)(nil)

// 确保RawJsonDataAPIClientImpl实现了RawJsonDataAPIClient接口
var _ RawJsonDataAPIClient = (*impl.RawJsonDataAPIClientImpl)(nil)

// 确保ImageDownloader实现了ImageDownloaderAPIClient接口
var _ ImageDownloaderAPIClient = (*impl.ImageDownloader)(nil)

// 确保FilterRuleAPIClientImpl实现了FilterRuleAPIClient接口
var _ FilterRuleAPIClient = (*impl.FilterRuleAPIClientImpl)(nil)

// 确保ProfitRuleAPIClientImpl实现了ProfitRuleAPIClient接口
var _ ProfitRuleAPIClient = (*impl.ProfitRuleAPIClientImpl)(nil)

// 确保PricingRuleAPIClientImpl实现了PricingRuleAPIClient接口
var _ PricingRuleAPIClient = (*impl.PricingRuleAPIClientImpl)(nil)

// 确保SensitiveWordAPIClientImpl实现了SensitiveWordAPIClient接口
var _ SensitiveWordAPIClient = (*impl.SensitiveWordAPIClientImpl)(nil)

// 确保ImportTaskAPIClientImpl实现了ImportTaskAPIClient接口
var _ ImportTaskAPIClient = (*impl.ImportTaskAPIClientImpl)(nil)

// 确保DailyListingCountAPIClientImpl实现了DailyListingCountAPIClient接口
var _ DailyListingCountAPIClient = (*impl.DailyListingCountAPIClientImpl)(nil)
