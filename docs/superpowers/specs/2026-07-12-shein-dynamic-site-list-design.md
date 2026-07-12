# SHEIN 动态商品站点列表设计

## 背景

当前 `store.SiteInfoHandler` 通过 `GetSiteListByRegion` 的硬编码地区映射生成商品发布站点。站点列表被写入 `TaskContext.SiteList` 和 `ProductData.SiteList`，并由尺码、SKU 和 SKC 流程共同消费。

SHEIN 商品发布页实际通过以下接口返回当前店铺可用站点：

`POST /spmp-api-prefix/spmp/supplier/query_site_list`

请求体为 `{}`。响应按主站分组，每个子站包含 `site_abbr`、`site_status`、`store_type` 和 `currency`。硬编码映射可能选择当前店铺未启用的站点，也无法自动适配新增站点。

## 目标

- 商品发布站点使用 SHEIN 接口返回的当前店铺有效站点。
- 同一任务只请求一次站点列表。
- 保持现有 `product.SiteInfo` 和所有下游消费接口不变。
- 接口失败或无有效站点时使用地区映射兜底。

## 非目标

- 不修改尺码、SKU、SKC 的站点消费逻辑。
- 不改变库存上下架接口使用的站点来源。
- 不新增跨任务持久缓存。
- 不移除 `GetSiteListByRegion`。
- 本次不根据 `store_type` 或 `currency` 分叉发布逻辑。

## API 层

在现有 SHEIN product API 中增加站点列表查询：

- endpoint 常量与 getter；
- 主站、子站响应模型；
- `QuerySiteList()` 客户端方法；
- 使用现有 `APIRequest` 和业务响应码校验。

接口固定使用 `POST` 和空 JSON 对象请求体。响应模型保留：

- 主站：`main_site`、`main_site_name`；
- 子站：`site_name`、`site_abbr`、`site_status`、`store_type`、`currency`。

虽然当前转换只使用站点标识和状态，保留其他字段可以完整表达接口语义，并避免未来重复定义模型。

## 站点规范化

增加聚焦的站点规范化函数，将 API 模型转换为现有 `[]product.SiteInfo`：

- `main_site`、`site_abbr` 去除首尾空白；
- 忽略空主站；
- 仅保留 `site_status == 1` 且 `site_abbr` 非空的子站；
- 主站按规范化后的标识合并去重；
- 同一主站内的子站去重；
- 主站和子站均保持接口首次出现顺序；
- 无有效子站的主站不进入结果。

转换时复制切片，避免上下文、发布模型与中间结果意外共享可变底层数组。

## 处理器与数据流

1. 管道执行现有 `SiteInfoHandler`。
2. 处理器通过 `ctx.ProductAPI.QuerySiteList()` 查询站点。
3. 规范化成功且结果非空时，通过 `ctx.SetSiteList` 写入任务上下文及 `ProductData.SiteList`。
4. 请求失败或规范化结果为空时记录告警，并调用 `GetSiteListByRegion(ctx.Task.Region)`。
5. 回退结果非空时同样通过 `SetSiteList` 写入。
6. 动态结果与回退结果均为空时返回明确错误，阻止后续流程在空站点状态下失败。
7. 尺码、SKU 和 SKC 继续读取现有 `SiteList`，无须修改。

处理器在管道中只执行一次，因此不新增额外任务状态标记。测试直接验证每次 `Handle` 仅产生一次站点请求。

## 错误处理

- 网络错误、非成功业务码或解析失败：告警并回退地区映射。
- API 返回空数据、所有主站无效或所有子站禁用：告警并回退地区映射。
- 单个无效条目：忽略该条，其他有效站点继续使用。
- API 与地区回退均无有效站点：返回包含地区信息的错误。
- 站点查询失败本身不使任务失败，只在无法得到任何有效站点时失败。

## 测试策略

按测试驱动方式覆盖：

- API 使用 `POST`、正确路径与 `{}` 请求体并完整解析示例响应；
- 禁用子站、空主站、空子站被过滤；
- 重复主站合并、重复子站去重、顺序保持；
- 成功路径写入 `TaskContext.SiteList` 和 `ProductData.SiteList`；
- 接口失败时回退地区映射；
- 空有效响应时回退地区映射；
- API 和回退均为空时返回明确错误；
- 现有尺码、SKU、SKC 站点相关测试继续通过。

## 验收标准

- 示例响应生成 `[{main_site: "shein", sub_site_list: ["shein-us"]}]`。
- 禁用站点不进入商品发布载荷。
- 动态站点成功时不使用地区硬编码。
- 接口不可用时保持当前地区站点行为。
- 完整 SHEIN 测试及相关包静态检查通过。
