# 运营策略执行优化说明

## 问题描述

在产品监控服务中，存在策略重复执行的问题：

- **库存变化策略** (`ExecuteStockChange`) - 当库存变化超过阈值时触发
- **缺货策略** (`ExecuteOutOfStock`) - 当产品缺货时触发

### 重复场景

当产品从有货变为缺货时（库存从 31 → 0），会同时满足：
1. 库存变化超过阈值 → 触发 `ExecuteStockChange`
2. `IsAvailable = false` → 触发 `ExecuteOutOfStock`

如果两个策略都配置为 `OFF_SHELF`，会导致下架操作执行两次。

## 解决方案

### 策略执行优先级

引入策略执行优先级机制，避免重复操作：

```
优先级：缺货策略 > 库存变化策略 > 低利润率策略
```

### 执行逻辑

1. **缺货策略（最高优先级）**
   - 当产品缺货时，优先执行缺货策略
   - 如果缺货策略已执行，跳过库存变化策略

2. **库存变化策略**
   - 仅在未执行缺货策略时执行
   - 避免与缺货策略重复操作

3. **低利润率策略**
   - 独立执行，不受其他策略影响
   - 可与缺货/库存变化策略并行

## 代码改动

### product_monitor_service.go

```go
// 策略执行优先级：缺货 > 库存变化 > 低利润率
executed := false

// 1. 优先执行缺货策略
if !amazonProduct.IsAvailable {
    if err := executor.ExecuteOutOfStock(...); err != nil {
        // 处理错误
    } else if strategy.OutOfStockAction != "NONE" {
        executed = true
    }
}

// 2. 执行库存变化策略（仅当未执行缺货策略时）
if !executed {
    if err := executor.ExecuteStockChange(...); err != nil {
        // 处理错误
    }
}

// 3. 执行低利润率策略（独立执行）
if err := executor.ExecuteLowProfit(...); err != nil {
    // 处理错误
}
```

### strategy_executor.go

为每个策略方法添加了详细注释，明确返回值含义：
- 返回 `error` 表示执行失败
- 返回 `nil` 表示执行成功或不需要执行

## 效果

- ✅ 避免了缺货和库存变化策略的重复执行
- ✅ 保持了低利润率策略的独立性
- ✅ 提高了策略执行的可控性和可预测性
- ✅ 减少了不必要的 API 调用

## 库存更新实现

### updateStock 方法

实现了完整的库存更新逻辑：

1. **应用库存更新比例**
   - 根据策略配置的 `StockUpdateRatio` 调整库存数量

2. **查询当前库存详情**
   - 调用 `QueryInventory` 获取产品的 SKC/SKU/仓库信息

3. **构建更新请求**
   - 遍历所有 SKC 和 SKU，找到匹配的平台 SKU
   - 只更新可销售的仓库（`IsSaleable = true`）
   - 构建 `InventoryUpdateRequest` 请求

4. **执行更新**
   - 调用 `UpdateInventory` API 更新库存

### 关键逻辑

```go
// 查询库存详情
inventoryResp, err := e.apiClient.QueryInventory(prod.PlatformProductID)

// 构建更新请求（只更新匹配的 SKU 和可销售仓库）
request := e.buildInventoryUpdateRequest(inventoryResp, platformSKU, oldStock, newStock)

// 执行更新
err := e.apiClient.UpdateInventory(request)
```

## 测试建议

1. 测试产品从有货变为缺货的场景
2. 测试库存大幅变化但未缺货的场景
3. 测试低利润率与缺货同时触发的场景
4. 验证策略计数器的准确性
5. 测试库存更新功能（包括库存比例调整）
6. 验证只更新可销售仓库的逻辑
