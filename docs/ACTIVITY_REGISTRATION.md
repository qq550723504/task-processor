# SHEIN 活动报名功能说明

## 功能概述

SHEIN 活动报名功能支持根据运营策略自动报名产品到促销活动或限时折扣活动。

## 运营策略配置

### 活动相关字段

在 `OperationStrategyDTO` 中新增以下字段：

| 字段名 | 类型 | 说明 | 示例 |
|--------|------|------|------|
| `ActivityEnabled` | bool | 是否启用活动功能 | true |
| `ActivityType` | string | 活动类型 | PROMOTION / TIME_LIMITED / MIXED |
| `ActivityDiscountRate` | float64 | 活动折扣率（0-1之间） | 0.1 表示打9折 |
| `ActivityStockRatio` | float64 | 活动库存比例（0-1之间） | 0.5 表示50%库存用于活动 |
| `PromotionRatio` | float64 | 促销活动比例（仅MIXED模式） | 0.5 表示50%产品促销，50%限时折扣 |

### 活动类型说明

1. **PROMOTION（促销活动）**
   - 报名产品到常规促销活动
   - 根据配置的折扣率和库存比例自动设置活动参数
   - 适用于长期促销

2. **TIME_LIMITED（限时折扣）**
   - 创建限时折扣活动
   - 支持设置活动时间范围（默认7天）
   - 自动生成活动名称（格式：#用户名#限时折扣#日期#序号）
   - 适用于短期促销

3. **MIXED（混合模式）**
   - 按比例分配产品到促销活动和限时折扣
   - 通过 `PromotionRatio` 控制比例（如 0.5 表示 50% 促销，50% 限时折扣）
   - 适用于需要同时运行两种活动的场景

## 使用示例

### 配置示例

```json
{
  "activityEnabled": true,
  "activityType": "PROMOTION",
  "activityDiscountRate": 0.15,
  "activityStockRatio": 0.8
}
```

上述配置表示：
- 启用活动功能
- 使用促销活动类型
- 活动价格为原价的85%（打8.5折）
- 使用80%的库存参与活动

### 折扣率计算

- `activityDiscountRate = 0.1` → 降价10%，活动价 = 原价 × 0.9
- `activityDiscountRate = 0.15` → 降价15%，活动价 = 原价 × 0.85
- `activityDiscountRate = 0.2` → 降价20%，活动价 = 原价 × 0.8

### 库存比例计算

- `activityStockRatio = 1.0` → 使用全部库存（100%）
- `activityStockRatio = 0.5` → 使用一半库存（50%）
- `activityStockRatio = 0.3` → 使用30%库存

## 执行流程

### 促销活动报名流程

1. **获取可报名产品**
   - 调用 `GetAvailableSkcList` API 获取可报名活动的产品列表
   - 分页获取所有符合条件的产品

2. **获取运营策略**
   - 根据店铺ID获取运营策略配置
   - 检查是否启用活动功能

3. **根据策略报名**
   - 如果 `ActivityEnabled = false`，跳过报名
   - 根据 `ActivityType` 选择报名方式：
     - PROMOTION：报名到促销活动
     - TIME_LIMITED：创建限时折扣活动

4. **构建活动配置**
   - 根据 `ActivityDiscountRate` 计算活动价格
   - 根据 `ActivityStockRatio` 计算活动库存
   - 为每个产品的每个站点设置活动价格

5. **提交报名**
   - 调用 SHEIN API 保存活动配置
   - 记录报名结果

### 限时折扣活动创建流程

1. **获取运营策略**
   - 根据店铺ID获取运营策略配置
   - 检查是否启用活动功能和限时折扣类型

2. **获取店铺信息**
   - 获取店铺详细信息（用户名等）
   - 用于生成活动名称

3. **构建活动配置**
   - 生成活动名称（格式：#用户名#限时折扣#日期#序号）
   - 设置活动时间（默认7天）
   - 根据策略调整库存数量

4. **查询可参加活动的商品**
   - 调用 `QueryPromotionGoods` API 查询商品
   - 与促销活动使用不同的接口

5. **计算价格和利润**
   - 调用 `CalculateSupplyPrice` API 计算
   - 进行价格风险检查

6. **创建限时折扣活动**
   - 调用 `CreateActivity` API 创建活动
   - 记录创建结果

## API 接口

### ActivityRegistrationService

```go
type ActivityRegistrationService interface {
    // 根据运营策略报名促销活动（完整流程：获取产品 → 构建配置 → 提交报名）
    RegisterPromotionActivity(ctx context.Context, strategy *OperationStrategyDTO) (int, error)
    
    // 根据运营策略创建限时折扣活动（完整流程：查询商品 → 计算价格 → 创建活动）
    CreateTimeLimitedDiscountActivity(ctx context.Context, strategy *OperationStrategyDTO) (int, error)
    
    // 根据运营策略按比例执行混合活动（部分促销 + 部分限时折扣）
    RegisterMixedActivity(ctx context.Context, strategy *OperationStrategyDTO) (promotionCount int, timeLimitedCount int, err error)
}
```

**接口设计说明**：
- 接口只暴露三个高层业务方法，对应三种活动类型
- 底层方法（如 `fetchAvailableProducts`、`queryPromotionGoods`、`calculateSupplyPrice`、`createTimeLimitedDiscount`）都是私有方法
- 每个方法都是完整的业务流程，调用者无需关心内部实现细节

## 日志示例

```
INFO  开始根据运营策略报名产品到活动 product_count=50 store_id=687
INFO  报名产品到促销活动 discount_rate=0.15 stock_ratio=0.8
INFO  成功报名 45 个产品到促销活动
INFO  SHEIN活动报名任务执行完成 total_products=50 registered_products=45
```

## 注意事项

1. **折扣率范围**：建议设置在 0.05-0.3 之间（5%-30%折扣）
2. **库存比例**：建议设置在 0.5-1.0 之间（50%-100%库存）
3. **已配置产品**：已经配置过的产品会自动跳过
4. **接口差异**：
   - 促销活动使用 `GetAvailableSkcList` 接口获取产品列表
   - 限时折扣使用 `QueryPromotionGoods` 接口查询商品
5. **限时折扣活动**：
   - 活动时间默认为7天（从当前时间开始）
   - 活动名称自动生成（格式：#用户名#限时折扣#日期#序号）
   - 会自动查询可参加活动的商品并计算价格
   - 支持价格风险检查

## 限时折扣配置说明

限时折扣活动使用 `TimeLimitedDiscountConfig` 配置，主要参数：

| 参数 | 说明 | 默认值 |
|------|------|--------|
| `ActivityName` | 活动名称 | 自动生成 |
| `TimeZone` | 时区 | America/Los_Angeles |
| `StartTime` | 开始时间 | 当前时间 |
| `EndTime` | 结束时间 | 开始时间+7天 |
| `DefaultStockNum` | 默认库存数量 | 30（会根据策略的库存比例调整） |
| `AllowRiskProducts` | 是否允许有风险的商品 | false |

## 未来计划

- [ ] 支持自定义活动时间范围
- [ ] 支持多活动并行管理
- [ ] 支持活动效果分析和自动优化
- [ ] 支持按产品类别设置不同的折扣策略
