# SHEIN 营销活动 API

本模块提供 SHEIN 平台的营销活动相关 API 接口实现。

## 📦 模块结构

```
marketing/
├── interface.go           # API 接口定义
├── promotion_types.go     # 促销商品查询相关类型
├── price_types.go         # 价格计算相关类型
├── activity_types.go      # 活动创建相关类型
├── promotion_example.go   # 单个接口使用示例
└── workflow_example.go    # 完整流程示例
```

## 🎯 核心功能

### 1. 查询促销商品 (QueryPromotionGoods)

查询可参加限时折扣活动的商品列表。

**请求参数：**
- 活动基础信息（名称、时间、时区等）
- 生效中心列表
- 是否上架
- 分页参数

**返回数据：**
- 商品列表（SKC、SKU、价格、库存等）
- 供货价格信息
- 库存检查信息
- 风险警告信息

### 2. 计算供货价格 (CalculateSupplyPrice)

计算促销活动中商品的价格和利润分解。

**请求参数：**
- 币种
- SKC/SKU 列表及价格
- 活动时间范围

**返回数据：**
- 商品原价
- 结算金额
- 促销金额
- 优惠券金额
- 折扣金额
- 库存费用
- 绩效金额
- 风险标签

### 3. 创建活动 (CreateActivity)

创建限时折扣促销活动。

**请求参数：**
- 活动基础信息（名称、时间、规则等）
- 商品成本和库存信息列表
- 定价类型

**返回数据：**
- 活动 ID
- 错误信息（如有）

## 🔄 完整流程

```
1. QueryPromotionGoods
   ↓ 获取可参加活动的商品
   
2. CalculateSupplyPrice
   ↓ 计算价格和利润，检查风险
   
3. CreateActivity
   ↓ 创建限时折扣活动
   
✅ 活动创建成功
```

## 📝 使用示例

详细的使用示例请参考：
- `promotion_example.go` - 单个接口调用示例
- `workflow_example.go` - 完整流程示例

## ⚠️ 注意事项

1. **活动名称格式**: `#用户名#限时折扣#日期#序号`
2. **时区一致性**: 所有时间参数必须使用相同时区
3. **价格检查**: 计算价格后务必检查 `risk_tag` 和 `warning_value`
4. **库存限制**: 活动库存不能超过可用库存
5. **时间验证**: 活动开始时间必须在未来

## 🔗 相关文件

- 接口实现: `internal/platforms/shein/repo/marketing_repo.go`
- 端点定义: `internal/platforms/shein/repo/client/endpoint.go`
- 端点注册: `internal/platforms/shein/repo/client/endpoints.go`
