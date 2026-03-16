// Package marketing 提供SHEIN限时折扣活动完整流程示例
package marketing

// 完整的限时折扣活动创建流程示例：
//
// ========================================
// 步骤 1: 查询可参加活动的商品
// ========================================
//
// queryReq := &marketing.QueryPromotionGoodsRequest{
//     ActivityBaseInfoRequest: marketing.ActivityBaseInfoRequest{
//         ActName:       "#GS4051729#限时折扣#2026-01-13#1",
//         RefToolID:     30,
//         TimeZone:      "America/Los_Angeles",
//         ZoneEndTime:   "2026-02-12 22:57:42",
//         ZoneStartTime: "2026-01-13 22:57:42",
//         SubTypeID:     2,
//     },
//     EffectiveCenterList: []int{2},
//     IsShelf:             1,
//     PageNum:             1,
//     PageSize:            30,
// }
//
// marketingAPI := repo.NewMarketingAPI(baseClient)
// queryResp, err := marketingAPI.QueryPromotionGoods(queryReq)
// if err != nil {
//     log.Printf("查询商品失败: %v", err)
//     return
// }
//
// log.Printf("查询到 %d 个可参加活动的商品", queryResp.Info.Meta.Count)
//
// ========================================
// 步骤 2: 计算商品价格和利润
// ========================================
//
// // 从查询结果中选择商品，构建价格计算请求
// skcInfoList := make([]marketing.SkcPriceInfo, 0)
// for _, goods := range queryResp.Info.Data {
//     skuInfoList := make([]marketing.SkuPriceInfo, 0)
//     for _, sku := range goods.SkuInfoList {
//         skuInfoList = append(skuInfoList, marketing.SkuPriceInfo{
//             SkuCode:       sku.Sku,
//             ProductPrice:  23.38,  // 商品原价
//             DiscountValue: 14.99,  // 折扣价
//         })
//     }
//     skcInfoList = append(skcInfoList, marketing.SkcPriceInfo{
//         SkcName:     goods.Skc,
//         SkuInfoList: skuInfoList,
//     })
// }
//
// calcReq := &marketing.CalculateSupplyPriceRequest{
//     Currency:      "USD",
//     RefToolID:     30,
//     SceneID:       1,
//     SkcInfoList:   skcInfoList,
//     TimeZone:      "America/Los_Angeles",
//     ZoneEndTime:   "2026-02-12 23:08:32",
//     ZoneStartTime: "2026-01-13 23:08:32",
// }
//
// calcResp, err := marketingAPI.CalculateSupplyPrice(calcReq)
// if err != nil {
//     log.Printf("计算价格失败: %v", err)
//     return
// }
//
// // 检查价格计算结果
// for _, skcResult := range calcResp.Info {
//     for _, skuInfo := range skcResult.SkuInfoList {
//         if skuInfo.RiskTag != 0 {
//             log.Printf("警告: SKU %s 存在风险，风险标签: %d",
//                 skuInfo.SkuCode, skuInfo.RiskTag)
//         }
//         log.Printf("SKU %s 价格信息: 原价=%.2f, 结算=%.2f, 促销=%.2f",
//             skuInfo.SkuCode,
//             skuInfo.PriceInfo.ProductAmount,
//             skuInfo.PriceInfo.SettlementAmount,
//             skuInfo.PriceInfo.PromotionAmount)
//     }
// }
//
// ========================================
// 步骤 3: 创建限时折扣活动
// ========================================
//
// // 构建活动创建请求
// costAndStockList := make([]marketing.CostAndStockInfo, 0)
// for _, goods := range queryResp.Info.Data {
//     addSkuList := make([]marketing.SkuCostInfo, 0)
//     for _, sku := range goods.SkuInfoList {
//         addSkuList = append(addSkuList, marketing.SkuCostInfo{
//             Sku:                sku.Sku,
//             CostPrice:          0,
//             MaxProductActPrice: 0,
//             ProductActPrice:    0,
//         })
//     }
//
//     costAndStockList = append(costAndStockList, marketing.CostAndStockInfo{
//         Skc:                goods.Skc,
//         AttendNum:          30,
//         StockNum:           30,
//         CenterList:         []int{2},
//         IsSaleAttribute:    0,
//         PromotionIDList:    nil,
//         CostPrice:          goods.USSupplyPrice,
//         MaxProductActPrice: goods.MaxUSSupplyPrice,
//         ProductActPrice:    14.99,
//         AddSkuList:         addSkuList,
//     })
// }
//
// createReq := &marketing.CreateActivityRequest{
//     ActivityBaseInfoRequest: marketing.ActivityBaseInfo{
//         ActName:       "#yangyou922#限时折扣#2026-01-14#1",
//         TimeZone:      "America/Los_Angeles",
//         ZoneStartTime: "2026-01-14 23:08:32",
//         ZoneEndTime:   "2026-02-13 23:08:32",
//         RefToolID:     30,
//         NotifyFlag:    1,
//         SubTypeID:     2,
//         ActivityRule: marketing.ActivityRule{
//             GoodsLimit:    1,
//             GoodsLimitNum: 1,
//         },
//     },
//     AddCostAndStockInfoList: costAndStockList,
//     PricingType:             2,
// }
//
// createResp, err := marketingAPI.CreateActivity(createReq)
// if err != nil {
//     log.Printf("创建活动失败: %v", err)
//     return
// }
//
// if createResp.Code == "0" {
//     log.Printf("活动创建成功！活动ID: %d", createResp.Info.ActivityID)
//
//     // 检查是否有错误信息
//     if createResp.Info.ErrorInfo != nil {
//         log.Printf("活动创建有错误: %v", createResp.Info.ErrorInfo)
//     }
//     if createResp.Info.SkcErrorInfo != nil {
//         log.Printf("SKC错误信息: %v", createResp.Info.SkcErrorInfo)
//     }
//     if createResp.Info.SkuErrorInfo != nil {
//         log.Printf("SKU错误信息: %v", createResp.Info.SkuErrorInfo)
//     }
// }
//
// ========================================
// 完整流程总结
// ========================================
//
// 1. QueryPromotionGoods - 查询可参加活动的商品列表
// 2. CalculateSupplyPrice - 计算选定商品的价格和利润
// 3. CreateActivity - 创建限时折扣活动
//
// 注意事项：
// - 活动名称格式: #用户名#限时折扣#日期#序号
// - 时区必须一致（如: America/Los_Angeles）
// - 价格计算后需检查风险标签
// - 库存数量不能超过可用库存
// - 活动时间必须在未来
