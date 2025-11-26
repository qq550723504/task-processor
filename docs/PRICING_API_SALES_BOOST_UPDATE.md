# Temu 核价 API 销量提升场景适配

## 概述

本次更新将 Temu 核价 API 从旧版本适配到新的"销量提升"场景接口。

## 主要变更

### 1. 接口地址变更

**旧版本：** `/mms/marigold/sku/v2/search_sales_boost`

**新版本：** `/mms/marigold/price/v2/search_sales_boost`

### 2. 请求参数调整

**旧版本：**
```json
{
  "page_no": 1,
  "page_size": 25,
  "order_type": 0,
  "order_field": "gmt_create",
  "enable_batch_search_text": true,
  "sku_search_type": 2
}
```

**新版本：**
```json
{
  "page_no": 1,
  "page_size": 25,
  "scene": "PRICING_HEALTH_SALES_BOOST"
}
```

### 3. 响应结构调整

**旧版本：** 直接返回 `sku_list`

**新版本：** 返回 `sales_boost_goods_list`，每个商品包含：
- `sales_boost_goods_basic_info`: 商品基本信息
- `sales_boost_sku_list`: SKU 列表
- `hover_info`: 悬浮信息

### 4. 新增数据类型

- `SalesBoostGoods`: 销量提升商品
- `SalesBoostGoodsBasicInfo`: 商品基本信息
- `SalesBoostSku`: 销量提升 SKU
- `ActionInfo`: 操作信息
- `HoverInfo`: 悬浮信息
- `HoverSkuInfo`: 悬浮 SKU 信息

### 5. 核心方法更新

#### `GetPendingPriceList`
- 简化请求参数，只需传入 `page_no`、`page_size` 和 `scene`
- 适配新的响应结构

#### `AutoProcessPendingPricesWithRules`
- 遍历 `SalesBoostGoodsList` 而不是 `SkuList`
- 对每个商品的每个 SKU 进行决策

#### 新增方法
- `MakeDecisionForSalesBoost`: 针对销量提升场景的决策方法
- `executeDecisionForSalesBoost`: 针对销量提升场景的执行方法
- `parsePrice`: 价格字符串解析工具方法

### 6. 兼容性

保留了旧版本的 `executeDecision` 方法，确保向后兼容。

## 使用示例

```go
// 获取待核价列表
resp, err := apiClient.GetPendingPriceList(1, 25)
if err != nil {
    log.Fatal(err)
}

// 遍历商品和 SKU
for _, goods := range resp.Result.SalesBoostGoodsList {
    for _, sku := range goods.SalesBoostSkuList {
        // 处理每个 SKU
        fmt.Printf("商品: %s, SKU: %s\n", 
            goods.SalesBoostGoodsBasicInfo.GoodsName, 
            sku.SkuID)
    }
}
```

## 测试

已更新单元测试以验证新的请求结构：

```bash
go test -v ./common/temu -run TestPendingPriceListRequest
```

## 影响范围

- `common/temu/pricing_types.go`: 数据结构定义
- `common/temu/pricing_api.go`: API 接口实现
- `common/temu/pricing_decision_service.go`: 决策服务
- `common/temu/pricing_api_test.go`: 单元测试

## 注意事项

1. 新接口使用 `scene` 参数替代了多个过滤参数
2. 响应结构层次更深，需要遍历商品列表和 SKU 列表
3. 价格字段从数值类型改为字符串类型，需要解析
4. 保留了旧版本方法以确保兼容性
