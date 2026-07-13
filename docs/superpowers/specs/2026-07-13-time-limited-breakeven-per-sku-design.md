# 限时折扣保本价按 SKU 提交

## 目标

修复旧的 `CreateTimeLimitedDiscountActivity` 流程：当定价模式为 `BREAKEVEN` 时，不能再按折扣率计算。每个 SKU 应以自身价格与成本计算保本活动价，并独立提交到 SHEIN。

## 范围

- 仅修改旧限时折扣入口调用的 `buildCalculateRequestWithPriceMode` 及其测试。
- 不改变候选商品自动报名路径；该路径已经通过 `buildCalculateRequestForPromotionProducts` 支持逐 SKU 保本价。
- 不改变 `DISCOUNT` 或 `PROFIT` 的既有行为。

## 数据与计算

旧入口从 SHEIN 活动商品接口取得每个 SKU 的美元供货价，作为 SKU 原价。成本从本地已同步的商品 Attributes 中按 SKC 和 SKU 编码读取 `AmazonMonitorData.Price`。

对每个 SKU：

`活动价 = 成本价 + FixedPriceAdjustment`

仅在原价、成本价和活动价均有效且 `活动价 < 原价` 时，该 SKU 才会进入价格计算请求。缺成本、缺价格或保本价不低于原价的 SKU 被单独跳过。

一个 SKC 至少保留一个有效 SKU 才进入请求；因此同一 SKC 的其他可报名 SKU 不会被无效 SKU 阻塞。

## 请求与结果

价格计算请求中的每个 `SkuPriceInfo` 使用该 SKU 的原价和独立活动价。后续创建请求已按 SKU 构造 `AddSkuList`，会提交每个有效 SKU 的活动价；SKC 代表价沿用所有有效 SKU 活动价中的最低值。

## 错误处理与可观测性

- 本地商品数据无法加载时，`BREAKEVEN` 不能安全推导成本；该 SKC 的 SKU 不进入请求。
- 记录 SKU 级跳过原因，避免将局部数据问题误报为整个 SKC 的活动失败。

## 测试

新增或扩展单元测试以证明：

1. 多 SKU 以各自成本生成不同保本活动价。
2. 无效 SKU 被排除而有效 SKU 仍被提交。
3. 全部 SKU 无效时，SKC 不进入请求。
4. 现有折扣和利润模式回归不变。
