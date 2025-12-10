# SHEIN 产品上下架功能说明

## 功能概述

实现了 SHEIN 平台产品自动上下架功能:

### 下架场景
1. **库存变化策略** - 当库存变化超过阈值时自动下架
2. **缺货策略** - 当产品缺货时自动下架
3. **低利润率策略** - 当利润率低于设定值时自动下架

### 上架场景
可通过 API 手动触发产品上架

## API 接口

### 端点
```
POST /spmp-api-prefix/spmp/product/operate_Shelf_status
```

### 下架请求示例
```json
{
  "skc_site_infos": [
    {
      "business_model": 1,
      "sub_sites": [],
      "off_sub_sites": [
        {
          "site_abbr": "shein-us",
          "store_type": 1
        }
      ],
      "skc_name": "sp251124754797599194"
    }
  ],
  "spu_name": "p2511247547975"
}
```

### 上架请求示例
```json
{
  "skc_site_infos": [
    {
      "business_model": 1,
      "sub_sites": [
        {
          "site_abbr": "shein-us",
          "store_type": 1
        }
      ],
      "off_sub_sites": [],
      "skc_name": "sp251123411017403946"
    }
  ],
  "spu_name": "p2511234110174"
}
```

### 响应示例
```json
{
  "code": "0",
  "msg": "OK",
  "info": {
    "data": [
      {
        "spu_name": "p2511234110174",
        "sale_name": "10 COLORS",
        "skc_name": "sp251123411017403946",
        "filtered": false,
        "msg": "该商品上架失败，未上站点:[shein-us] 其他运营策略"
      }
    ],
    "meta": {
      "count": 1,
      "customObj": null
    }
  },
  "bbl": null
}
```

## 代码结构

### 1. 数据结构 (`common/shein/api/product/shelf.go`)
- `ShelfOperateRequest` - 上下架请求
- `ShelfOperateResponse` - 上下架响应
- `ShelfOperateResult` - 操作结果
- `SkcSiteInfo` - SKC 站点信息
- `SubSite` - 子站点信息

### 2. API 实现 (`common/shein/impl/product_api.go`)
- `OffShelf()` - 调用下架接口
- `OnShelf()` - 调用上架接口

### 3. 策略执行器 (`platforms/shein/strategy_executor.go`)
- `offShelfProduct()` - 执行下架逻辑
- `onShelfProduct()` - 执行上架逻辑
- `buildOffShelfRequest()` - 构建下架请求
- `buildOnShelfRequest()` - 构建上架请求

## 使用示例

### 自动下架(通过策略)
```go
// 创建策略执行器
executor := NewStrategyExecutor(strategy, apiClient)

// 执行库存变化策略(可能触发下架)
err := executor.ExecuteStockChange(prod, skuMapping, amazonProduct)

// 执行缺货策略(可能触发下架)
err := executor.ExecuteOutOfStock(prod, skuMapping, amazonProduct)

// 执行低利润率策略(可能触发下架)
err := executor.ExecuteLowProfit(prod, skuMapping, amazonProduct)
```

### 手动上架
```go
// 构建上架请求
request := &product.ShelfOperateRequest{
    SkcSiteInfos: []product.SkcSiteInfo{
        {
            BusinessModel: 1,
            SubSites: []product.SubSite{
                {
                    SiteAbbr:  "shein-us",
                    StoreType: 1,
                },
            },
            OffSubSites: []product.SubSite{},
            SkcName:     "sp251123411017403946",
        },
    },
    SpuName: "p2511234110174",
}

// 调用上架接口
err := apiClient.OnShelf(request)
```

## 配置说明

在运营策略中配置下架动作:

```yaml
strategy:
  stock_change_action: "OFF_SHELF"  # 库存变化时下架
  out_of_stock_action: "OFF_SHELF"  # 缺货时下架
  low_profit_action: "OFF_SHELF"    # 低利润时下架
```

## 注意事项

1. 上下架操作需要产品包含有效的 SKC 映射信息
2. 请求会针对所有 SKC 执行
3. 默认操作美国站点 (shein-us)
4. 支持认证过期自动处理
5. 上架时 `sub_sites` 包含目标站点,`off_sub_sites` 为空
6. 下架时 `off_sub_sites` 包含目标站点,`sub_sites` 为空
