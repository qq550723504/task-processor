# ListingKit 保本活动价设计

## 目标

修复 ListingKit 限时活动的 `BREAKEVEN` 报名：每个 SKU 的最终活动价必须等于该 SKU 的 SHEIN 成本加固定调整值。

## 现状与根因

报名计算阶段已为每个 SKU 生成保本目标活动价。但 `buildCreateActivityRequest` 仅在 `PROFIT` 模式保留请求中的目标价；`BREAKEVEN` 会回退到价格计算接口的返回差额，导致最终价格低于成本。

## 方案

将“使用请求中的 SKU 活动价”条件从仅 `PROFIT` 扩展至 `PROFIT` 和 `BREAKEVEN`。保留价格计算接口调用及其风险校验；仅在创建活动请求时，保本 SKU 使用先前算出的目标活动价。

## 验收

- 保本 SKU 最终 `AddSkuList[].ProductActPrice` 等于 `SKU 成本 + 固定调整值`。
- 即使价格计算接口返回不同的价格差额，最终保本活动价也不被覆盖。
- `PROFIT` 与折扣模式行为不变。
