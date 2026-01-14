# SHEIN 调度器服务

本模块提供 SHEIN 平台的调度器相关服务实现。

## 📦 模块结构

```
scheduler/
├── activity_registration.go              # 活动报名服务核心实现
├── time_limited_discount.go              # 限时折扣活动实现
├── activity_config.go                    # 活动配置定义
├── activity_errors.go                    # 错误定义
├── time_limited_discount_example.go      # 使用示例
├── auto_pricing.go                       # 自动核价服务
├── inventory_sync.go                     # 库存同步服务
├── product_sync.go                       # 商品同步服务
└── README.md                             # 本文档
```

## 🎯 核心功能

### 1. 活动报名服务 (ActivityRegistrationService)

提供两大类功能：

#### A. 自动报名活动
- `FetchAvailableProducts` - 获取可报名活动的产品列表
- `RegisterProducts` - 自动报名产品到活动

#### B. 限时折扣活动
- `QueryPromotionGoods` - 查询促销活动商品列表
- `CalculateSupplyPrice` - 计算供货价格和利润
- `CreateTimeLimitedDiscount` - 创建限时折扣活动
- `AutoCreateTimeLimitedDiscount` - 自动创建限时折扣活动（完整流程）

## 🔄 限时折扣完整流程

```
1. 配置验证
   ↓
2. 查询可参加活动的商品 (QueryPromotionGoods)
   ↓
3. 计算商品价格和利润 (CalculateSupplyPrice)
   ↓
4. 检查价格风险
   ↓
5. 构建活动创建请求
   ↓
6. 创建限时折扣活动 (CreateTimeLimitedDiscount)
   ↓
7. 检查创建结果
   ↓
✅ 活动创建成功
```

## 📝 使用方式

### 方式一：自动创建（推荐）

```go
// 1. 创建服务
activityService := NewActivityRegistrationService(managementClient, marketingAPI)

// 2. 配置活动
config := DefaultTimeLimitedDiscountConfig()
config.ActivityName = GenerateActivityName("yangyou922", 1)
config.StartTime = time.Now().Add(24 * time.Hour)
config.EndTime = time.Now().Add(30 * 24 * time.Hour)

// 3. 执行创建
err := activityService.AutoCreateTimeLimitedDiscount(ctx, config)
```

### 方式二：手动控制流程

```go
// 1. 查询商品
queryResp, err := activityService.QueryPromotionGoods(ctx, queryReq)

// 2. 计算价格
calcResp, err := activityService.CalculateSupplyPrice(ctx, calcReq)

// 3. 创建活动
createResp, err := activityService.CreateTimeLimitedDiscount(ctx, createReq)
```

## ⚙️ 配置说明

### TimeLimitedDiscountConfig

| 字段 | 类型 | 说明 | 默认值 |
|------|------|------|--------|
| ActivityName | string | 活动名称 | - |
| TimeZone | string | 时区 | America/Los_Angeles |
| StartTime | time.Time | 开始时间 | - |
| EndTime | time.Time | 结束时间 | - |
| RefToolID | int | 工具ID | 30 |
| SubTypeID | int | 子类型ID | 2 |
| GoodsLimit | int | 商品限制 | 1 |
| GoodsLimitNum | int | 商品限制数量 | 1 |
| EffectiveCenterList | []int | 生效中心列表 | [2] |
| IsShelf | int | 是否上架 | 1 |
| PageSize | int | 每页查询数量 | 30 |
| Currency | string | 币种 | USD |
| SceneID | int | 场景ID | 1 |
| PricingType | int | 定价类型 | 2 |
| DefaultAttendNum | int | 默认参与数量 | 30 |
| DefaultStockNum | int | 默认库存数量 | 30 |
| AllowRiskProducts | bool | 是否允许风险商品 | false |
| MaxWarningValue | int | 最大警告值 | 0 |

## 🛡️ 错误处理

### 配置相关错误
- `ErrInvalidActivityName` - 活动名称不能为空
- `ErrInvalidActivityTime` - 活动时间不能为空
- `ErrInvalidActivityTimeRange` - 活动时间范围无效
- `ErrInvalidTimeZone` - 时区不能为空

### 商品相关错误
- `ErrNoAvailableProducts` - 没有可用的商品
- `ErrProductPriceRisk` - 商品价格存在风险
- `ErrInsufficientStock` - 商品库存不足

### 活动创建错误
- `ErrActivityCreationFailed` - 活动创建失败
- `ErrActivityAlreadyExists` - 活动已存在

## 🔧 工具函数

### GenerateActivityName

生成符合规范的活动名称。

```go
name := GenerateActivityName("yangyou922", 1)
// 输出: #yangyou922#限时折扣#2026-01-14#1
```

格式：`#用户名#限时折扣#日期#序号`

## 📚 详细示例

详细的使用示例请参考：
- `time_limited_discount_example.go` - 完整的使用示例代码

## 🔗 相关模块

- API 接口定义: `internal/platforms/shein/api/marketing/`
- Repo 实现: `internal/platforms/shein/repo/marketing_repo.go`
- Task 调度: `internal/platforms/shein/scheduler/activity_task.go`

## ⚠️ 注意事项

1. **活动名称格式**: 必须严格遵循 `#用户名#限时折扣#日期#序号` 格式
2. **时区一致性**: 所有时间参数必须使用相同时区
3. **价格风险**: 创建前务必检查价格计算结果的风险标签
4. **库存限制**: 活动库存不能超过商品可用库存
5. **时间验证**: 活动开始时间必须在未来
6. **并发控制**: 同一商品不能同时参加多个活动
