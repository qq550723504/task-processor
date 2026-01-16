# 限时折扣活动配置选项说明

## 概述

限时折扣活动新增了多个重要的配置选项，用于更灵活地控制活动规则、定价策略和库存管理。支持两种定价模式：按折扣率定价和按最低利润率定价。

## 配置选项

### 1. 定价模式配置

控制活动价格的计算方式。

#### 字段说明

- **`activityPriceMode`** (string) - 通用定价模式
  - 适用于所有活动类型（促销活动、限时折扣）
  - 可选值:
    - `DISCOUNT`: 按折扣率定价
    - `PROFIT`: 按最低利润率定价
  - 默认值: `DISCOUNT`

- **`timeLimitedPriceMode`** (string) - 限时折扣专属定价模式
  - 仅适用于限时折扣活动
  - 优先级高于通用配置
  - 可选值同上
  - 默认值: 使用通用配置

#### 定价模式说明

**DISCOUNT 模式（按折扣率）**
- 根据设定的折扣率计算活动价格
- 计算公式: `活动价格 = 原价 × (1 - 折扣率)`
- 适用场景: 统一折扣促销、清仓甩卖

**PROFIT 模式（按最低利润率）**
- 根据成本价和最低利润率计算活动价格
- 计算公式: `活动价格 = 成本价 / (1 - 最低利润率)`
- 适用场景: 保证利润底线、精细化定价

#### 示例配置

```json
{
  "activityPriceMode": "PROFIT",
  "activityMinProfitRate": 0.15
}
```

---

### 2. 折扣率配置

控制按折扣率定价时的折扣力度（仅在 DISCOUNT 模式下生效）。

#### 字段说明

- **`activityDiscountRate`** (float64) - 通用折扣率
  - 适用于所有活动类型
  - 取值范围: 0-1 之间
  - 默认值: `0.1` (10% off)

- **`timeLimitedDiscountRate`** (float64) - 限时折扣专属折扣率
  - 仅适用于限时折扣活动
  - 优先级高于通用配置
  - 取值范围: 0-1 之间
  - 默认值: `0.4` (40% off，打 6 折)

#### 价格计算公式

**DISCOUNT 模式:**
```
活动价格 = 原价 × (1 - 折扣率)
```

示例：
- 原价: $100
- 折扣率: 0.4 (40% off)
- 活动价格: $100 × (1 - 0.4) = $100 × 0.6 = $60

#### 使用场景

- **高折扣 (0.4-0.6)**: 清仓促销、爆款引流
- **中等折扣 (0.2-0.4)**: 常规促销活动
- **低折扣 (0.1-0.2)**: 会员专享、小幅优惠

#### 示例配置

```json
{
  "activityPriceMode": "DISCOUNT",
  "timeLimitedDiscountRate": 0.4
}
```

上述配置表示：使用折扣率模式，商品打 6 折（40% off）。

---

### 3. 最低利润率配置

控制按最低利润率定价时的利润底线（仅在 PROFIT 模式下生效）。

#### 字段说明

- **`activityMinProfitRate`** (float64) - 通用最低利润率
  - 适用于所有活动类型
  - 取值范围: 0-1 之间
  - 计算方式: 利润率 = (售价 - 成本) / 售价
  - 默认值: `0.15` (15% 利润率)

- **`timeLimitedMinProfitRate`** (float64) - 限时折扣专属最低利润率
  - 仅适用于限时折扣活动
  - 优先级高于通用配置
  - 取值范围: 0-1 之间
  - 默认值: 使用通用配置

#### 价格计算公式

**PROFIT 模式:**
```
活动价格 = 成本价 / (1 - 最低利润率)
```

示例：
- 成本价: $50
- 最低利润率: 0.15 (15%)
- 活动价格: $50 / (1 - 0.15) = $50 / 0.85 = $58.82

验证利润率:
- 利润 = $58.82 - $50 = $8.82
- 利润率 = $8.82 / $58.82 = 15%

#### 使用场景

- **高利润率 (0.25-0.40)**: 高端商品、品牌溢价
- **中等利润率 (0.15-0.25)**: 常规商品、保证利润
- **低利润率 (0.05-0.15)**: 走量商品、薄利多销

#### 风险控制

当原价低于按最低利润率计算的价格时，系统会：
1. 记录警告日志
2. 自动降级使用折扣率模式
3. 确保活动正常进行

#### 示例配置

```json
{
  "activityPriceMode": "PROFIT",
  "activityMinProfitRate": 0.15
}
```

上述配置表示：使用利润率模式，保证至少 15% 的利润率。

---

### 4. 单用户限购配置

控制单个用户在活动期间可购买的商品数量。

#### 字段说明

- **`timeLimitedUserLimit`** (bool)
  - 是否启用单用户限购
  - `true`: 启用限购，用户购买数量受限
  - `false`: 不限购，用户可无限购买（受库存限制）
  - 默认值: `false`

- **`timeLimitedUserLimitNum`** (int)
  - 单用户限购数量
  - 仅当 `timeLimitedUserLimit = true` 时生效
  - 取值范围: >= 1
  - 默认值: `1`

#### 使用场景

- **启用限购**: 适用于爆款商品、稀缺商品，防止单个用户囤货
- **不限购**: 适用于普通促销商品，鼓励用户多买

#### 示例配置

```json
{
  "timeLimitedUserLimit": true,
  "timeLimitedUserLimitNum": 3
}
```

上述配置表示：每个用户最多可购买 3 件该商品。

---

### 5. 活动库存限量配置

控制活动期间投放的库存数量。

#### 字段说明

- **`timeLimitedStockLimit`** (bool)
  - 是否启用活动库存限量
  - `true`: 启用限量，按百分比投放库存
  - `false`: 不限量，投放全部可用库存
  - 默认值: `false`

- **`timeLimitedStockLimitPercent`** (int)
  - 活动库存限量百分比
  - 仅当 `timeLimitedStockLimit = true` 时生效
  - 取值范围: 1-100
  - 默认值: `100`

#### 使用场景

- **启用限量**: 
  - 分批投放库存，避免一次性售罄
  - 控制活动节奏，延长活动热度
  - 预留库存用于其他渠道或活动

- **不限量**: 
  - 清仓促销，快速去库存
  - 常规促销活动

#### 库存计算逻辑

```
活动库存 = 商品实际库存 × (timeLimitedStockLimitPercent / 100)
```

如果计算结果小于 1，则至少投放 1 件库存。

#### 示例配置

```json
{
  "timeLimitedStockLimit": true,
  "timeLimitedStockLimitPercent": 50
}
```

上述配置表示：投放商品实际库存的 50% 用于活动。

例如：
- 商品实际库存: 100 件
- 活动库存: 100 × 50% = 50 件
- 剩余 50 件可用于其他渠道或后续活动

---

## 配置组合示例

### 场景 1: 爆款限量抢购（高折扣模式）

```json
{
  "activityPriceMode": "DISCOUNT",
  "timeLimitedDiscountRate": 0.5,
  "timeLimitedUserLimit": true,
  "timeLimitedUserLimitNum": 1,
  "timeLimitedStockLimit": true,
  "timeLimitedStockLimitPercent": 30
}
```

- 按折扣率定价
- 50% off（打 5 折）
- 每人限购 1 件
- 仅投放 30% 库存
- 适用于制造稀缺感，提升转化率

### 场景 2: 保利润促销（利润率模式）

```json
{
  "activityPriceMode": "PROFIT",
  "activityMinProfitRate": 0.20,
  "timeLimitedUserLimit": false,
  "timeLimitedStockLimit": false
}
```

- 按最低利润率定价
- 保证 20% 利润率
- 不限购
- 投放全部库存
- 适用于保证利润底线的促销

### 场景 3: 常规促销（折扣模式）

```json
{
  "activityPriceMode": "DISCOUNT",
  "timeLimitedDiscountRate": 0.3,
  "timeLimitedUserLimit": false,
  "timeLimitedStockLimit": false
}
```

- 按折扣率定价
- 30% off（打 7 折）
- 不限购
- 投放全部库存
- 适用于日常促销活动

### 场景 4: 分批投放（灵活控制）

```json
{
  "activityPriceMode": "DISCOUNT",
  "timeLimitedDiscountRate": 0.35,
  "timeLimitedUserLimit": true,
  "timeLimitedUserLimitNum": 5,
  "timeLimitedStockLimit": true,
  "timeLimitedStockLimitPercent": 60
}
```

- 按折扣率定价
- 35% off（打 6.5 折）
- 每人限购 5 件
- 投放 60% 库存
- 适用于多轮次活动，预留库存用于后续批次

### 场景 5: 会员专享（利润率保底）

```json
{
  "activityPriceMode": "PROFIT",
  "activityMinProfitRate": 0.10,
  "timeLimitedUserLimit": true,
  "timeLimitedUserLimitNum": 10,
  "timeLimitedStockLimit": false
}
```

- 按最低利润率定价
- 保证 10% 利润率
- 每人限购 10 件
- 不限量
- 适用于会员福利、薄利多销

### 场景 6: 混合定价（限时折扣专属配置）

```json
{
  "activityPriceMode": "DISCOUNT",
  "activityDiscountRate": 0.2,
  "timeLimitedPriceMode": "PROFIT",
  "timeLimitedMinProfitRate": 0.15,
  "timeLimitedUserLimit": true,
  "timeLimitedUserLimitNum": 3
}
```

- 促销活动：按 20% 折扣率定价
- 限时折扣：按 15% 利润率定价（专属配置优先）
- 每人限购 3 件
- 适用于不同活动类型使用不同定价策略

---

## 技术实现

### 数据结构

#### OperationStrategyDTO (运营策略)

```go
type OperationStrategyDTO struct {
    // ... 其他字段 ...
    
    // 通用定价配置（适用于所有活动类型）
    ActivityMinProfitRate float64 `json:"activityMinProfitRate"`
    ActivityPriceMode     string  `json:"activityPriceMode"`
    
    // 限时折扣专属配置
    TimeLimitedDiscountRate      float64 `json:"timeLimitedDiscountRate"`
    TimeLimitedMinProfitRate     float64 `json:"timeLimitedMinProfitRate"`
    TimeLimitedPriceMode         string  `json:"timeLimitedPriceMode"`
    TimeLimitedUserLimit         bool    `json:"timeLimitedUserLimit"`
    TimeLimitedUserLimitNum      int     `json:"timeLimitedUserLimitNum"`
    TimeLimitedStockLimit        bool    `json:"timeLimitedStockLimit"`
    TimeLimitedStockLimitPercent int     `json:"timeLimitedStockLimitPercent"`
}
```

#### TimeLimitedDiscountConfig (活动配置)

```go
type TimeLimitedDiscountConfig struct {
    // ... 其他字段 ...
    
    // 活动规则
    DiscountRate  float64 // 折扣率（0-1之间）
    MinProfitRate float64 // 最低利润率（0-1之间）
    PriceMode     string  // 定价模式（DISCOUNT/PROFIT）
    GoodsLimit    int     // 商品限制（0:不限购, 1:限购）
    GoodsLimitNum int     // 商品限制数量
    StockLimit    bool    // 是否启用活动库存限量
    StockPercent  int     // 活动库存限量百分比（1-100）
}
```

### 配置优先级

配置读取遵循以下优先级（从高到低）：

1. **限时折扣专属配置** (`timeLimitedXxx`)
2. **通用活动配置** (`activityXxx`)
3. **默认值**

示例：
```go
// 定价模式优先级
if strategy.TimeLimitedPriceMode != "" {
    config.PriceMode = strategy.TimeLimitedPriceMode  // 优先级1
} else if strategy.ActivityPriceMode != "" {
    config.PriceMode = strategy.ActivityPriceMode     // 优先级2
}
// 否则使用默认值 "DISCOUNT"                           // 优先级3
```

### 价格计算逻辑

#### 按折扣率计算

```go
func calculatePriceByDiscount(originalPrice float64, discountRate float64) float64 {
    return originalPrice * (1 - discountRate)
}
```

#### 按最低利润率计算

```go
func calculatePriceByProfit(originalPrice float64, costPrice float64, minProfitRate float64) (float64, error) {
    // 计算最低售价 = 成本价 / (1 - 最低利润率)
    minPrice := costPrice / (1 - minProfitRate)
    
    // 如果原价低于最低售价，返回错误
    if originalPrice < minPrice {
        return 0, fmt.Errorf("原价低于最低售价")
    }
    
    return minPrice, nil
}
```

#### 根据定价模式计算

```go
func calculateActivityPrice(config TimeLimitedDiscountConfig, originalPrice float64, costPrice float64) (float64, error) {
    switch config.PriceMode {
    case "PROFIT":
        return calculatePriceByProfit(originalPrice, costPrice, config.MinProfitRate)
    case "DISCOUNT":
        return calculatePriceByDiscount(originalPrice, config.DiscountRate), nil
    default:
        return calculatePriceByDiscount(originalPrice, config.DiscountRate), nil
    }
}
```

### 库存计算逻辑

```go
// 确定库存数量
stockNum := config.DefaultStockNum

// 如果启用了库存限量，按百分比计算
if config.StockLimit && actualInventory > 0 {
    stockNum = int(float64(actualInventory) * float64(config.StockPercent) / 100.0)
    if stockNum < 1 {
        stockNum = 1 // 至少1个
    }
} else if actualInventory > 0 && actualInventory < stockNum {
    // 如果不限量，但实际库存小于默认值，使用实际库存
    stockNum = actualInventory
}
```

---

## 注意事项

1. **定价模式**: 
   - `activityPriceMode` 和 `timeLimitedPriceMode` 必须是 `DISCOUNT` 或 `PROFIT`
   - 限时折扣专属配置优先级高于通用配置

2. **折扣率**: 
   - `timeLimitedDiscountRate` 和 `activityDiscountRate` 必须在 0-1 之间
   - 建议范围 0.1-0.6

3. **最低利润率**: 
   - `timeLimitedMinProfitRate` 和 `activityMinProfitRate` 必须在 0-1 之间
   - 建议范围 0.05-0.40
   - 利润率过高可能导致活动价格不具竞争力

4. **限购数量**: `timeLimitedUserLimitNum` 必须 >= 1

5. **库存百分比**: `timeLimitedStockLimitPercent` 必须在 1-100 之间

6. **最小库存**: 无论如何配置，活动库存至少为 1 件（如果商品有库存）

7. **价格风险控制**: 
   - 使用 PROFIT 模式时，如果原价低于按利润率计算的最低价格，系统会自动降级使用 DISCOUNT 模式
   - 系统会记录警告日志，便于后续分析

8. **兼容性**: 
   - 保留了旧的 `activityDiscountRate` 和 `activityStockRatio` 配置
   - 新配置优先级高于旧配置

9. **日志记录**: 所有配置变更和价格计算都会记录到日志中，便于追踪和调试

---

## 相关文件

- `internal/pkg/management/api/operation_strategy.go` - 运营策略数据结构
- `internal/platforms/shein/service/scheduler/activity_config.go` - 活动配置定义
- `internal/platforms/shein/service/scheduler/activity_registration_config.go` - 配置构建逻辑
- `internal/platforms/shein/service/scheduler/time_limited_discount.go` - 活动创建逻辑
- `internal/platforms/shein/service/scheduler/price_calculator.go` - 价格计算逻辑
