# SHEIN 同步商品 SKU 供货价展示设计

## 目标

在 ListingKit 的同步商品页显示 SHEIN 供货价接口返回的每个 SKU 供货价，和现有销售价快照明确区分。

## 根因

`QueryCostPrice` 已返回每个 `sku_code` 的 `cost_price_info`。现有同步解析器将同一 SKC 的 SKU 价格折叠为最大值，写入商品级 `supply_price`；逐 SKU 价格没有持久化，因此前端只能显示一条供货价。

## 设计

1. 为同步商品增加 `supply_price_snapshot` 文本字段，存储 JSON：`{"sku_supply_prices":[{"sku_code":"...","supply_price":12.34,"currency":"USD"}]}`。
2. 成本解析器保留每个有效 SKU 的供货价；商品级 `supply_price` 仍保存最大值，只用于兼容现有读取方，不能作为逐 SKU 报名计算输入。
3. 同步服务将 SKU 供货价快照写入记录，GORM 自动迁移新增列，列表 API 原样返回。
4. 同步商品表格在销售价列表旁展示“SKU 供货价（SHEIN 供货价接口）”。快照缺失时显示“下次同步后可见”，不使用 SDS 成本或销售价替代。

## 数据迁移与回填

新列由现有运行时 AutoMigrate 创建。历史记录没有 SKU 供货价快照，需要在发布后重新同步店铺或对应源 SDS 商品；不会伪造历史数据。

## 验收条件

- 一个含两个 SKU 供货价的 SHEIN 成本响应同步后，数据库/API 记录保留两条 SKU 供货价。
- 同步商品页能同时显示两条 SKU 销售价和两条 SKU 供货价，并标明各自来源。
- 缺少供货价快照时，页面不会将销售价或 SDS 成本显示为供货价。
