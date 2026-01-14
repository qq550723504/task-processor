# SHEIN 限时折扣活动实现文档

## 📋 概述

本文档描述了 SHEIN 平台限时折扣活动的完整实现，包括 API 接口、服务层和调度器的集成。

## 🏗️ 架构设计

### 分层架构

```
┌─────────────────────────────────────────┐
│         Scheduler Layer (调度层)         │
│  ActivityTask - 任务调度和执行           │
└─────────────────┬───────────────────────┘
                  │
┌─────────────────▼───────────────────────┐
│         Service Layer (服务层)           │
│  ActivityRegistrationService             │
│  - 活动报名                              │
│  - 限时折扣创建                          │
└─────────────────┬───────────────────────┘
                  │
┌─────────────────▼───────────────────────┐
│         Repository Layer (仓储层)        │
│  MarketingAPI                            │
│  - QueryPromotionGoods                   │
│  - CalculateSupplyPrice                  │
│  - CreateActivity                        │
└─────────────────┬───────────────────────┘
                  │
┌─────────────────▼───────────────────────┐
│         API Layer (接口层)               │
│  SHEIN Platform API                      │
└─────────────────────────────────────────┘
```

## 📦 实现的文件

### 1. API 层 (internal/platforms/shein/api/marketing/)

| 文件 | 说明 |
|------|------|
| interface.go | 营销 API 接口定义 |
| promotion_types.go | 促销商品查询相关类型 |
| price_types.go | 价格计算相关类型 |
| activity_types.go | 活动创建相关类型 |
| promotion_example.go | 使用示例 |
| workflow_example.go | 完整流程示例 |
| README.md | API 文档 |

### 2. Repository 层 (internal/platforms/shein/repo/)

| 文件 | 说明 |
|------|------|
| marketing_repo.go | 营销 API 实现 |
| marketing_repo_interface.go | 营销 API 接口 |
| client/endpoint.go | API 端点定义 |
| client/endpoints.go | 端点注册 |

### 3. Service 层 (internal/platforms/shein/service/scheduler/)

| 文件 | 说明 |
|------|------|
| activity_registration.go | 活动服务核心实现 |
| time_limited_discount.go | 限时折扣业务逻辑 |
| activity_config.go | 配置定义 |
| activity_errors.go | 错误定义 |
| time_limited_discount_example.go | 使用示例 |
| README.md | 服务文档 |

### 4. Scheduler 层 (internal/platforms/shein/scheduler/)

| 文件 | 说明 |
|------|------|
| activity_task.go | 活动任务实现 |
| factory.go | 任务工厂（已支持） |

## 🎯 核心功能

### 1. 查询促销商品 (QueryPromotionGoods)

**功能**: 查询可参加限时折扣活动的商品列表

**API 端点**: `/mrs-api-prefix/promotion/simple_platform/query_goods`

**主要参数**:
- 活动基础信息（名称、时间、时区）
- 生效中心列表
- 是否上架
- 分页参数

**返回数据**:
- 商品列表（SKC、SKU、价格、库存）
- 供货价格信息
- 风险警告信息

### 2. 计算供货价格 (CalculateSupplyPrice)

**功能**: 计算促销活动中商品的价格和利润分解

**API 端点**: `/mrs-api-prefix/capital/loss/calculate_supply_price`

**主要参数**:
- 币种
- SKC/SKU 列表及价格
- 活动时间范围

**返回数据**:
- 商品原价
- 结算金额
- 促销金额
- 优惠券金额
- 折扣金额
- 风险标签

### 3. 创建活动 (CreateActivity)

**功能**: 创建限时折扣促销活动

**API 端点**: `/mrs-api-prefix/promotion/simple_platform/create_activity`

**主要参数**:
- 活动基础信息
- 商品成本和库存信息
- 定价类型

**返回数据**:
- 活动 ID
- 错误信息（如有）

## 🔄 完整流程

### 自动创建流程

```go
// 1. 配置活动
config := DefaultTimeLimitedDiscountConfig()
config.ActivityName = GenerateActivityName("yangyou922", 1)
config.StartTime = time.Now().Add(24 * time.Hour)
config.EndTime = time.Now().Add(30 * 24 * time.Hour)

// 2. 执行自动创建
err := activityService.AutoCreateTimeLimitedDiscount(ctx, config)
```

### 内部执行步骤

1. **配置验证** - 验证活动名称、时间、时区等
2. **查询商品** - 调用 QueryPromotionGoods 获取可用商品
3. **计算价格** - 调用 CalculateSupplyPrice 计算价格和利润
4. **风险检查** - 检查价格风险标签和警告值
5. **构建请求** - 构建活动创建请求参数
6. **创建活动** - 调用 CreateActivity 创建活动
7. **结果检查** - 检查创建结果和错误信息

## 📊 数据流

```
用户配置
    ↓
[配置验证]
    ↓
[查询商品] → SHEIN API (query_goods)
    ↓
商品列表
    ↓
[计算价格] → SHEIN API (calculate_supply_price)
    ↓
价格信息
    ↓
[风险检查]
    ↓
[构建请求]
    ↓
[创建活动] → SHEIN API (create_activity)
    ↓
活动 ID
```

## 🔧 配置说明

### 默认配置

```go
config := DefaultTimeLimitedDiscountConfig()
// TimeZone: "America/Los_Angeles"
// RefToolID: 30
// SubTypeID: 2
// Currency: "USD"
// DefaultAttendNum: 30
// DefaultStockNum: 30
// AllowRiskProducts: false
```

### 必填配置

- `ActivityName` - 活动名称（格式：#用户名#限时折扣#日期#序号）
- `StartTime` - 开始时间
- `EndTime` - 结束时间

### 可选配置

- `EffectiveCenterList` - 生效中心列表
- `IsShelf` - 是否上架
- `PageSize` - 每页查询数量
- `DefaultAttendNum` - 默认参与数量
- `DefaultStockNum` - 默认库存数量
- `AllowRiskProducts` - 是否允许风险商品
- `MaxWarningValue` - 最大警告值

## 🛡️ 错误处理

### 错误类型

| 错误 | 说明 | 处理方式 |
|------|------|----------|
| ErrInvalidActivityName | 活动名称无效 | 检查名称格式 |
| ErrInvalidActivityTime | 活动时间无效 | 检查时间设置 |
| ErrNoAvailableProducts | 没有可用商品 | 调整筛选条件 |
| ErrProductPriceRisk | 价格存在风险 | 调整价格或允许风险 |
| ErrActivityCreationFailed | 创建失败 | 检查 API 响应 |

### 错误处理示例

```go
err := activityService.AutoCreateTimeLimitedDiscount(ctx, config)
if err != nil {
    switch {
    case errors.Is(err, ErrNoAvailableProducts):
        log.Println("没有可用商品，请调整筛选条件")
    case errors.Is(err, ErrProductPriceRisk):
        log.Println("商品价格存在风险，请检查价格设置")
    default:
        log.Printf("创建活动失败: %v", err)
    }
}
```

## 🔗 集成方式

### 在 Task 中使用

```go
func (t *ActivityTask) ExecuteTimeLimitedDiscount(ctx context.Context) error {
    // 生成活动名称
    activityName := GenerateActivityName("yangyou922", 1)
    
    // 配置活动
    config := DefaultTimeLimitedDiscountConfig()
    config.ActivityName = activityName
    config.StartTime = time.Now().Add(24 * time.Hour)
    config.EndTime = time.Now().Add(30 * 24 * time.Hour)
    
    // 执行创建
    return t.activityService.AutoCreateTimeLimitedDiscount(ctx, config)
}
```

### 在 Factory 中注册

ActivityTask 已经在 Factory 中注册，支持 `TaskTypeActivity` 类型。

## ⚠️ 注意事项

1. **活动名称格式**: 必须严格遵循 `#用户名#限时折扣#日期#序号` 格式
2. **时区一致性**: 所有时间参数必须使用相同时区
3. **价格风险**: 创建前务必检查价格计算结果的风险标签
4. **库存限制**: 活动库存不能超过商品可用库存
5. **时间验证**: 活动开始时间必须在未来
6. **并发控制**: 同一商品不能同时参加多个活动

## 📈 扩展性

### 支持的扩展点

1. **自定义价格策略**: 修改 `buildCalculateRequest` 中的折扣逻辑
2. **商品筛选规则**: 扩展 `buildQueryRequest` 添加更多筛选条件
3. **风险控制策略**: 自定义 `validatePriceRisk` 的检查逻辑
4. **活动规则**: 修改 `ActivityRule` 支持更多规则类型

### 未来优化方向

1. 支持批量创建多个活动
2. 支持活动模板配置
3. 支持活动效果分析
4. 支持活动自动续期
5. 支持活动冲突检测

## 📚 参考文档

- [SHEIN API 文档](internal/platforms/shein/api/marketing/README.md)
- [服务层文档](internal/platforms/shein/service/scheduler/README.md)
- [使用示例](internal/platforms/shein/service/scheduler/time_limited_discount_example.go)
- [完整流程示例](internal/platforms/shein/api/marketing/workflow_example.go)

## ✅ 测试建议

1. **单元测试**: 测试各个方法的独立功能
2. **集成测试**: 测试完整流程的执行
3. **边界测试**: 测试异常情况和边界条件
4. **性能测试**: 测试大批量商品的处理性能

## 🎉 总结

本实现提供了完整的 SHEIN 限时折扣活动创建功能，包括：

- ✅ 3 个核心 API 接口
- ✅ 完整的服务层封装
- ✅ 自动化流程支持
- ✅ 灵活的配置系统
- ✅ 完善的错误处理
- ✅ 详细的文档和示例

可以直接集成到现有的调度器系统中使用。
